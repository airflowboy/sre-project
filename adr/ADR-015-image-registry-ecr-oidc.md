# ADR-015: 이미지 레지스트리 = ECR + GitHub OIDC (정공법 CI)

- **Date**: 2026-05-17
- **Status**: Accepted
- **Phase**: Ch10 Phase D-2
- **Decider**: geontae

## Context
issue-api Container 이미지를 어디에 push하고 EKS가 어떻게 pull할지 결정. 캡스톤 컨셉이 AWS 깊게이며, Phase G(CI/CD)에서 자동화 본격이지만 D-2는 *이미지 1번 push*가 필요.

자격증명 정책: **AWS access key를 GitHub에 박지 않는다** (사용자 명시 원칙).

## Options
| 옵션 | 자격증명 | 운영 |
|---|---|---|
| **ECR + GitHub OIDC** | OIDC short-lived token | AWS-native, 키 없음 |
| ECR + Access Key (GitHub secret) | 장기 access key | 키 관리 부담 |
| ghcr.io (Ch07 재활용) | GITHUB_TOKEN | EKS도 pull 가능 (public) |

## Decision
**ECR + GitHub OIDC**.

설정:
1. `aws_iam_openid_connect_provider` for `token.actions.githubusercontent.com`
2. `aws_iam_role` "github-actions-issue-api" with web identity trust:
   - Federated: GitHub OIDC provider ARN
   - Condition `StringEquals` `:aud` = `sts.amazonaws.com`
   - Condition `StringLike` `:sub` = `repo:airflowboy/sre-project:ref:refs/heads/main` (브랜치 한정)
3. `aws_iam_policy` ECR push (Get/Put/Initiate/Complete/Batch + ecr:GetAuthorizationToken)
4. `aws_ecr_repository` "issue-api" (mutable tag, scan on push)
5. `.github/workflows/issue-api.yml`:
   - `permissions: id-token: write, contents: read`
   - `aws-actions/configure-aws-credentials@v4` with `role-to-assume`
   - `docker buildx build --push -t $ECR_URL:sha-$GITHUB_SHA`

EKS node IAM role엔 이미 `AmazonEC2ContainerRegistryReadOnly` (Phase B) → ECR pull 즉시 작동.

## Consequences
**+**:
- 정적 자격증명 0 — 키 유출 위험 없음
- 토큰은 ~15분짜리 — 탈취되어도 영향 짧음
- AWS-native = ECR 이미지 스캔 + KMS 암호화 통합
- Phase G에서 자동화 정공법 그대로 확장
- IAM trust의 `sub` 조건이 *정확한 repo·브랜치만* 허용 — 다른 repo가 fork해서 push 불가

**−**:
- OIDC provider 설정 1회 필요 (~10분)
- IAM trust policy 디버깅 시 `sub` 패턴 확인 필요 (오타 → 401)
- ECR 비용: 첫 500MB/월 무료, 그 이후 $0.10/GB·월 (이미지 ~10MB이라 무료 범위)

## 왜 ghcr.io가 아닌가
- Ch07에서 이미 다룬 패턴 — 새 학습 가치 약함
- AWS 컨셉에 끼어드는 GitHub Registry는 통제면이 둘로 갈림
- private 시 EKS에 imagePullSecret 필요 (학습 추가 부담)

## 왜 Access Key가 아닌가
- 키가 GitHub repo secrets에 영구 박힘 — 유출 시 회수 어려움
- 사용자 명시 원칙: "액세스 키는 절대 git/`.tf`에 안 올림" — 비록 secrets는 git에 안 박지만 *키 자체*가 살아있음
- IAM 정책상 OIDC가 정공법

## 운영 시 변경 사항
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| Tag mutability | mutable | **immutable** |
| Scan on push | true | true |
| Lifecycle policy | 없음 | "30일 이상 untagged 자동 삭제" |
| Cross-account replication | 없음 | 멀티 리전 분기 |

## 보안 메모: OIDC trust `sub` 조건
```
repo:<owner>/<repo>:ref:refs/heads/<branch>      # 브랜치 push
repo:<owner>/<repo>:environment:<env>            # GitHub Environments
repo:<owner>/<repo>:pull_request                 # PR (위험: fork PR도 매칭)
```
**fork PR 매칭 금지** — `:ref:refs/heads/main`로 좁혀 fork PR 자동 트리거 차단.

## 면접 답
"CI에서 AWS push는 정적 키 없이 GitHub OIDC. 토큰 15분짜리 + IAM trust의 sub 조건으로 *내 repo, main 브랜치에서만* 허용. ECR scan on push로 CVE 자동 검사 — Trivy(Ch09 Phase C) 대체 또는 보완."

## 검토 일정
Phase G(CI/CD) — 다중 환경(staging/prod) 도입 시 GitHub Environments + 환경별 IAM role 분리 검토.
