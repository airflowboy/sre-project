# ADR-016: 비동기 이벤트 큐 = Strimzi Kafka on EKS

- **Date**: 2026-05-18
- **Status**: Accepted
- **Phase**: Ch10 Phase E-1

## Context
발급 응답 경로는 Redis Lua 하나로 끝나야 하므로(<1ms, ADR-008), DB 영속화·분석·후처리는 **응답 경로 밖**에서 일어나야 한다. 발급 이벤트를 큐에 던지고, 컨슈머가 RDS에 INSERT한다. 이 큐가 무엇이어야 하는가.

요구:
- At-least-once delivery (멱등성은 ADR-010의 idempotency key로 이미 처리)
- 컨슈머 lag·throughput 가시화
- 캡스톤 후속 Phase에서 *재활용 가능한 기반* — Phase F(봇 탐지 stream), H(SLO 이벤트 집계), I(보안 audit log)
- 매 세션 destroy 가능

## Options
| 옵션 | 학습 가치 | 비용 | 운영성 | 분량 |
|---|---|---|---|---|
| **Strimzi Kafka on EKS** | 높음 — Kafka 원리 + StatefulSet 운영 | EKS 노드만 (~$0) | KRaft mode면 ZooKeeper 없음, Operator가 ops 자동화 | 큼 |
| MSK Serverless | 중간 — Kafka API + AWS 매니지드 | ~$0.75/h | AWS 책임 | 중간 |
| SQS | 낮음 — 추상화에 묻힘 | ~$0.40/백만 req | AWS 책임, 가장 단순 | 작음 |

## Decision
**Strimzi Kafka on EKS**.

구체:
- Strimzi Operator (`strimzi/strimzi-kafka-operator`) Helm 설치
- `Kafka` CR 1개 — KRaft mode (ZooKeeper 없음, broker 1 + controller 1 = single replica for learning)
- `KafkaTopic` CR — `issuance.events` (partitions=3, replicas=1)
- Storage: emptyDir (학습 — 데이터 잃어도 OK, destroy 빠르게). 운영은 persistent PV
- Listener: internal `tls: false`, `authentication: none` (학습 — VPC 내부)
- producer = `issue-api` Pod (sarama 또는 kafka-go 라이브러리)
- consumer = 별도 Deployment `issuance-consumer` (Pod 1개, Kafka consumer group 1개)

## Consequences
**+**:
- Kafka 메커니즘 본격 학습 — broker / partition / consumer group / offset / lag
- StatefulSet on EKS 운영 체험 — Phase D-1 RDS와 다른 결의 stateful (CRD가 manifest 추상화)
- Phase F/H/I에서 *같은 Kafka로 stream* 가능 — 봇 탐지 모델 입력, SLO 이벤트, audit log
- 마스터 플랜 mermaid 도식과 일관 ("Kafka (Strimzi)")
- 비용 ~$0 (EKS 노드에 얹힘, RDS/MSK 같은 별도 매니지드 비용 없음)

**−**:
- StatefulSet + Operator + CR 학습 곡선 — Pod이 즉시 안 뜨고 Operator 거쳐 reconcile
- Single broker는 *프로덕션 안티패턴* — 학습 정당화. 운영은 3 brokers 최소
- emptyDir는 *재시작 시 데이터 소실* — `terraform destroy` 사이클엔 OK, 운영은 PV 필수
- Phase J 부하 100만 RPS 시점에 broker 1개는 병목 — 그때 brokers 3 + partitions 늘려 재평가

## 왜 SQS가 아닌가
- AWS 매니지드라 *큐 메커니즘 자체*는 가려짐 — 학습 가치 낮음
- 캡스톤이 "DevOps/SRE 스택 직접 구축" 컨셉이라 stream 인프라도 직접 보는 게 일관
- Phase F·H에서 stream replay·multi-consumer가 필요해질 때 SQS는 한계 (FIFO+DLQ로 어느 정도 가능하지만 어색)

## 왜 MSK Serverless가 아닌가
- Kafka API는 동일하지만 *어디서 broker가 도는지 안 보임*
- 시간당 비용이 학습 환경엔 부담 ($0.75/h × 매 세션)
- Strimzi가 *원리 + 운영* 둘 다 잡힘

## 운영 시 변경 사항 (현재는 학습용 단순화)
| 항목 | 학습 (현재) | 운영 |
|---|---|---|
| Broker replica | 1 | 3 (min.insync.replicas=2) |
| Topic replication factor | 1 | 3 |
| Storage | emptyDir | PV (jbod, gp3 100Gi+) |
| Listener TLS | false | mTLS + ACL |
| Authentication | none | SCRAM or mTLS |
| Cruise Control | 미설치 | 설치 (자동 partition rebalance) |
| Schema Registry | 미설치 | Confluent or Karapace + Avro |

## 검토 일정
Phase J(부하 테스트) — broker 1로 100만 RPS 한계 측정. 부족하면 broker 3 + partitions 늘려 재시도. KRaft mode 운영 안정성도 그때 평가.
