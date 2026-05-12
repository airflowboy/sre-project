output "bucket_name" {
  description = "Name of the S3 bucket (Phase A)"
  value       = aws_s3_bucket.hello.bucket
}

output "bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.hello.arn
}

# --- Phase B: network ---

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "public_subnet_id" {
  description = "ID of the public subnet"
  value       = aws_subnet.public.id
}

output "security_group_id" {
  description = "ID of the web security group"
  value       = aws_security_group.web.id
}

output "my_ip_cidr" {
  description = "Public IP allowed for SSH (detected)"
  value       = local.my_ip_cidr
}
