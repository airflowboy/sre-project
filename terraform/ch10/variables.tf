variable "region" {
  description = "AWS region"
  type        = string
  default     = "ap-northeast-2" # Seoul
}

variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
  default     = "sre-roadmap"
}

# Future EKS cluster name — also used in subnet tags so EKS auto-discovers them.
variable "cluster_name" {
  description = "EKS cluster name (also used in kubernetes.io/cluster/<name> subnet tags)"
  type        = string
  default     = "sre-roadmap-ch10"
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}

# Two AZs is the EKS minimum and the standard pattern (ADR-004).
variable "azs" {
  type    = list(string)
  default = ["ap-northeast-2a", "ap-northeast-2c"]
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.0.1.0/24", "10.0.2.0/24"]
}

variable "private_subnet_cidrs" {
  type    = list(string)
  default = ["10.0.11.0/24", "10.0.12.0/24"]
}

# ADR-003: single NAT GW for learning (SPOF accepted, cost ↓).
variable "single_nat_gateway" {
  description = "true = 1 NAT (cheap, SPOF) / false = 1 per AZ (HA, 2x cost). See ADR-003."
  type        = bool
  default     = true
}

# --- Phase B: EKS ---

variable "kubernetes_version" {
  description = "EKS Kubernetes minor version"
  type        = string
  default     = "1.31"
}

variable "node_instance_types" {
  type    = list(string)
  default = ["t3.small"]
}

variable "node_capacity_type" {
  description = "ON_DEMAND (stable) or SPOT (60-90% cheaper, can be reclaimed)"
  type        = string
  default     = "ON_DEMAND"
}

variable "node_desired_size" {
  type    = number
  default = 2
}

variable "node_min_size" {
  type    = number
  default = 1
}

variable "node_max_size" {
  type    = number
  default = 4
}

# --- Phase D: Data layer ---

# ADR-011 — RDS PostgreSQL db.t3.micro (Free Tier 12mo).
variable "db_instance_class" {
  type    = string
  default = "db.t3.micro"
}

variable "db_engine_version" {
  type    = string
  default = "16.10"
}

variable "db_name" {
  type    = string
  default = "issueapi"
}

variable "db_username" {
  type    = string
  default = "issueapi"
}

variable "db_allocated_storage" {
  description = "GB. 20 = Free Tier max for gp3."
  type        = number
  default     = 20
}

# ADR-012 — ElastiCache Redis cache.t3.micro (Free Tier 12mo).
variable "redis_node_type" {
  type    = string
  default = "cache.t3.micro"
}

variable "redis_engine_version" {
  type    = string
  default = "7.1"
}

# Kubernetes ServiceAccount that issue-api will use (Phase D-2).
# Pre-declared here so IRSA trust policy can lock the role to this exact SA.
variable "app_namespace" {
  type    = string
  default = "default"
}

variable "app_service_account" {
  type    = string
  default = "issue-api"
}

# --- Phase D-2: CI / Ingress ---

# ADR-015 — GitHub OIDC trust 조건에 박는 repo 식별자.
variable "github_owner" {
  type    = string
  default = "airflowboy"
}

variable "github_repo" {
  type    = string
  default = "sre-project"
}
