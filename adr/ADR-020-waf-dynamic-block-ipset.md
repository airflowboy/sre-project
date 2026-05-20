# ADR-020: WAF 동적 차단 = aws_wafv2_ip_set + SDK 갱신

- **Date**: 2026-05-20
- **Status**: Accepted
- **Phase**: Ch10 Phase F-2

## Context
ADR-019의 봇 탐지가 *차단 후보 IP*를 산출하면, 이를 WAF에 반영해 *다음 요청부터 차단*해야 한다. 피드백 루프의 마지막 단계 — "탐지 결과를 어떻게 WAF에 먹이나".

## Options
| 옵션 | 갱신 단위 | API | 적합 |
|---|---|---|---|
| **aws_wafv2_ip_set + UpdateIPSet** | IP 목록 | `GetIPSet` → `UpdateIPSet` (LockToken) | 봇 탐지 결과가 IP 단위 — 정확히 맞음 |
| RuleGroup 동적 수정 | 복잡한 rule | `UpdateRuleGroup` | rule 자체를 바꾸는 건 과함 |
| WAF ACL 직접 수정 | ACL 전체 | `UpdateWebACL` | ACL 전체 rewrite — 위험·과함 |
| Lambda@Edge / 별도 차단 | 코드 레벨 | - | WAF 우회, 일관성 ↓ |

## Decision
**`aws_wafv2_ip_set` (REGIONAL) + SDK `UpdateIPSet` 갱신**.

흐름:
1. Terraform이 빈 `aws_wafv2_ip_set` "bot-blocklist" 생성 (IPv4, 초기 addresses=[])
2. WAF ACL(ADR-018)에 **priority 0** rule 추가 — IPSet 매칭 시 BLOCK (managed rule보다 먼저 평가)
3. bot-detector Pod이 봇 IP 판정 → AWS SDK:
   - `GetIPSet` 으로 현재 addresses + **LockToken** 획득
   - addresses에 새 봇 IP(`/32` CIDR) 추가
   - `UpdateIPSet` 호출 (LockToken 동봉 — optimistic locking)
4. WAF가 ~수십 초 내 전파 → 그 IP의 다음 요청 403

`lifecycle { ignore_changes = [addresses] }` — Terraform이 IPSet을 만들지만 *내용은 bot-detector가 관리*. apply가 런타임 추가분을 되돌리지 않도록.

## Consequences
**+**:
- 봇 탐지 결과(IP 목록)와 WAF 갱신 단위가 *정확히 일치* — 변환 없음
- IPSet rule이 priority 0 = managed rule보다 먼저 → 봇 IP는 *즉시 컷*
- `UpdateIPSet`의 LockToken = optimistic locking → 여러 갱신 충돌 안전
- Terraform(생성) / bot-detector(내용) 책임 분리 — `ignore_changes`로 깔끔
- IPSet은 ACL과 독립 자원 — 다른 ACL에서도 재사용 가능

**−**:
- IPSet 1개 최대 10,000 CIDR (REGIONAL) — 대량 봇엔 부족, 운영은 IPSet 복수 + 회전
- WAF 전파 지연 ~수십 초 — 탐지~차단 사이 봇이 더 두드릴 수 있음 (rate-based rule이 그 사이 일부 흡수)
- 차단 *해제* 로직 별도 필요 — 현재 학습 범위는 *추가만*, TTL/해제는 운영 항목
- `UpdateIPSet`은 *전체 addresses 덮어쓰기* — bot-detector가 항상 최신 목록 전체를 들고 있어야 (GetIPSet 먼저)

## 왜 RuleGroup이 아닌가
- RuleGroup은 *rule 묶음*을 관리하는 자원 — IP 목록 갱신엔 무겁다
- 봇 탐지 출력은 *IP*라는 단순 형태 → IPSet이 1:1
- RuleGroup 동적 수정은 rule 로직 자체가 바뀔 때 (예: 새 페이로드 시그니처) 의미

## 왜 ACL 직접 수정이 아닌가
- `UpdateWebACL`은 ACL 전체 정의를 덮어씀 → managed rule 설정까지 통째 rewrite 위험
- IPSet은 ACL이 *참조*만 하므로 ACL 안 건드리고 내용만 갱신 — 격리

## 보안 메모: bot-detector의 IAM 권한
IRSA role에 부여하는 권한은 *최소*로:
```
wafv2:GetIPSet
wafv2:UpdateIPSet
```
특정 IPSet ARN으로 resource 제한. `wafv2:*` 절대 금지 — bot-detector가 탈취돼도 *그 IPSet 하나*만 영향.

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| 차단 해제 | 없음 (추가만) | TTL 또는 점수 회복 시 자동 제거 |
| IPSet 용량 | 1개 (10k 한도) | 복수 IPSet + 회전 |
| 갱신 빈도 | 탐지 즉시 | 배치(N초 모아서) — API rate limit 회피 |
| 감사 | CloudWatch만 | 차단 IP + 사유를 DB/로그에 영속 |
| 오탐 복구 | 수동 | allowlist IPSet 우선순위 + 자동 |

## 검토 일정
Phase J — 부하 테스트 시 IPSet 갱신이 WAF 전파 지연 안에서 얼마나 효과적인지, 10k 한도에 얼마나 빨리 닿는지 측정.
