# --- Phase A: network ---

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "vpc_cidr" {
  value = aws_vpc.main.cidr_block
}

output "public_subnet_ids" {
  description = "Public subnet IDs (for ALB, NAT GW)"
  value       = aws_subnet.public[*].id
}

output "private_subnet_ids" {
  description = "Private subnet IDs (for EKS nodes, RDS, ElastiCache)"
  value       = aws_subnet.private[*].id
}

output "nat_gateway_public_ips" {
  description = "Public IPs of NAT Gateways (private subnets' outbound IP)"
  value       = aws_eip.nat[*].public_ip
}

output "azs_used" {
  value = var.azs
}

# --- Phase B: EKS ---

output "eks_cluster_name" {
  value = aws_eks_cluster.main.name
}

output "eks_cluster_endpoint" {
  description = "EKS API server endpoint"
  value       = aws_eks_cluster.main.endpoint
}

output "eks_cluster_version" {
  value = aws_eks_cluster.main.version
}

output "eks_oidc_provider_arn" {
  description = "OIDC provider ARN — referenced by IRSA roles"
  value       = aws_iam_openid_connect_provider.eks.arn
}

output "eks_oidc_issuer" {
  description = "OIDC issuer URL — used in IRSA trust policy conditions"
  value       = aws_eks_cluster.main.identity[0].oidc[0].issuer
}

output "node_group_arn" {
  value = aws_eks_node_group.main.arn
}

output "kubeconfig_command" {
  description = "Run this once after apply to wire up kubectl"
  value       = "aws eks update-kubeconfig --name ${aws_eks_cluster.main.name} --region ${var.region}"
}

# --- Phase D: Data layer ---

output "rds_endpoint" {
  description = "PostgreSQL endpoint (host:port)"
  value       = "${aws_db_instance.postgres.address}:${aws_db_instance.postgres.port}"
}

output "rds_db_name" {
  value = aws_db_instance.postgres.db_name
}

output "redis_endpoint" {
  description = "ElastiCache Redis primary endpoint (single-node cluster)"
  value       = "${aws_elasticache_cluster.redis.cache_nodes[0].address}:${aws_elasticache_cluster.redis.port}"
}

output "secret_db_password_arn" {
  value = aws_secretsmanager_secret.db_password.arn
}

output "secret_db_url_arn" {
  value = aws_secretsmanager_secret.db_url.arn
}

# IRSA role for issue-api Pod (Phase D-2 ServiceAccount will reference this).
output "issue_api_role_arn" {
  description = "IRSA role ARN — set as eks.amazonaws.com/role-arn annotation on the SA"
  value       = aws_iam_role.issue_api.arn
}

output "issue_api_role_service_account" {
  description = "K8s SA that may assume the role"
  value       = "${var.app_namespace}/${var.app_service_account}"
}

# --- Phase D-2: CI / Ingress ---

output "ecr_repository_url" {
  description = "ECR repo URL for issue-api"
  value       = aws_ecr_repository.issue_api.repository_url
}

output "ecr_consumer_repository_url" {
  description = "ECR repo URL for issuance-consumer (Phase E-1)"
  value       = aws_ecr_repository.issuance_consumer.repository_url
}

output "consumer_role_arn" {
  description = "IRSA role ARN for issuance-consumer SA (Phase E-1)"
  value       = aws_iam_role.consumer.arn
}

output "github_actions_role_arn" {
  description = "IAM role for GitHub Actions OIDC — paste into workflow as role-to-assume"
  value       = aws_iam_role.github_actions.arn
}

output "alb_controller_role_arn" {
  description = "IRSA role ARN for aws-load-balancer-controller SA (kube-system)"
  value       = aws_iam_role.alb_controller.arn
}
