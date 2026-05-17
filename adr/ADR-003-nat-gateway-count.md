# ADR-003: NAT Gateway 수 = 단일 (학습용)

- **Date**: 2026-05-15
- **Status**: Accepted (학습 한정 — 운영은 ADR-XXX로 재평가 예정)
- **Phase**: Ch10 Phase A
- **Decider**: geontae

## Context
프라이빗 서브넷의 EKS 워커 노드·RDS·Redis는 인터넷 outbound가 필요 (컨테이너 이미지 pull, Secrets Manager·S3 API 호출, OS 패치 등). NAT Gateway가 해당 경로. **요구**: 비용 ↓ + 학습 환경에선 SPOF 허용 가능.

## Options
| 옵션 | 비용 | 가용성 | 데이터 비용 |
|------|:---:|:---:|:---:|
| **단일 NAT GW (1 AZ에)** | $0.045/hr × 1 = ~$32/월 | SPOF (그 AZ 장애 = 전 프라이빗 outbound 중단) | 표준 |
| AZ당 NAT GW (×2) | × 2 = ~$64/월 | HA — AZ 장애 자동 격리 | 표준 (cross-AZ 데이터 비용 없음) |
| NAT instance (직접 운영) | EC2 t3.nano ~$3/월 × N | 자체 운영 (HA 어려움, 처리량 한계) | 표준 |
| VPC Endpoints + NAT | NAT $32/월 + 일부 endpoint 무료 | AWS API는 endpoint로, 일반 인터넷은 NAT | ↓ (S3/ECR endpoint 무료) |

## Decision
**단일 NAT Gateway** (`var.single_nat_gateway = true` 기본값)

## Consequences
**+**:
- 비용 절감 — 학습 환경에서 NAT 비용이 EKS control plane 다음으로 큼
- 모든 프라이빗 서브넷이 동일 NAT를 공유 — 라우트 테이블 단순 (단일 private RT)
- 매 세션 destroy로 누적 비용 0 — 24/7 SPOF 노출 시간이 없음
- 변수 `single_nat_gateway`만 false로 바꾸면 즉시 AZ당 NAT로 전환 (코드 동일, count만 변화)

**−**:
- 운영 환경엔 부적합 — NAT가 있는 AZ에 장애가 나면 전 프라이빗 outbound 중단
- 캡스톤 chaos engineering 시 "AZ 1개 죽이기" 시나리오는 의도적으로 NAT가 있는 AZ를 피해야 의미 있음
- 운영 시점엔 ADR로 재평가 — VPC Endpoint 도입(S3/ECR/Secrets Manager 등)으로 NAT 트래픽을 줄이고 NAT 자체는 AZ당 ×2로

## 비용 비교 (한 세션 2~3시간)
- 단일 NAT: $0.045 × 3h = **$0.135**
- AZ당 NAT: $0.045 × 2 × 3h = **$0.27**
- 차이 ~$0.13/세션. 매 세션 destroy로 누적은 비슷하지만, 학습 환경 SPOF 허용 + 코드 단순성 우선.

## 검토 일정
Phase J(부하 테스트) 직전 — 부하 테스트 트래픽이 NAT를 통과한다면 LCU/데이터 비용 검토. Phase E(Kafka)·F(ML) 도입 시 outbound 트래픽이 늘면 VPC Endpoint 도입 고려.
