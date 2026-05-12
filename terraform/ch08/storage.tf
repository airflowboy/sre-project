# Phase A — the first Terraform resources: an S3 bucket (Free Tier, harmless).
# S3 bucket names are GLOBALLY unique, so we append a random suffix.
#
# Phase B will add network.tf (VPC/Subnet/IGW/SG); Phase C compute.tf (EC2).

resource "random_id" "suffix" {
  byte_length = 4
}

resource "aws_s3_bucket" "hello" {
  bucket = "${var.name_prefix}-tf-hello-${random_id.suffix.hex}"
}

# Sensible default for any bucket: block all public access.
resource "aws_s3_bucket_public_access_block" "hello" {
  bucket                  = aws_s3_bucket.hello.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
