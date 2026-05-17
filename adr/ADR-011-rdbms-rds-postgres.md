# ADR-011: 영속 발급 기록 DB = RDS PostgreSQL (단일 AZ, 학습용)

- **Date**: 2026-05-18
- **Status**: Accepted
- **Phase**: Ch10 Phase D
- **Decider**: geontae

## Context
Phase E에서 발급 이벤트를 *비동기로* 영속 저장(Kafka 컨슈머 → DB). 발급 ID·user_id·timestamp·idempotency_key를 기록. **재고 정합성의 *진실의 출처*는 Redis Lua (실시간)**, **감사·복구·분석의 진실의 출처는 DB (영속)**. 트랜잭션 SQL DB가 필요.

## Options
| 옵션 | 비용 | 운영 | 캡스톤 적합도 |
|------|:---:|:---:|:---:|
| **RDS PostgreSQL db.t3.micro** | Free Tier 12mo, 그 후 ~$15/월 | 매니지드 | ✅ Ch 04 학습 연속성 |
| Aurora PostgreSQL Serverless v2 | ~$45/월 (최소 0.5 ACU) | 더 매니지드, 자동 스케일 | 가격↑↑ — 학습엔 과 |
| RDS MySQL | Free Tier 동일 | 매니지드 | PostgreSQL의 풍부한 타입 시스템 손해 |
| DynamoDB | Free Tier (25GB) | 서버리스 | NoSQL — JOIN·집계 약함 (분석 시 손해) |

## Decision
**RDS PostgreSQL db.t3.micro** (Free Tier, 단일 AZ, 빠른 destroy)

설정:
- `engine = "postgres"`, `engine_version = "16.4"` (현재 stable)
- `instance_class = "db.t3.micro"` (1 vCPU, 1 GB RAM)
- `allocated_storage = 20` (GB, gp3)
- `multi_az = false` (학습용 — 운영은 true로)
- `backup_retention_period = 0` (학습용 destroy 빠르게 — 운영은 7+)
- `skip_final_snapshot = true` (destroy 시 스냅샷 안 만듦)
- `deletion_protection = false`
- `publicly_accessible = false` (private subnet only)
- `vpc_security_group_ids` → 새 SG (EKS cluster SG에서만 5432 허용)
- `db_subnet_group` → Phase A의 private subnets ×2 (DB subnet group은 최소 2 AZ 요구)
- password = `random_password` 24자 → Secrets Manager에 저장 (ADR-014)

## Consequences
**+**:
- Free Tier 12개월 — 학습 비용 0
- Ch 04 PostgreSQL StatefulSet 학습을 클라우드 매니지드 버전으로 자연스럽게 확장
- PostgreSQL의 풍부한 타입(uuid, jsonb, timestamptz) — 발급 기록에 유용
- 매 세션 destroy 가능 (`skip_final_snapshot=true`, `backup_retention=0`)
- ACID 보장 — 감사·복구의 진실의 출처로 적합
- Phase F의 봇 탐지 학습 데이터도 같은 DB에 (간단한 분석은 SQL로)

**−**:
- 단일 AZ — DB AZ 장애 시 ~10분 다운 (운영은 `multi_az = true`로 자동 페일오버 60초)
- backup 0 — 데이터 잃으면 끝 (학습용 OK). 운영은 7일 + 스냅샷
- t3.micro 1GB RAM — 100만 동접 부하 테스트엔 부족 (Phase J에서 t3.medium 일시 업그레이드)
- Free Tier 12개월 후 ~$15/월 — 만료 시점에 재검토

## 왜 Aurora가 더 좋지 않은가
- Aurora는 분리된 스토리지 레이어로 HA·읽기 복제본 강함
- 단점: Serverless v2도 최소 0.5 ACU 상시 = ~$45/월
- 학습 환경엔 RDS 표준이 충분 — Aurora는 운영 가치 본격 (수십만 QPS·복제 지연 최소화 필요할 때)
- "RDS → Aurora 이전"은 endpoint 갱신 정도라 *나중에 옮기기 쉬움*

## 왜 DynamoDB가 아닌가
- 발급 기록은 *주로 시간순 조회·집계* — SQL이 자연스러움
- DynamoDB도 가능하지만 GSI 설계·관계 조회 복잡
- Phase F 봇 탐지 학습 데이터 분석 시 SQL이 압도적
- Cost: DynamoDB on-demand는 사용량 패턴에 따라 RDS보다 비쌀 수 있음 (특히 분석 쿼리)

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| Multi-AZ | false | **true** |
| Backup retention | 0일 | 7~35일 |
| 자동 마이너 버전 업그레이드 | ?? | true |
| Performance Insights | 비활성 | 활성 |
| 암호화 (at rest) | 기본 | KMS 키 명시 |
| Parameter Group | default | 커스텀 (`shared_buffers` 등 튜닝) |

## 검토 일정
Phase J(부하 테스트) — 100만 동접에서 DB 쓰기 부하가 어떤지 측정. 필요 시 Aurora 이전·Read Replica 도입 검토.
