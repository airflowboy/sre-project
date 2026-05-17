# ADR-004: VPC 구조 = 2-AZ × (public + private), DB는 private 공유

- **Date**: 2026-05-15
- **Status**: Accepted
- **Phase**: Ch10 Phase A
- **Decider**: geontae

## Context
캡스톤은 EKS + ALB + RDS + ElastiCache + (옵션) Kafka·ML 컴포넌트. **요구**: ALB가 2 AZ 필요, EKS 노드 그룹 멀티 AZ 권장, RDS 멀티 AZ 옵션, 비용 합리. 서브넷 CIDR을 어떻게 쪼개나.

## Options
| 옵션 | 서브넷 수 | 격리 수준 |
|------|:---:|------|
| **2-AZ × (public + private), DB도 private 공유** | 4 | 표준 EKS 패턴, DB·노드 같은 RT |
| 2-AZ × (public + app-private + db-private) | 6 | DB 라우팅 더 엄격 (별도 RT 가능) |
| 3-AZ × (public + private) | 6 | 99.99% 가용성 가능, AZ 비용·복잡도 ↑ |

## Decision
**2-AZ × (public + private) = 4 서브넷**, DB는 private과 공유 (DB subnet group이 같은 private subnet 2개 참조)

### CIDR 할당 (VPC `10.0.0.0/16` = 65,536 IPs)
| 서브넷 | AZ | CIDR | IPs | 용도 |
|--------|----|------|----:|------|
| public-a | ap-northeast-2a | 10.0.1.0/24 | 256 | ALB, NAT GW, bastion (옵션) |
| public-c | ap-northeast-2c | 10.0.2.0/24 | 256 | ALB (멀티 AZ 요구) |
| private-a | ap-northeast-2a | 10.0.11.0/24 | 256 | EKS 노드, RDS-a, Redis |
| private-c | ap-northeast-2c | 10.0.12.0/24 | 256 | EKS 노드, RDS-c, Redis |

→ /24 = 256 IPs per subnet. EKS VPC CNI는 Pod당 ENI/IP 소비 — 노드 2~3개 + Pod 30~50개면 충분. 부하 테스트 시 더 필요하면 /22로 확장 가능.

### EKS 자동 발견 태그
- **public subnet**: `kubernetes.io/role/elb = 1` (외부 ALB가 이 서브넷에 placement)
- **private subnet**: `kubernetes.io/role/internal-elb = 1` (내부 ALB용)
- **공통**: `kubernetes.io/cluster/<cluster-name> = shared` (EKS가 이 서브넷을 자기 자원으로 인식)

## Consequences
**+**:
- ALB 멀티 AZ 요구 충족 (최소 2 AZ)
- EKS 노드 그룹이 멀티 AZ 자동 분산 → AZ 장애 시 한쪽이 살아있음
- DB subnet group이 private subnet 재사용 — 추가 서브넷 없이 RDS 멀티 AZ 가능
- 단순 구조 = 라우트 테이블 적음 = 디버깅 쉬움
- 표준 EKS 시작 템플릿과 일치 — 다른 사람도 이해하기 쉬움

**−**:
- DB와 EKS 노드가 같은 서브넷 = SG로 트래픽 제어 (DB SG는 EKS 노드 SG에서만 5432 허용)
- 3-AZ 가용성(99.99%) 포기 — 99.9%까지만. 캡스톤엔 충분
- /24 = Pod 30~50개로 충분하지만 100+ Pod 시나리오면 CIDR 확장 필요 (variable 변경)
- DB subnet 별도 분리(6 서브넷)가 운영 정석에 더 가까움 — 학습엔 4 서브넷이 충분

## 검토 일정
Phase D(RDS·ElastiCache 도입) 시 — DB subnet group이 잘 동작하는지 검증 후 재평가.
