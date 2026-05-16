# ADR-006: EKS Add-ons 관리 = AWS-managed Add-ons

- **Date**: 2026-05-16
- **Status**: Accepted
- **Phase**: Ch10 Phase B
- **Decider**: geontae

## Context
EKS 클러스터에 필수 컴포넌트(VPC CNI, CoreDNS, kube-proxy, 후에 EBS CSI driver 등) 설치 방식 결정.

## Options
| 옵션 | 업그레이드 | 호환성 검증 | 운영 부담 |
|------|:--------:|:--------:|:--------:|
| **AWS-managed EKS Add-ons** | 콘솔/Terraform 한 줄로 버전 변경, AWS가 검증 | AWS가 K8s 버전과 호환 매트릭스 자동 체크 | ↓ |
| Self-managed (Helm/manifest) | 우리가 manifest 끌어와서 적용 | 우리가 호환성 확인 | ↑ |
| 혼합 | 일부 매니지드, 일부 self-managed | 분리 관리 | 중간 |

## Decision
**AWS-managed Add-ons**. Phase B에서 설치할 3종:
- **vpc-cni** — Pod이 VPC IP 받게 (EKS의 기본 네트워킹)
- **coredns** — 클러스터 DNS
- **kube-proxy** — Service의 iptables/IPVS 룰 관리

Phase D에서 추가: **aws-ebs-csi-driver** — PV/PVC를 EBS 볼륨으로 (IRSA 필요)

## Consequences
**+**:
- `aws_eks_addon` Terraform 리소스 3줄로 설치
- 클러스터 버전 업그레이드 시 add-on도 *호환되는 최신*으로 같이
- AWS Support가 add-on 자체 이슈 다룸 (이슈 발생 시 책임 분리)
- IRSA 통합이 매끄러움 (`service_account_role_arn` 옵션)

**−**:
- AWS-managed 옵션만 사용 가능 (커스텀 빌드 X) — 캡스톤 범위에선 무관
- Tigera Operator(Calico) 같은 *third-party 매니지드 add-on*은 별도 marketplace 거쳐야 — Phase I에서 NetworkPolicy 도입 시 Calico VPC CNI add-on을 매니지드로 (또는 Helm)
- 버전 핀 못 하면 자동 업그레이드가 호환성 깨질 가능성 — 우리는 `addon_version` 명시 안 하고 AWS 권장 사용

## 매니지드 vs Self-managed의 실용적 판단
- **매니지드**: 코어 컴포넌트 (CNI, DNS, kube-proxy, CSI) — *클러스터 자체와 묶여있는 것*
- **Self-managed (Helm)**: 워크로드 옆 컴포넌트 (ingress-nginx, cert-manager, ArgoCD, Prometheus) — *우리가 운영의 *고객*인 것*

→ Phase B는 매니지드 3종. Helm 설치는 Phase D/G/H에서.

## 면접 답
"EKS 코어 컴포넌트(CNI, DNS, kube-proxy)는 AWS-managed Add-ons. 업그레이드 호환성을 AWS가 보장하고 IRSA 통합도 매끄러워서. ingress-nginx·ArgoCD·Prometheus 같은 *워크로드 옆 컴포넌트*는 Helm으로 — 운영의 주체가 우리니까."

## 검토 일정
Phase I(보안)에서 — NetworkPolicy 도입 시 Calico VPC CNI를 매니지드 add-on으로 갈지 self-managed Helm으로 갈지 재평가.
