# ADR-007: IRSA OIDC Provider = Phase B에 미리 셋업

- **Date**: 2026-05-16
- **Status**: Accepted
- **Phase**: Ch10 Phase B
- **Decider**: geontae

## Context
**IRSA (IAM Roles for Service Accounts)** = K8s ServiceAccount가 AWS IAM Role을 *직접 assume*하는 매커니즘. Pod이 AWS SDK를 통해 S3·Secrets Manager·DynamoDB 등 *환경변수에 키 안 박고* 접근 가능.

작동 원리:
1. EKS 클러스터는 OIDC issuer URL을 가짐
2. AWS IAM에 OIDC provider 등록 → "이 OIDC issuer를 신뢰함"
3. IAM Role의 trust policy에 "이 OIDC + ServiceAccount 조건이면 assume 허용"
4. Pod에 ServiceAccount 부여 + 그 SA에 IAM Role ARN annotation
5. Pod 안의 SDK가 토큰 교환으로 IAM 자격증명 받음

**핵심 질문**: OIDC provider (IAM 자원)를 *언제* 만들 것인가?

## Options
| 옵션 | 시점 |
|------|------|
| **A. Phase B** — 클러스터 만들면서 같이 OIDC provider도 | EKS apply 한 사이클에 다 끝남 |
| B. Phase D — 첫 IAM Role 만들 때 같이 | "필요할 때까지 미루기" 원칙 |
| C. 클러스터 콘솔에서 클릭 한 번 | Terraform 관리 외 — *비추천* (IaC 원칙 깨짐) |

## Decision
**A. Phase B**

## Consequences
**+**:
- Phase D에서 IAM Role을 만들 때 *준비물이 이미 있음* → manifest 1개 추가만으로 IRSA 작동
- Add-ons 중 **EBS CSI driver**(Phase D 도입 예정)가 IRSA 필요 → 미리 준비 매끄러움
- Terraform `data "tls_certificate"`로 thumbprint 자동 추출 → 사람이 직접 안 박음
- OIDC provider 자원 자체 비용 무료
- `aws_iam_openid_connect_provider` 리소스 5줄이면 끝 — 미루지 말고 그냥 함

**−**:
- 첫 IRSA 사용까지 *시간 차*가 있음 (Phase B 만들고 Phase D에서 사용) — 코드 리뷰 시 "왜 이거 있지?"라고 헷갈릴 수 있음 → 주석으로 "Phase D EBS CSI·App IAM Role에서 사용" 명시

## Phase D에서 어떻게 쓰일지 미리보기
```hcl
# Phase D에서 추가될 코드 (예시):
resource "aws_iam_role" "ebs_csi" {
  name = "${var.cluster_name}-ebs-csi"
  assume_role_policy = data.aws_iam_policy_document.ebs_csi_assume.json
}

data "aws_iam_policy_document" "ebs_csi_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.eks.arn]  # ← Phase B 산출
    }
    condition {
      test     = "StringEquals"
      variable = "${replace(aws_iam_openid_connect_provider.eks.url, "https://", "")}:sub"
      values   = ["system:serviceaccount:kube-system:ebs-csi-controller-sa"]
    }
  }
}
```

→ Phase B에 OIDC provider가 *이미 있어서* 이 코드가 즉시 동작.

## 검토 일정
Phase D 끝나고 — 실제 IRSA 사용 경험을 토대로 *환경변수 access key 시절*과 운영 부담·보안 차이 회고.
