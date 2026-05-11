output "bucket_name" {
  description = "Name of the S3 bucket (Phase A)"
  value       = aws_s3_bucket.hello.bucket
}

output "bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.hello.arn
}
