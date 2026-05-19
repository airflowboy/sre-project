# ADR-017: 가상 대기열 = Redis ZSET 시간순 + Lua 글로벌 rate cap

- **Date**: 2026-05-19
- **Status**: Accepted
- **Phase**: Ch10 Phase E-2

## Context
한정판 발급 시스템에 100만 동시 진입이 몰리면 Redis Lua(ADR-008)도 처리량 상한(~수만 ops/s)을 넘어 응답 지연 → 클라이언트 timeout → 재시도 폭주 → multidie. 가상 대기열로 *시스템 입장*을 *초당 N명*으로 평탄화해야 함.

요구:
- *공정한 순서* (먼저 누른 사람 먼저 입장)
- *글로벌 admission rate cap* — 분산 환경(N replicas)에서도 정확히 N/sec
- 사용자가 현재 위치 조회 가능
- 부수효과로 봇 1차 throttle (Phase F WAF 전 사전 필터)

## Options
| 옵션 | 공정성 | 위치 표시 | 분산 cap | 단순도 |
|---|---|---|---|---|
| **Redis ZSET 시간순 + Lua rate window** | ✅ score=join ms | ✅ ZRANK | ✅ Lua atomic | 단순 |
| 토큰 버킷 | ❌ rate cap만, 순서 X | ❌ 위치 개념 없음 | ✅ 글로벌 토큰 | 단순 |
| Kafka 자체 대기열 (partition + consumer group) | partition 안에서만 보장 | ❌ rank 어색 | partition lag로 추정 | 복잡 |

## Decision
**Redis ZSET 시간순 + Lua 글로벌 rate window**.

자료 구조:
- `queue:waiting:<eventID>` ZSET — member=토큰(KSUID), score=join 시각 ms
- `queue:admitted:<eventID>` SET — member=토큰, 입장권 받은 사람
- `queue:rate:<eventID>:<unix_sec>` STRING — 현재 초의 admit 카운터, EXPIRE 2s

핵심 Lua (issue-api의 worker goroutine이 매초 호출):
```lua
-- KEYS[1]=waiting, KEYS[2]=admitted, KEYS[3]=rate window
-- ARGV[1]=max admit/sec, ARGV[2]=current_sec
local now = redis.call('GET', KEYS[3])
local already = now and tonumber(now) or 0
local quota = tonumber(ARGV[1]) - already
if quota <= 0 then return {} end
local popped = redis.call('ZPOPMIN', KEYS[1], quota)
local tokens = {}
for i = 1, #popped, 2 do table.insert(tokens, popped[i]) end
if #tokens > 0 then
  redis.call('SADD', KEYS[2], unpack(tokens))
  redis.call('INCRBY', KEYS[3], #tokens)
  redis.call('EXPIRE', KEYS[3], 2)
end
return tokens
```

→ **모든 issue-api replica가 매초 호출**해도 Lua 한 번 = single-thread = 글로벌 N/sec 보장. Leader election 불필요.

Endpoint:
- `POST /queue/join` → token 발급, ZADD, 위치 반환 (`{"token":"...","position":42}`)
- `GET /queue/status?token=...` → waiting이면 `{"status":"waiting","position":N}`, admitted면 `{"status":"admitted"}`, 둘 다 없으면 404
- `POST /issue` 헤더 `X-Queue-Token` 추가. SISMEMBER로 검증. 미통과 시 403. 통과 시 기존 Redis Lua 발급 흐름.

## Consequences
**+**:
- *순서 보장* — score=join ms로 정확히 시간순
- *분산 cap* — Lua single-thread가 글로벌 quota 자연 보장. K8s lease/leader election 코드 0
- *위치 가시화* — ZRANK 한 번에 사용자 UX 풍부
- 부수효과 봇 throttle — 토큰 없이 /issue 못 받으므로 단순 봇은 join 단계에서 N/sec 안에 갇힘
- 마스터 플랜 mermaid 일관 ("Virtual Waiting Queue Redis Sorted Set")

**−**:
- ZSET 메모리 = 대기 인원 × ~50 bytes. 100만 = 50MB (cache.t3.micro의 0.5GB 안에 OK, 운영은 t3.medium+)
- Lua 호출 빈도가 replicas × 1/s → 10 replicas면 10 calls/s. cache.t3.micro 충분
- 클라이언트가 polling (1-3초 간격) → 100만 동접 시 polling이 join보다 부하 큼. WebSocket/SSE로 push가 운영 정공법 (캡스톤은 polling으로 학습 우선)
- 토큰 재사용 정책 → 학습용엔 admitted set 멤버 영속, 운영은 사용 1회 + EXPIRE

## 왜 토큰 버킷이 아닌가
- rate cap은 같지만 *공정성·위치*가 빠짐. 한정판 발급은 *공정 순서*가 본질 (먼저 누른 사람이 먼저 받음)
- 위치 표시 없으면 "지금 안 되니까 다시 누르세요"가 사용자 UX → 재시도 폭주로 cap 의미 약화

## 왜 Kafka가 아닌가
- Kafka는 *영속 이벤트 stream*이지 *공정 대기*가 아님
- partition 분산되면 cross-partition 순서 보장 없음
- consumer lag로 "위치 추정"은 가능하지만 직접 표시 어색

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| 토큰 재사용 | admitted set 영속 | 1회 사용 후 EXPIRE 또는 SREM |
| 위치 알림 | 클라이언트 polling | WebSocket/SSE push |
| admission rate | env 고정값 | 동적 (Redis throughput 모니터링 + 자동 조정) |
| 대기열 만료 | 없음 | join 후 N분 미입장 시 토큰 무효 |
| 봇 차단 | join은 누구나 가능 | join 전에 reCAPTCHA / Phase F WAF |
| 다중 이벤트 | eventID 1개 (`summer-shoes-2026`) | 이벤트별 독립 큐 + 별도 admission rate |

## 검토 일정
Phase J(부하 테스트) — 100만 동접 시뮬에서 cap이 실제 작동하는지 + ZSET 메모리 사용량 측정.
