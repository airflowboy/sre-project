# ADR-005: 노드 그룹 종류 = Managed Node Group

- **Date**: 2026-05-16
- **Status**: Accepted
- **Phase**: Ch10 Phase B
- **Decider**: geontae

## Context
EKS 워커 노드를 어떻게 운영할지 결정. control plane은 AWS 매니지드(ADR-001) 확정. 워커 노드는 *별도 선택*.

## Options
| 옵션 | 운영 부담 | 비용 | 유연성 |
|------|:--------:|:----:|:------:|
| **Managed Node Group** | ↓ (AWS가 launch template·ASG 관리, 자동 패치) | EC2 비용만 | EKS 권장 옵션만 가능 |
| Self-managed Node Group | ↑ (직접 ASG·user_data·AMI 관리) | EC2 비용 + 운영 시간 | 최대 — 커스텀 AMI, GPU 등 |
| Fargate | ↓↓ (서버리스, Pod 단위 과금) | Pod당 vCPU·메모리 시간당 (비싸기 쉬움) | DaemonSet 미지원, cold start, 권한 제약 |

## Decision
**Managed Node Group**

## Consequences
**+**:
- ASG·launch template을 AWS가 만들고 관리 — Terraform 코드 단순 (15줄 정도)
- **자동 패치** — Kubernetes 마이너 버전 upgrade가 콘솔 클릭 한 번
- Spot 인스턴스 지원(`capacity_type = "SPOT"`) — 비용 60-90% 절감 가능 (캡스톤 비용 시연 시 활용)
- IAM Role 1개 + 정책 3개(WorkerNode·CNI·ECR ReadOnly)만 붙이면 됨
- 노드 graceful drain 자동 (rolling update 시)

**−**:
- 커스텀 AMI 사용 어려움 (Optimized AMI만 표준) — 캡스톤 범위엔 무관
- GPU·Inferentia 등 특수 인스턴스는 launch template 커스터마이즈 필요 (Bottlerocket으로 가능)
- Fargate 대비 cold start 없지만, 노드 추가는 ~2-3분 (HPA + Cluster Autoscaler 필요)

**Fargate를 안 고른 이유**:
- DaemonSet 미지원 → kube-prometheus-stack의 node-exporter 등 못 깖
- 모든 Pod이 별도 micro-VM → DNS·SG 격리는 좋지만 *Phase F·H의 sidecar/통합 패턴*과 마찰
- Pod 단위 과금이 부하 테스트(Phase J)에서 폭증 가능

## 면접 답
"EKS 매니지드 노드 그룹은 control plane이 매니지드인 거랑 자연스럽게 짝. ASG·패치를 AWS에 위임하고, 우리는 워크로드에 집중. Fargate는 DaemonSet 미지원·cold start로 캡스톤의 부하 테스트·관찰 패턴과 안 맞아서 배제. 운영 시 Spot 비율을 60-70%로 두면 비용 대폭 절감 가능."

## 검토 일정
Phase J(부하 테스트) 후 — Spot 도입 효과·노드 추가 속도(HPA·Cluster Autoscaler) 측정 후 Fargate Pod-level 대비 비교 가능.
