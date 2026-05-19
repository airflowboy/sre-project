# Architecture Decision Records (ADR)

본 프로젝트의 주요 기술·설계 결정 기록. 각 ADR은 **Context / Options / Decision / Consequences** 4 섹션 (Michael Nygard 형식).

> 한 결정 = 한 ADR. *왜 X를 골랐는가*와 *어떤 트레이드오프를 받아들였는가*를 박제.

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
| [008](ADR-008-atomic-counter-redis-lua.md) | 원자적 재고 차감 = Redis Lua 스크립트 | Ch10 Phase C | ✅ Accepted |
| [009](ADR-009-go-http-stdlib.md) | Go HTTP 프레임워크 = net/http (stdlib) | Ch10 Phase C | ✅ Accepted |
| [010](ADR-010-idempotency-key-redis.md) | Idempotency Key 저장소 = Redis with TTL | Ch10 Phase C | ✅ Accepted |
| [011](ADR-011-rdbms-rds-postgres.md) | 영속 발급 기록 DB = RDS PostgreSQL (단일 AZ) | Ch10 Phase D | ✅ Accepted |
| [012](ADR-012-managed-kv-elasticache.md) | 매니지드 KV = ElastiCache Redis (단일 노드) | Ch10 Phase D | ✅ Accepted |
| [013](ADR-013-ingress-alb-controller.md) | 외부 노출 Ingress = AWS Load Balancer Controller (ALB) | Ch10 Phase D-2 | ✅ Accepted |
| [014](ADR-014-secrets-irsa-direct-sdk.md) | Secrets 동기화 = IRSA + AWS SDK Direct (학습용) | Ch10 Phase D | ✅ Accepted |
| [015](ADR-015-image-registry-ecr-oidc.md) | 이미지 레지스트리 = ECR + GitHub OIDC | Ch10 Phase D-2 | ✅ Accepted |
| [016](ADR-016-async-queue-strimzi-kafka.md) | 비동기 이벤트 큐 = Strimzi Kafka on EKS | Ch10 Phase E-1 | ✅ Accepted |
| [017](ADR-017-virtual-waiting-queue-redis-zset.md) | 가상 대기열 = Redis ZSET + Lua 글로벌 rate cap | Ch10 Phase E-2 | ✅ Accepted |
| [018](ADR-018-waf-v2-managed-rules.md) | WAF = AWS WAF v2 REGIONAL + Managed Rules + rate-based | Ch10 Phase F-1 | ✅ Accepted |

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
