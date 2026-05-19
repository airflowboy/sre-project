# ADR-018: WAF = AWS WAF v2 (REGIONAL) + Managed Rules + rate-based rule

- **Date**: 2026-05-19
- **Status**: Accepted
- **Phase**: Ch10 Phase F-1

## Context
ALB(ADR-013) 앞에 *L7 방화벽*이 필요. 1차 목표:
- SQL injection·XSS 등 *알려진 악성 패턴* 자동 차단
- 평판 나쁜 IP 차단 (AWS가 관리하는 reputation list)
- 단순 봇/스크립트의 burst를 *IP rate-limit*으로 throttle
- **커스텀 봇 탐지(머신러닝)는 Phase F-2** — F-1은 *기성 보호*에 집중

## Options
| 옵션 | 학습 가치 | 비용 | ALB 적합 | 분량 |
|---|---|---|---|---|
| **AWS WAF v2 (REGIONAL) + Managed Rules** | 중간 — annotation 한 줄로 ALB 연결 + 관리형 rule | ~$5/월 + req/M | ✅ | 작음 |
| WAF v2 CLOUDFRONT scope | CloudFront 앞단 보호 | 동일 + CDN | ALB 직접 아님 | 중간 |
| 직접 nginx ModSecurity (ingress-nginx) | 높음 — OWASP CRS 직접 | nginx Pod | ingress-nginx 필요 (ADR-013에서 ALB 선택) | 큼 |
| Third-party (Cloudflare 등) | 중간 | 외부 비용 | AWS 컨셉 이탈 | 중간 |

## Decision
**AWS WAF v2 (scope=REGIONAL) + Managed Rule Groups + 1 custom rate-based rule**.

구성:
1. `aws_wafv2_web_acl`
   - scope = REGIONAL (ALB·API Gateway·Cognito와 같이 사용; CLOUDFRONT는 별도 scope)
   - default_action = ALLOW
   - rules:
     - **AWS Managed**: `AWSManagedRulesCommonRuleSet` (OWASP top 10 기본 보호)
     - **AWS Managed**: `AWSManagedRulesKnownBadInputsRuleSet` (알려진 익스플로잇 페이로드)
     - **AWS Managed**: `AWSManagedRulesAmazonIpReputationList` (AWS의 IP 평판)
     - **Custom rate-based**: 5분당 100 req / IP 초과 시 BLOCK (학습용 낮은 임계, 운영은 더 높이)
   - visibility_config로 CloudWatch metrics 활성 (Phase H에서 대시보드)
2. ALB 연결 = Helm Ingress annotation `alb.ingress.kubernetes.io/wafv2-acl-arn` 한 줄
   - ALB Controller가 `AssociateWebACL` API 호출 자동 처리

## Consequences
**+**:
- *annotation 한 줄로 보호* — ALB Controller가 위임받아 처리, 운영 부담 0
- AWS Managed Rules가 *항상 최신* — AWS 보안팀이 패턴 갱신
- Phase F-2의 봇 탐지 ML이 *추가 rule*로 들어올 공간 확보 (`aws_wafv2_rule_group` 별도 자원으로 동적 갱신)
- CloudWatch metrics로 차단량·룰별 hit 가시화

**−**:
- 비용: $5/월 base + rule당 $1 + $0.60/M requests. 학습 한 세션은 무시 가능
- managed rule이 *너무 strict* 시 정상 트래픽 false positive 가능 → COUNT 모드로 먼저 보고 BLOCK 전환 (운영 패턴)
- rate-based rule 임계 100/5min은 *학습용* — 100만 동접 시 정상 사용자도 burst 발생 가능, 가상 대기열(E-2)이 이미 막아주는 부분
- WAF는 IP 기반 — VPN/공유 IP에 차단 시 1명 차단 = 여러 명 영향

## 왜 nginx ModSecurity가 아닌가
- ADR-013에서 ALB로 결정 → ingress-nginx 안 띄움. 일관성
- ModSecurity는 학습 가치 크지만 *룰 관리·튜닝 부담* + 운영 시 OWASP CRS 충돌·false positive 핸들링 큼
- AWS Managed Rules가 AWS 컨셉의 정공법

## 왜 CLOUDFRONT scope가 아닌가
- CloudFront 별도 도입 = Phase 추가. 캡스톤 마스터 플랜엔 CDN 따로 없음 (mermaid는 *선택적*)
- REGIONAL scope로 ALB 직접 보호가 *최소 구성*

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| Rate limit | 100/5min | 1000~10000/5min + URI/UA별 세분화 |
| Managed rule action | BLOCK 즉시 | **COUNT 1주 → BLOCK** (false positive 측정 후) |
| Custom rules | 없음 | Phase F-2 봇 탐지 모델 결과 반영 |
| Log destination | CloudWatch metrics만 | S3 + Athena + Kinesis Firehose |
| Geographical block | 없음 | 사업 지역 외 차단 (한국 발급이면 ja3/ip-geo) |
| Bot Control | 없음 | AWS WAF Bot Control (월 $10 + req) |

## 검토 일정
Phase F-2 — 봇 탐지 모델 결과를 WAF에 어떻게 반영(rule group 갱신 vs IPSet 갱신) 검토. Phase J 부하 테스트 시 rate-based rule 임계 재조정.
