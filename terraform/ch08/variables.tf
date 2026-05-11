variable "region" {
  description = "AWS region to deploy into"
  type        = string
  default     = "ap-northeast-2" # Seoul
}

variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
  default     = "sre-roadmap"
}
