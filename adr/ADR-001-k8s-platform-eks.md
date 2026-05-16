# ADR-001: 캡스톤 K8s 플랫폼 = EKS

- **Date**: 2026-05-15
- **Status**: Accepted
- **Phase**: Ch10 마스터 (캡스톤 전반)
- **Decider**: geontae

## Context
캡스톤(100만 동시접속 시뮬레이션 + 봇 탐지 + Self-monitoring)에 K8s 필요. AWS 위에 구축. Ch 03·05에서 kubeadm 자체구축·Ansible 자동화 학습은 이미 확보됨.

## Options
| 옵션 | 학습 가치 | AWS 통합 | 운영 부담 | 비용 |
|------|:---:|:---:|:---:|:---:|
| **kubeadm on EC2** | ↑ (이미 Ch 03 완료) | 수동 | ↑ | EC2 비용 |
| **EKS (managed)** | AWS 매니지드 운영 (신규) | **즉시** (IRSA/ALB Ctrl/CSI) | ↓ | + $0.10/hr control plane |
| **k3s on EC2** | 가벼움 | 수동 | △ | EC2 비용 |

## Decision
**EKS**

## Consequences
**+**:
- IRSA로 Pod이 IAM Role 직접 assume → Secrets Manager·S3·DynamoDB 접근이 manifest 1줄
- ALB Load Balancer Controller로 Ingress가 ALB target group 자동 관리
- EBS CSI driver로 PV 즉시
- 실무 표준 — 대부분 회사가 매니지드 K8s 사용
- Ch 03·05 학습은 별도 자산 (홈랩 kubeadm + Ansible 자동화 = 차별화는 이미 확보)

**−**:
- Control plane 시간당 $0.10 비용 (24/7 = $72/월). → 매 세션 destroy 패턴 엄수 + AWS Budgets 임계 $20
- Control plane 내부에 접근 불가 → chaos engineering은 worker 죽이기만 가능
- EKS API 일부 매니지드 잠금-in (K8s 표준 API는 호환되어 다른 매니지드 이전 가능)

## 면접 답
"Ch 03에서 kubeadm 자체구축해서 control plane 동작 원리 이해했고, 캡스톤은 실무 표준에 맞춰 EKS로 선택했습니다. IRSA 같은 AWS 통합 학습이 우선이었고, 두 방식을 직접 비교한 게 차별화 포인트입니다."

## 검토 일정
캡스톤 Phase J(부하 테스트) 후 — 운영 경험을 토대로 "다음 프로젝트라면 EKS Fargate? 또는 ROSA?" 같은 재평가 가능.
