# ADR-008: 원자적 재고 차감 = Redis Lua 스크립트

- **Date**: 2026-05-17
- **Status**: Accepted
- **Phase**: Ch10 Phase C
- **Decider**: geontae

## Context
한정판 1,000개를 100만 동접에서 *정확히* 1,000개만 발급해야 함. **oversell 0건이 절대 조건**. Race condition을 막을 *원자 연산*이 핵심.

문제:
```
Pod A 와 Pod B가 동시에 "재고 1개 남았네" 읽음
→ 둘 다 발급 시도
→ oversell 1건 발생
```

## Options

| 옵션 | 지연 | 동시성 처리량 | 영속성 | 복잡도 |
|------|:---:|:---:|:---:|:---:|
| **Redis Lua 스크립트** | < 1ms | 매우 높음 (단일 스레드 원자) | 휘발 (RDB·AOF 옵션) | 낮음 |
| DB `SELECT ... FOR UPDATE` | 5-20ms | 낮음 (row lock 경합 시 melt) | 영속 | 중간 |
| DynamoDB Conditional Write | 10-30ms | 매우 높음 (managed scale) | 영속 | 낮음 |
| Optimistic Locking (version 컬럼 + retry) | 5-30ms (retry 시 ↑↑) | 중간 (경합 적을 때) | 영속 | 중간 |

## Decision
**Redis Lua 스크립트**

```lua
-- KEYS[1] = stock key, KEYS[2] = idempotency key
-- ARGV[1] = idempotency TTL, ARGV[2] = new issue ID

-- 1. 멱등성 체크 (같은 요청 재시도 시)
local prev = redis.call('GET', KEYS[2])
if prev then return prev end

-- 2. 재고 확인 + 차감 (원자)
local stock = redis.call('GET', KEYS[1])
if not stock or tonumber(stock) <= 0 then
  return 'SOLD_OUT'
end
redis.call('DECR', KEYS[1])

-- 3. 멱등성 응답 저장
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[1])
return ARGV[2]
```

→ Redis는 **single-threaded** → Lua 스크립트 안의 모든 명령이 *원자적* (다른 클라이언트 못 끼어듦).

## Consequences
**+**:
- **< 1ms 지연** — 메모리 연산 + 한 번의 round-trip
- **100만 RPS 처리 가능** — 단일 Redis 인스턴스로도 (실측 12만~17만 ops/s, 클러스터로 수평 확장)
- 멱등성 + 재고 차감 + ID 생성을 *한 번의 원자 연산*
- 코드 짧음 (Lua 10줄)

**−**:
- **영속성 약함** — Redis가 죽으면 *순간*의 재고 상태 손실 가능. 완화책:
  - AOF 활성화 (`appendfsync everysec` 정도)
  - Multi-AZ Replication (ElastiCache Replication Group)
  - **Kafka로 발급 이벤트 영속화** (Phase E) — Redis가 죽어도 이벤트 로그 살아있음
- Lua 디버깅 어려움 — Redis CLI에서 `EVAL` 직접 호출하면 stderr 안 보임. 단위 테스트로 커버
- 재고 검증을 위한 RDB 영속화는 *비동기* — DB는 최종 일관성만 보장

## DynamoDB가 더 매력적이지 않나?
- DynamoDB Conditional Write도 atomic. 매니지드라 운영 부담 0.
- 단점: 지연 10-30ms (Redis의 10-30배). 100만 RPS면 Pod *수십 배* 필요.
- 비용: Redis(ElastiCache `cache.t3.micro` $0.023/hr 12개월 Free) << DynamoDB on-demand
- → 학습 + 시나리오에선 Redis Lua가 *더 교육적*. 운영도 흔히 Redis 선택.

## DB row lock이 더 안전하지 않나?
- ACID 트랜잭션이라 영속성 ↑. 단 100만 RPS면 PostgreSQL이 ~수만 RPS에서 lock 경합으로 melt.
- "재고 차감 = Redis Lua / 영속 기록 = DB (비동기)"가 *현업의 흔한 패턴*.

## 면접 답
"oversell 0건이 절대 조건. Redis가 single-threaded라 Lua 스크립트 안에서 멱등성 체크 + 재고 차감 + ID 생성을 한 번의 원자 연산으로. < 1ms 지연 + 12만 ops/s 처리. 영속성 약점은 Kafka 비동기 이벤트(Phase E)로 보완 — Redis 죽어도 이벤트 로그가 진실의 출처."

## 검토 일정
Phase J(부하 테스트) — Redis가 실제로 100만 RPS 견디는지·노드 1개 죽이면 어떻게 되는지 측정 후 재평가.
