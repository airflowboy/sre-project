# ADR-010: Idempotency Key 저장소 = Redis (with TTL)

- **Date**: 2026-05-17
- **Status**: Accepted
- **Phase**: Ch10 Phase C
- **Decider**: geontae

## Context
**Idempotency Key**: 클라이언트가 같은 요청을 재시도해도 *한 번만 처리*되도록 보장. 한정 발급에서 필수 — 네트워크 끊김 등으로 재시도 시 *중복 발급* 방지.

```
Client → POST /issue, Idempotency-Key: abc123
Server → 발급 ID "X1" 응답 + abc123 ↔ X1 매핑 저장
Client (재시도) → POST /issue, Idempotency-Key: abc123
Server → 매핑 보고 *동일한* X1 반환 (재발급 X)
```

저장소 선택.

## Options
| 옵션 | 지연 | TTL 관리 | 영속성 | 비용 |
|------|:---:|:---:|:---:|:---:|
| **Redis with EXPIRE** | < 1ms | 네이티브 (Redis가 자동 만료) | 휘발 (or AOF) | 무료 (재고 Redis 재사용) |
| DB `unique constraint` on idempotency_key 컬럼 | 5-20ms | 별도 cleanup job 필요 | 영속 | DB 자원 |
| 둘 다 (Redis 캐시 + DB 진실의 출처) | 1ms cache hit | Redis TTL + DB cleanup | 영속 | 둘 다 |
| 메모리 (Pod 안 map) | < 0.1ms | 어려움 | 휘발, Pod 재시작 시 잃음 | 무료 |

## Decision
**Redis with TTL** (TTL = **24시간**)

→ **ADR-008의 재고 Redis와 같은 인스턴스 사용**. 키 prefix만 다름:
- 재고: `stock:event:summer-shoes-2026`
- Idempotency: `idem:summer-shoes-2026:<key>`

## Consequences
**+**:
- 재고 Lua 스크립트 안에서 *함께* 처리 (한 round-trip) — ADR-008 스크립트의 `KEYS[2]`가 idempotency key
- **자동 만료** (EXPIRE) — 24시간 후 자동 삭제, cleanup job 불필요
- 클라이언트 재시도 윈도우는 보통 분 단위라 24시간 충분 (긴 의도 — 사용자가 *내일* 다시 시도해도 안전)
- 비용 0 (재고 Redis 재사용)

**−**:
- **영속성 한계** — Redis 죽으면 idempotency 매핑 잃음. 그 사이 재시도 → 중복 발급 가능
  - 완화: AOF + Replication (ElastiCache Replication Group)
  - 완화: Kafka에 발급 이벤트 영속화 (Phase E) — *최종 일관성*으로 중복 발견·취소 가능
- TTL 24h 동안 메모리 점유 — 100만 발급 + 100B 키 ≈ 100MB. cache.t3.micro(0.5GB)면 여유

## Idempotency Key 형식 (HTTP 헤더)
```
POST /issue HTTP/1.1
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000
```
- 클라이언트가 UUID·KSUID 등 생성
- 헤더 누락 시 400 Bad Request (필수)
- Stripe API 패턴 차용 (`Idempotency-Key` 헤더가 사실상 표준)

## 다른 접근들의 트레이드오프
- **DB unique constraint**: 영속성 ↑이지만 *DB 부하 ↑*. 100만 RPS → DB가 lock·write 경합으로 melt
- **Hybrid (Redis + DB)**: 가장 견고. 운영 시 도입 검토 — *학습엔 과함*
- **메모리 (sync.Map)**: 한 Pod에서만 유효. Pod 여러 개면 same key가 다른 Pod에 가면 중복 처리. ❌

## 검토 일정
Phase E(Kafka) 후 — 발급 이벤트 영속화 추가되면, Redis idempotency가 손상돼도 Kafka 이벤트로 *중복 발견·취소* 가능. 그 시점에 영속성 트레이드오프 재평가.
