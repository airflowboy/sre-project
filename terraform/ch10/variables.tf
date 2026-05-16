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
# Flip to false for one NAT per AZ (HA + 2x cost).
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

# t3.small = 2GB, ~$0.026/hr. Tight but fine for Phase B-D learning.
# Bump to t3.medium for Phase J load test (or use a separate Spot node group).
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
