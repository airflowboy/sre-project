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

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidr" {
  description = "CIDR block for the public subnet"
  type        = string
  default     = "10.0.1.0/24"
}

variable "az" {
  description = "Availability Zone for the public subnet"
  type        = string
  default     = "ap-northeast-2a"
}

variable "instance_type" {
  description = "EC2 instance type — keep it Free Tier eligible (t2.micro in ap-northeast-2)"
  type        = string
  default     = "t2.micro"
}

variable "public_key_path" {
  description = "Path to the SSH public key to register as an EC2 key pair"
  type        = string
  default     = "~/.ssh/ec2-ch08.pub"
}
