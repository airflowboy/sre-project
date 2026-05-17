# Ch 10 Phase D — ElastiCache Redis (ADR-012).
#
# Single cache.t3.micro node, no AUTH, no TLS — all justified by ADR-012 for
# the learning environment. SG-restricted to EKS cluster traffic.

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.cluster_name}-redis-subnets"
  subnet_ids = aws_subnet.private[*].id
}

# Same SG pattern as RDS: only EKS cluster SG can hit 6379.
resource "aws_security_group" "redis" {
  name        = "${var.cluster_name}-redis-sg"
  description = "Redis - accept 6379 from EKS cluster SG only"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Redis from EKS pods"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.main.vpc_config[0].cluster_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.cluster_name}-redis-sg" }
}

resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "${var.cluster_name}-redis"
  engine               = "redis"
  engine_version       = var.redis_engine_version
  node_type            = var.redis_node_type
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  subnet_group_name    = aws_elasticache_subnet_group.redis.name
  security_group_ids   = [aws_security_group.redis.id]
  port                 = 6379

  # No snapshots in the learning env so destroy is instant.
  snapshot_retention_limit = 0

  tags = { Name = "${var.cluster_name}-redis" }
}
