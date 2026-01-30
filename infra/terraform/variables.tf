variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "ap-northeast-1"
}

variable "environment" {
  description = "Environment name (e.g., dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "project_name" {
  description = "Project name"
  type        = string
  default     = "ec-event-driven"
}

variable "kinesis_shard_count" {
  description = "Number of shards for Kinesis Data Stream"
  type        = number
  default     = 1
}

variable "lambda_memory_size" {
  description = "Memory size for Lambda functions (MB)"
  type        = number
  default     = 256
}

variable "lambda_timeout" {
  description = "Timeout for Lambda functions (seconds)"
  type        = number
  default     = 30
}

variable "database_url" {
  description = "PostgreSQL connection string"
  type        = string
  sensitive   = true
}

variable "smtp_host" {
  description = "SMTP server host"
  type        = string
  default     = "localhost"
}

variable "smtp_port" {
  description = "SMTP server port"
  type        = string
  default     = "1025"
}

variable "smtp_from" {
  description = "SMTP from address"
  type        = string
  default     = "noreply@example.com"
}

variable "s3_bucket_name" {
  description = "S3 bucket name for event archive"
  type        = string
  default     = ""
}
