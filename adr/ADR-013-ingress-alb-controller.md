# ADR-013: 외부 노출 Ingress = AWS Load Balancer Controller (ALB)

- **Date**: 2026-05-17
- **Status**: Accepted
- **Phase**: Ch10 Phase D-2
- **Decider**: geontae

## Context
issue-api Pod을 외부에 노출하려면 K8s Ingress controller가 필요. 캡스톤 컨셉:
- L7 라우팅(POST /issue 등 경로 기반)
- Phase F에서 **AWS WAF 연계** (봇 탐지 → IP 차단)
- TLS 종단
- 매 세션 destroy 가능

## Options
| 옵션 | 특성 | 캡스톤 적합도 |
|---|---|:---:|
| **AWS Load Balancer Controller (ALB)** | AWS 매니지드 ALB 자동 생성. WAF·Cognito·ACM 통합. IRSA 동작 | ✅ Phase F WAF와 자연연결 |
| ingress-nginx + NLB | Ch04-C 자산 재사용. NLB는 L4 — WAF는 별도 연결 | portable하지만 캡스톤 WAF 흐름 우회 |
| Istio Gateway | service mesh 묶음 — 캡스톤 범위 초과 | 과 |

## Decision
**AWS Load Balancer Controller**. ALB Ingress class 사용. 매 Ingress 리소스당 ALB 1개 자동 생성·삭제.

설정:
- Helm chart `eks/aws-load-balancer-controller` 설치
- ServiceAccount `aws-load-balancer-controller` (kube-system ns) + IRSA annotation
- IAM Role + AWSLoadBalancerControllerIAMPolicy (공식 정책 JSON 사용)
- Ingress 어노테이션:
  - `kubernetes.io/ingress.class: alb`
  - `alb.ingress.kubernetes.io/scheme: internet-facing`
  - `alb.ingress.kubernetes.io/target-type: ip` (Pod IP 직접 — VPC CNI 덕)
  - `alb.ingress.kubernetes.io/listen-ports: '[{"HTTP":80}]'` (학습: HTTP, 운영: 443+ACM)

## Consequences
**+**:
- AWS 네이티브 — ALB·WAF·ACM 단일 통제면
- Phase F WAF 연결이 **annotation 한 줄** (`alb.ingress.kubernetes.io/wafv2-acl-arn`)
- target-type=ip로 kube-proxy 우회 → 레이턴시 한 hop 감소
- HPA·readiness probe와 자연 통합 (unready Pod 자동 제외)
- ALB 자체가 자동 HA·확장

**−**:
- AWS 종속 (cloud-agnostic 아님) — *캡스톤이 AWS 컨셉이라 OK*
- Ingress 당 ALB 1개 = 시간당 ~$0.022 (학습용엔 미미, 운영 다중 도메인은 IngressGroup으로 공유)
- Controller IRSA 정책이 길음 (~150 actions) — 공식 JSON 그대로 사용

## 왜 ingress-nginx + NLB가 아닌가
- portable이지만 캡스톤 *의도가 AWS 깊게*
- WAF는 NLB 앞에 못 붙음 (L4). CloudFront 거쳐야 — 추가 hop·복잡도
- ingress-nginx는 *Pod 내부 nginx 프로세스*가 SPOF (replicas 늘려도 hostPort·DaemonSet 패턴 학습 부담)
- Ch 04-C에서 이미 다룬 패턴 — 캡스톤은 *새 것* 배우는 게 학습 가치

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| TLS | HTTP only | ACM 인증서 + listen-ports HTTPS 443 + SSL redirect |
| Ingress 그룹화 | 단일 Ingress = 단일 ALB | IngressGroup으로 ALB 공유 |
| WAF | 없음 (Phase F) | WAF v2 ACL ARN annotation 필수 |
| Access log | 비활성 | S3 버킷 + Athena 분석 |

## 검토 일정
Phase F(WAF) — annotation 한 줄로 WAF가 실제 붙는지 검증.
