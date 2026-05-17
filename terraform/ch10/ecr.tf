# Ch 10 Phase D-2 - ECR repository for issue-api image (ADR-015).
#
# EKS node IAM role already has AmazonEC2ContainerRegistryReadOnly (Phase B),
# so the kubelet can pull from this repo with zero extra config. CI pushes
# via GitHub OIDC (see github_oidc.tf).

resource "aws_ecr_repository" "issue_api" {
  name                 = "${var.cluster_name}/issue-api"
  image_tag_mutability = "MUTABLE" # learning - prod should be IMMUTABLE
  force_delete         = true      # learning - allow destroy with images present

  image_scanning_configuration {
    scan_on_push = true
  }

  tags = { Name = "${var.cluster_name}-issue-api" }
}
