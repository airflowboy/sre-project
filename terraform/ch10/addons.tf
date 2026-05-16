# Ch 10 Phase B — AWS-managed EKS Add-ons (ADR-006).
#
# Core cluster components installed and version-managed by AWS:
#   vpc-cni     — assigns VPC IPs to Pods
#   coredns     — in-cluster DNS (e.g., service.namespace.svc.cluster.local)
#   kube-proxy  — translates Service ClusterIP → Pod IP via iptables/IPVS
#
# EBS CSI driver is deferred to Phase D (when we add IRSA + storage).
#
# We don't pin addon_version — AWS auto-picks the version compatible with our
# Kubernetes version. Add `addon_version = "v1.x.x-eksbuild.y"` when you need
# a specific version for reproducibility.

resource "aws_eks_addon" "vpc_cni" {
  cluster_name = aws_eks_cluster.main.name
  addon_name   = "vpc-cni"
}

resource "aws_eks_addon" "coredns" {
  cluster_name = aws_eks_cluster.main.name
  addon_name   = "coredns"

  # CoreDNS runs as Pods → needs worker nodes to schedule on.
  depends_on = [aws_eks_node_group.main]
}

resource "aws_eks_addon" "kube_proxy" {
  cluster_name = aws_eks_cluster.main.name
  addon_name   = "kube-proxy"
}
