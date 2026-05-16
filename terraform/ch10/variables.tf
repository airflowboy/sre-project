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
