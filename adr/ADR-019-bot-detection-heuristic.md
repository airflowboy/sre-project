# ADR-019: 봇 탐지 = 휴리스틱 + 교체 가능한 BotDetector 인터페이스

- **Date**: 2026-05-20
- **Status**: Accepted
- **Phase**: Ch10 Phase F-2

## Context
F-1의 AWS WAF Managed Rules는 *알려진 페이로드*(XSS·log4shell)는 잡지만 *행동 패턴* 봇은 못 잡는다(F-1 smoke에서 입증). 한정판 발급 시스템의 실제 위협은 *매크로/스크립트*가 정상 페이로드로 빠르게 두드리는 것 — WAF Managed Rules의 사각지대.

행동 기반 탐지가 필요. 어느 수준으로 만들 것인가가 결정 대상 ("라이트/풀" 분기).

## Options
| 옵션 | 학습 | 실무 적합 | 캡스톤 분량 | 함정 |
|---|---|---|---|---|
| **휴리스틱 (rule-based)** | Kafka stream 분석 + 피드백 루프 | 봇 차단 1차 방어선은 실무에서도 휴리스틱이 70-80% | 1세션 | 적음 |
| Isolation Forest (ML) | ML 이상치 탐지 + 모델 서빙 | ML 봇 탐지는 보통 SaaS(Cloudflare/DataDome) 또는 전담팀 | 2세션 | **학습 데이터·라벨링·평가셋 없으면 형식만** |
| SaaS (Cloudflare Bot Mgmt 등) | 적음 | 운영 흔함 | 외부 의존 | AWS 컨셉 이탈 |

## Decision
**휴리스틱 탐지 + `BotDetector` 인터페이스로 추상화**.

```go
type RequestSignal struct {
    IP        string
    UserAgent string
    Result    string    // issued | sold_out | idempotent_replay | rejected
    Timestamp time.Time
}

type BotDetector interface {
    // Inspect는 최근 시그널 윈도우를 받아 차단 후보 IP를 반환.
    Inspect(signals []RequestSignal) []string
}
```

구현체: `HeuristicDetector`
- **규칙 1 — burst**: 한 IP가 슬라이딩 윈도우(10초)에 N회(기본 30) 초과 → 봇
- **규칙 2 — 매진 후 집착**: sold_out 응답을 받고도 같은 IP가 계속 두드림 (sold_out 카운트 ≥ M)
- **규칙 3 (확장 슬롯)**: UA 빈/기계적, idempotency-key 패턴 등 — 인터페이스 뒤라 추가 자유

Isolation Forest 등 ML 모델은 *같은 인터페이스의 다른 구현체*로 나중에 교체. 탐지 루프(Kafka consume → Inspect → WAF 갱신)는 그대로.

## Consequences
**+**:
- **닫힌 피드백 루프를 한 세션에 완성** — 탐지 → WAF IPSet 갱신 → 차단 → 재탐지
- 휴리스틱은 *설명 가능* — "왜 이 IP를 막았나"가 규칙으로 명확 (ML의 블랙박스 대비)
- 실무 봇 차단의 1차 방어선과 동일 — rate·행동 규칙
- `BotDetector` 인터페이스로 ML 교체 시 *탐지 모듈만* 바뀜 (SRP)
- false positive 시 규칙 임계 조정이 즉각적 (ML은 재학습)

**−**:
- 휴리스틱은 *우회 가능* — 봇이 윈도우 임계 아래로 속도 낮추면 회피. ML이 *미묘한 패턴*엔 강함
- 규칙 임계가 *수동 튜닝* — 트래픽 패턴 바뀌면 사람이 조정
- 분산 카운팅: bot-detector가 여러 replica면 윈도우 카운트가 쪼개짐 → **단일 replica로 시작** (또는 Redis 공유 카운터, F-2 범위 밖)

## 왜 Isolation Forest를 지금 안 하나
- *학습 데이터가 없다* — 캡스톤엔 라벨링된 봇/정상 트래픽 데이터셋이 없음. 데이터 없는 ML은 *형식만* 남고 탐지 품질 보증 불가
- ML 봇 탐지는 SRE보다 *ML 엔지니어* 영역 — 본 로드맵(DevOps/SRE)의 정체성은 *피드백 루프 자동화*
- **Phase J 부하 테스트에서 실제 봇 트래픽이 쌓이면** 그때 데이터 기반으로 Isolation Forest 교체가 정당 — 그 시점에 ADR 갱신
- 마스터 플랜 mermaid의 "Isolation Forest"는 *목표*로 유지, 단 *데이터 확보 후* 도입

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| 윈도우 카운트 | bot-detector 단일 replica 메모리 | Redis 공유 슬라이딩 윈도우 (multi-replica) |
| 탐지 모델 | 휴리스틱 | 휴리스틱 + ML (앙상블, 같은 인터페이스) |
| 차단 해제 | 수동 / TTL | 자동 (점수 회복 시 IPSet에서 제거) |
| false positive 방어 | 임계 보수적 | shadow 모드 + 사람 검토 큐 |
| 시그널 소스 | /issue 요청만 | 전 endpoint + ALB access log |

## 검토 일정
Phase J(부하 테스트) — 실제 봇 시뮬 트래픽으로 휴리스틱 false positive/negative 측정. 데이터 충분하면 Isolation Forest 구현체 추가 검토.
