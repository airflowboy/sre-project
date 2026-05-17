# ADR-012: 매니지드 KV (재고·Idempotency) = ElastiCache Redis (단일 노드, 학습용)

- **Date**: 2026-05-18
- **Status**: Accepted
- **Phase**: Ch10 Phase D
- **Decider**: geontae

## Context
Phase C에서 코드는 *Redis Lua*에 의존. 실제 운영에선 그 Redis를 *어디서* 호스팅할지 선택. ADR-008은 "Redis Lua" 자체를 결정. 이 ADR은 *AWS 어떤 서비스*에 호스팅할지.

## Options
| 옵션 | 비용 | Multi-AZ | 영속성 | 우리 코드 호환 |
|------|:---:|:---:|:---:|:---:|
| **ElastiCache Redis (cache.t3.micro 단일)** | Free Tier 12mo | ❌ (단일 노드) | RDB 스냅샷 | ✅ 100% (Redis 7) |
| ElastiCache Redis Replication Group (2 노드) | 2× = ~$33/월 | ✅ (자동 페일오버 ~30초) | RDB+AOF | ✅ |
| **MemoryDB for Redis** | 최소 ~$70/월 | ✅ (강한 일관성) | 영속 (멀티 AZ 트랜잭션 로그) | ✅ (Redis API) |
| K8s 내 Redis (Helm chart) | 노드 자원만 | StatefulSet으로 가능 | 직접 PVC 관리 | ✅ |

## Decision
**ElastiCache Redis cache.t3.micro 단일 노드** (학습용 — 운영은 Replication Group 또는 MemoryDB)

설정:
- `engine = "redis"`, `engine_version = "7.1"` (현재 stable, Lua 7 호환)
- `node_type = "cache.t3.micro"` (Free Tier 750h/mo × 12mo)
- `num_cache_nodes = 1` (단일 노드, 단일 AZ — 운영은 Replication Group)
- `parameter_group_name = "default.redis7"`
- `subnet_group_name` → Phase A의 private subnets ×2
- `security_group_ids` → 새 SG (EKS cluster SG에서만 6379)
- `port = 6379`
- AUTH 없음, TLS 없음 (학습용 — 운영은 Multi-AZ + AUTH + transit_encryption)

## Consequences
**+**:
- **Free Tier 12개월** — 학습 비용 0
- 우리 Phase C 코드(go-redis)가 *수정 없이* 즉시 작동 — `REDIS_ADDR` 환경변수만 바꾸면 됨
- ElastiCache는 *Redis 정확히 호환* — Lua 스크립트 그대로 작동
- 매 세션 destroy 가능 (snapshot 비활성)
- VPC 안에 있어 *Pod ↔ Redis* 통신이 사설망 (낮은 지연 + 비용 0)

**−**:
- 단일 노드 — 노드 죽으면 *데이터 손실 + 다운*. 운영엔 부적합
- AUTH 없음 — SG로만 격리. 운영은 AUTH + TLS 권장 (그러나 SG 잘 짜면 *현실적 위협 거의 0*)
- 매니지드라도 *minor version 자동 업그레이드*가 잠깐의 페일오버 일으킬 수 있음 — 운영은 maintenance window 명시
- Free Tier 12개월 후 ~$11/월 — 만료 시점에 재검토

## 왜 MemoryDB가 아닌가
- MemoryDB는 *강한 일관성 + 영속성* (Multi-AZ 트랜잭션 로그) → 진짜 진실의 출처로도 사용 가능
- 단점: 최소 ~$70/월. 캡스톤 학습 환경엔 과
- 우리 패턴 = "Redis = 빠르고 휘발성 / Kafka·DB = 영속 진실의 출처" — 이 분리가 *현업 표준*
- 만약 *Redis 자체*를 진실의 출처로 쓴다면 MemoryDB가 답이지만, 우리는 그러지 않음

## 왜 K8s 내 Redis가 아닌가
- StatefulSet + EBS PVC로 가능. Bitnami Helm chart 기준 ~10분에 떰
- 단점: 우리가 *운영 책임* (백업·페일오버·minor 버전 업그레이드). EKS 노드 죽으면 Pod 재스케줄·EBS 재attach 시간
- 매니지드(ElastiCache)는 *그 부담을 AWS가 짐* — 학습 단계에선 매니지드가 가치 ↑
- 운영에서도 *Redis 인스턴스 운영* 직접 하는 건 드묾 (전문 운영팀이면 모를까)

## 운영 변경 사항 (현재 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| Multi-AZ | 단일 노드 | **Replication Group, 2+ AZ** |
| 페일오버 | 수동 (다운) | 자동 (~30초) |
| AUTH | 없음 | 있음 (Secrets Manager에 저장) |
| TLS (transit) | 없음 | 있음 (`transit_encryption_enabled=true`) |
| 백업 | 비활성 | RDB 스냅샷 일별 |
| Maintenance window | 자동 | 명시 (트래픽 적은 시간) |

## 면접 답
"ElastiCache Redis = 매니지드라 운영 부담 0, Redis 정확히 호환되어 코드 수정 X. 학습은 cache.t3.micro Free Tier 단일 노드 — 운영은 Replication Group(Multi-AZ + 자동 페일오버) + AUTH + TLS. MemoryDB는 영속·강일관성이 *Redis 자체*를 진실의 출처로 쓸 때 가치 — 우리는 Kafka·DB가 영속 진실이라 ElastiCache로 충분."

## 검토 일정
Phase J(부하 테스트) — Redis 단일 노드가 100만 RPS 견디는지 측정. 노드 죽이기 chaos 시 *어떻게 다운되는지* 관찰 후 Replication Group 도입 검토.
