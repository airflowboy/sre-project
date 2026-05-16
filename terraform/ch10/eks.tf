# Ch 10 Phase B — EKS cluster (control plane) + OIDC provider for IRSA.
# See ADR-001 (EKS choice), ADR-007 (OIDC provider in Phase B).

resource "aws_eks_cluster" "main" {
  name     = var.cluster_name
  role_arn = aws_iam_role.cluster.arn
  version  = var.kubernetes_version

  vpc_config {
    # EKS needs subnets in ≥ 2 AZs. We give it BOTH public + private:
    #   - public subnets host ALB + NAT, and EKS places control-plane ENIs here too
    #   - private subnets host worker nodes (see nodes.tf)
    subnet_ids = concat(aws_subnet.public[*].id, aws_subnet.private[*].id)

    # Public endpoint = kubectl from internet (auth still via IAM).
    # Private endpoint = also reachable from within the VPC.
    # Both true is the common learning/staging default; tighten in production.
    endpoint_public_access  = true
    endpoint_private_access = true
  }

  # The IAM policy attachment must exist BEFORE the cluster, or EKS rejects.
  depends_on = [
    aws_iam_role_policy_attachment.cluster_AmazonEKSClusterPolicy,
  ]
}

# --- IRSA OIDC provider (ADR-007) ----------------------------------------
# Lets K8s ServiceAccounts assume IAM Roles directly. Required for the EBS
# CSI driver (Phase D) and any in-Pod AWS SDK call without static credentials.

# Fetch the thumbprint of the EKS OIDC issuer's TLS certificate so the IAM
# provider can validate tokens issued by that endpoint.
data "tls_certificate" "eks" {
  url = aws_eks_cluster.main.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "eks" {
  url             = aws_eks_cluster.main.identity[0].oidc[0].issuer
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.eks.certificates[0].sha1_fingerprint]
}
