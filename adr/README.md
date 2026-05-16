# Architecture Decision Records (ADR)

본 프로젝트의 주요 기술·설계 결정 기록. 각 ADR은 **Context / Options / Decision / Consequences** 4 섹션 (Google SRE 책 + Michael Nygard 형식).

> "왜 X를 골랐어요? 왜 Y가 아니라?" — 면접에서 이 ADR 한 장으로 답.

## 인덱스

| # | 제목 | Phase | 상태 |
|:-:|------|:-----:|:----:|
| [001](ADR-001-k8s-platform-eks.md) | 캡스톤 K8s 플랫폼 = EKS | Ch10 마스터 | ✅ Accepted |
| [002](ADR-002-backend-language-go.md) | 백엔드 언어 = Go | Ch10 마스터 | ✅ Accepted |
| [003](ADR-003-nat-gateway-count.md) | NAT Gateway 수 = 단일 (학습용) | Ch10 Phase A | ✅ Accepted |
| [004](ADR-004-vpc-structure.md) | VPC 구조 = 2-AZ × (public + private) | Ch10 Phase A | ✅ Accepted |
| [005](ADR-005-node-group-type.md) | 노드 그룹 = Managed Node Group | Ch10 Phase B | ✅ Accepted |
| [006](ADR-006-eks-addons-management.md) | EKS Add-ons = AWS-managed | Ch10 Phase B | ✅ Accepted |
| [007](ADR-007-irsa-oidc-timing.md) | IRSA OIDC = Phase B에 미리 셋업 | Ch10 Phase B | ✅ Accepted |

## 작성 원칙
- **각 Phase 시작 시 그 Phase의 ADR을 먼저 작성** (코드 전).
- 1 ADR = 1 결정. 너무 잘게 쪼개지 말 것.
- A4 1장 이내. Decision과 Consequences가 핵심.
- **Consequences는 -(단점·트레이드오프)도 정직하게**.

## 상태
- **Proposed** — 검토 중
- **Accepted** — 적용 중
- **Deprecated** — 더 이상 적용 안 됨 (대체 ADR 링크)
- **Superseded by ADR-NNN** — 후속 ADR이 교체
