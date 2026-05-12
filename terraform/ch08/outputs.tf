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

# --- Phase C: compute ---

output "ami_id" {
  description = "AMI used for the EC2 instance"
  value       = data.aws_ami.al2023.id
}

output "instance_id" {
  description = "ID of the EC2 instance"
  value       = aws_instance.web.id
}

output "instance_public_ip" {
  description = "Public IP of the EC2 instance"
  value       = aws_instance.web.public_ip
}

output "instance_public_dns" {
  description = "Public DNS name of the EC2 instance"
  value       = aws_instance.web.public_dns
}

output "ssh_command" {
  description = "Ready-to-paste SSH command"
  value       = "ssh -i ${trimsuffix(var.public_key_path, ".pub")} ec2-user@${aws_instance.web.public_ip}"
}
