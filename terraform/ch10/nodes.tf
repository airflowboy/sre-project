# Ch 10 Phase B — EKS Managed Node Group (ADR-005).
#
# AWS manages the launch template + ASG. We just say "I want N nodes of type X
# in these subnets" and EKS takes care of joining them to the cluster.

resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "${var.cluster_name}-ng"
  node_role_arn   = aws_iam_role.node.arn

  # Workers live in PRIVATE subnets — no direct public IP, outbound via NAT.
  # ALB will reach them via target groups (Phase D).
  subnet_ids = aws_subnet.private[*].id

  instance_types = var.node_instance_types
  capacity_type  = var.node_capacity_type # ON_DEMAND or SPOT

  scaling_config {
    desired_size = var.node_desired_size
    min_size     = var.node_min_size
    max_size     = var.node_max_size
  }

  # Rolling updates: take at most 1 node out at a time.
  update_config {
    max_unavailable = 1
  }

  # The node IAM role's policy attachments must exist before EKS tries to
  # register nodes — otherwise nodes fail to join with permission errors.
  depends_on = [
    aws_iam_role_policy_attachment.node_AmazonEKSWorkerNodePolicy,
    aws_iam_role_policy_attachment.node_AmazonEKS_CNI_Policy,
    aws_iam_role_policy_attachment.node_AmazonEC2ContainerRegistryReadOnly,
  ]
}
