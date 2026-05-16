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

output "cluster_name" {
  description = "EKS cluster name (already baked into subnet tags, used by Phase B)"
  value       = var.cluster_name
}
