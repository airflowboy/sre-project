# Ch 10 Phase A — VPC + multi-AZ subnets + IGW + NAT GW + route tables.
# See ADR-003 (NAT count) and ADR-004 (VPC structure).
#
# Layout (2 AZs, 4 subnets):
#   public-a  (10.0.1.0/24)   ALB, NAT GW
#   public-c  (10.0.2.0/24)   ALB (ALB needs ≥ 2 AZs)
#   private-a (10.0.11.0/24)  EKS nodes, RDS, ElastiCache
#   private-c (10.0.12.0/24)  EKS nodes, RDS, ElastiCache
#
# Subnets are pre-tagged for EKS auto-discovery so Phase B's `aws_eks_cluster`
# resource doesn't need to re-tag them.

resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_support   = true
  enable_dns_hostnames = true # EKS requires DNS hostnames for nodes/pods
  tags                 = { Name = "${var.name_prefix}-vpc" }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = "${var.name_prefix}-igw" }
}

# --- Public subnets (one per AZ) ---

resource "aws_subnet" "public" {
  count                   = length(var.azs)
  vpc_id                  = aws_vpc.main.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = var.azs[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.name_prefix}-public-${substr(var.azs[count.index], -1, 1)}"
    # Tell AWS Load Balancer Controller to use these subnets for INTERNET-FACING ALBs.
    "kubernetes.io/role/elb" = "1"
    # Tell EKS this subnet belongs to our cluster (shared = can hold workloads from any cluster).
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }
}

# --- Private subnets (one per AZ) ---

resource "aws_subnet" "private" {
  count             = length(var.azs)
  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_subnet_cidrs[count.index]
  availability_zone = var.azs[count.index]

  tags = {
    Name = "${var.name_prefix}-private-${substr(var.azs[count.index], -1, 1)}"
    # For INTERNAL ALBs.
    "kubernetes.io/role/internal-elb"           = "1"
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }
}

# --- Public routing: subnet → IGW ---

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
  tags = { Name = "${var.name_prefix}-public-rt" }
}

resource "aws_route_table_association" "public" {
  count          = length(var.azs)
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# --- NAT Gateway(s) for private-subnet outbound internet ---

resource "aws_eip" "nat" {
  count  = var.single_nat_gateway ? 1 : length(var.azs)
  domain = "vpc"
  tags   = { Name = "${var.name_prefix}-nat-eip-${count.index}" }
}

resource "aws_nat_gateway" "main" {
  count         = var.single_nat_gateway ? 1 : length(var.azs)
  allocation_id = aws_eip.nat[count.index].id
  # NAT GW lives in a PUBLIC subnet (so it has internet access).
  subnet_id  = aws_subnet.public[count.index].id
  tags       = { Name = "${var.name_prefix}-nat-${count.index}" }
  depends_on = [aws_internet_gateway.main]
}

# --- Private routing: subnet → NAT GW ---
# Single NAT mode: ONE private route table shared by all private subnets.
# Per-AZ NAT mode: ONE route table per AZ, each pointing to its own NAT.

resource "aws_route_table" "private" {
  count  = var.single_nat_gateway ? 1 : length(var.azs)
  vpc_id = aws_vpc.main.id
  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main[count.index].id
  }
  tags = { Name = "${var.name_prefix}-private-rt-${count.index}" }
}

resource "aws_route_table_association" "private" {
  count          = length(var.azs)
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[var.single_nat_gateway ? 0 : count.index].id
}
