output "dynamodb_events_table_name" {
  description = "DynamoDB events table name"
  value       = aws_dynamodb_table.events.name
}

output "dynamodb_events_table_arn" {
  description = "DynamoDB events table ARN"
  value       = aws_dynamodb_table.events.arn
}

output "dynamodb_snapshots_table_name" {
  description = "DynamoDB snapshots table name"
  value       = aws_dynamodb_table.snapshots.name
}

output "kinesis_stream_name" {
  description = "Kinesis Data Stream name"
  value       = aws_kinesis_stream.events.name
}

output "kinesis_stream_arn" {
  description = "Kinesis Data Stream ARN"
  value       = aws_kinesis_stream.events.arn
}

output "lambda_projector_arn" {
  description = "Lambda Projector ARN"
  value       = aws_lambda_function.projector.arn
}

output "lambda_notifier_arn" {
  description = "Lambda Notifier ARN"
  value       = aws_lambda_function.notifier.arn
}

output "event_archive_bucket" {
  description = "S3 bucket for event archive (if enabled)"
  value       = var.s3_bucket_name != "" ? aws_s3_bucket.event_archive[0].id : null
}
