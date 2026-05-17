# ADR-014: Secrets 동기화 = IRSA + AWS SDK Direct (앱 안에서 직접 호출)

- **Date**: 2026-05-18
- **Status**: Accepted (학습 한정 — 운영은 External Secrets Operator 권장)
- **Phase**: Ch10 Phase D
- **Decider**: geontae

## Context
issue-api Pod이 RDS 비번·Redis AUTH·기타 시크릿을 어떻게 *안전하게* 받을지. **환경변수에 평문 키 박는 안티패턴**은 절대 안 함. AWS Secrets Manager에 보관 + IRSA로 Pod이 *IAM 신원으로* 직접 접근.

## Options
| 옵션 | 동작 | 의존성 | 학습 가치 |
|------|------|:---:|:---:|
| **IRSA + AWS SDK Direct** (이 ADR) | Pod 코드가 시작 시 `secretsmanager:GetSecretValue` 호출 | AWS SDK | ✅ IRSA 메커니즘 직접 노출 |
| External Secrets Operator (ESO) | 별도 오퍼레이터가 AWS → K8s Secret 동기화. Pod은 K8s Secret 마운트만 | ESO 설치 (Helm) | 운영 표준이지만 IRSA가 *오퍼레이터 안에 숨겨짐* |
| CSI Secret Store Driver (AWS provider) | Pod이 secret을 파일시스템으로 마운트 (CSI 볼륨) | CSI driver 설치 | 마운트 모델, 회전 자동 |
| 환경변수에 평문 | 절대 X | — | 안티패턴 |

## Decision
**IRSA + AWS SDK Direct** (학습용. 운영은 ESO 또는 CSI Secret Store Driver로 마이그레이션 권장)

흐름:
```
Pod 시작
 ↓ ServiceAccount(annotation: eks.amazonaws.com/role-arn = <IRSA Role ARN>)
 ↓ AWS SDK가 STS AssumeRoleWithWebIdentity 호출 (SA 토큰 + OIDC trust)
 ↓ 임시 IAM 자격증명 받음
 ↓ secretsmanager.GetSecretValue("sre-roadmap-ch10/db/password")
 ↓ RDS 비번 받아서 sql.Open()에 사용
```

## Consequences
**+**:
- IRSA 메커니즘이 *Pod 코드 안에서 직접 보임* — 학습 가치 ↑
- 의존성 0 (별도 operator 설치 불필요)
- AWS Secrets Manager → IAM 신원 → Pod의 흐름이 *한 호출에 압축*
- ADR-007의 OIDC provider (Phase B에서 미리 만든 것)가 *드디어 사용됨* — Phase B의 사전 작업이 효과
- 비번 회전 시 Pod 재시작 시 자동 갱신 (Pod 시작 시 호출하니까)

**−**:
- **회전 자동화 부족** — Secrets Manager가 비번 회전해도 Pod이 *재시작 안 하면* 옛 비번 사용. AWS는 회전 시 ECS·Lambda는 자동 갱신해주지만 Pod은 별도 처리 필요
- 매 Pod 시작마다 Secrets Manager API 호출 — 1 호출 = $0.05/10k = ~$0.000005. 무의미한 비용이지만 *호출 수 의존* 패턴
- 코드에 AWS SDK 의존 추가 (Go 이미지 살짝 커짐)
- K8s 네이티브 컨벤션 약간 위반 (보통 환경변수·파일로 시크릿 받음)

## ESO가 더 표준 아닌가
**맞음.** ESO 패턴이 운영 표준:
- ESO Operator가 Secrets Manager polling → K8s Secret 갱신
- Pod은 *그냥 K8s Secret 환경변수*로 받음 — Pod 코드가 AWS 모름 = *클라우드 중립*
- 회전 자동 (operator가 polling → secret 갱신 → Pod이 reload 트리거)

**그런데 학습엔 직접 패턴이 더 좋은 이유**:
1. IRSA 메커니즘이 *코드에서 직접 보임* — "왜 Pod이 AWS API 호출이 되지?"의 답을 *내 코드*에 적음
2. ESO는 *오퍼레이터 안*에 IRSA 사용을 숨김 — 동작은 같지만 *학습 효과 ↓*
3. 캡스톤은 issue-api 1개 — operator의 가치(여러 워크로드 공유)가 작음
4. Phase D-2에서 ESO 추가하는 *진화 경로*가 자연스러움 ("이제 충분히 이해했으니 표준 패턴으로")

## 코드 예시 (issue-api Phase D-2 추가)
```go
import (
    "context"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func fetchSecret(ctx context.Context, name string) (string, error) {
    cfg, err := config.LoadDefaultConfig(ctx)  // ← IRSA 토큰 자동 사용
    if err != nil { return "", err }
    client := secretsmanager.NewFromConfig(cfg)
    out, err := client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
        SecretId: &name,
    })
    if err != nil { return "", err }
    return *out.SecretString, nil
}
```
→ Pod 시작 시 `dbPassword, _ := fetchSecret(ctx, "sre-roadmap-ch10/db/password")` 호출. IRSA 토큰은 *환경변수 + 토큰 파일*로 자동 주입(IRSA mutating webhook).

## IRSA의 마법 — Pod이 IAM 자격증명 받는 과정
1. ServiceAccount에 `eks.amazonaws.com/role-arn` annotation
2. Pod 생성 시 IRSA mutating webhook이 끼어들어 환경변수 `AWS_ROLE_ARN`, `AWS_WEB_IDENTITY_TOKEN_FILE` 자동 주입 + projected service account token volume 마운트
3. AWS SDK가 환경변수 보고 토큰 파일 읽어서 STS `AssumeRoleWithWebIdentity` 호출
4. STS가 OIDC issuer로 토큰 검증 (Phase B의 OIDC provider 등록 덕)
5. trust policy의 조건(SA 이름 매칭) 통과하면 임시 IAM 자격증명 반환
6. SDK가 이후 API 호출에 그 자격증명 사용

→ **Pod 코드가 AWS access key 한 번도 안 가짐**. 신원 = K8s ServiceAccount. 권한 = IAM Role. 둘이 OIDC로 연결.

## 운영 마이그레이션 경로 (학습 → 운영)
1. **Phase D-2 (현재)**: IRSA + SDK Direct
2. **(나중)** ESO Operator 설치 → AWS Secrets Manager → K8s Secret 자동 동기화 → Pod은 K8s Secret만 봄
3. **결과**: Pod 코드에서 AWS SDK 제거 → 클라우드 중립 + 회전 자동

## 면접 답
"학습 목적으로 IRSA + AWS SDK Direct — Pod이 자기 IAM 신원으로 Secrets Manager 직접 호출. ServiceAccount annotation → projected SA token → STS AssumeRoleWithWebIdentity → 임시 자격증명. 운영 표준은 External Secrets Operator로 K8s Secret 자동 동기화 + 회전 자동화. 직접 패턴 → ESO 마이그레이션이 자연스러운 진화."

## 검토 일정
Phase D-2 끝나고 — 실제 issue-api Pod이 IRSA로 secrets 받는 거 작동 확인 후, ESO 도입 시기 검토 (또는 캡스톤 후 별도 챕터).
