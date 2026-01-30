# Kinesis Data Stream for events
resource "aws_kinesis_stream" "events" {
  name             = "${local.name_prefix}-events"
  shard_count      = var.kinesis_shard_count
  retention_period = 24

  stream_mode_details {
    stream_mode = "PROVISIONED"
  }

  tags = local.common_tags
}

# S3 Bucket for event archive (optional)
resource "aws_s3_bucket" "event_archive" {
  count  = var.s3_bucket_name != "" ? 1 : 0
  bucket = var.s3_bucket_name

  tags = local.common_tags
}

resource "aws_s3_bucket_lifecycle_configuration" "event_archive" {
  count  = var.s3_bucket_name != "" ? 1 : 0
  bucket = aws_s3_bucket.event_archive[0].id

  rule {
    id     = "archive-old-events"
    status = "Enabled"

    transition {
      days          = 30
      storage_class = "STANDARD_IA"
    }

    transition {
      days          = 90
      storage_class = "GLACIER"
    }
  }
}

# Firehose for event archive (optional)
resource "aws_kinesis_firehose_delivery_stream" "event_archive" {
  count       = var.s3_bucket_name != "" ? 1 : 0
  name        = "${local.name_prefix}-event-archive"
  destination = "extended_s3"

  kinesis_source_configuration {
    kinesis_stream_arn = aws_kinesis_stream.events.arn
    role_arn           = aws_iam_role.firehose[0].arn
  }

  extended_s3_configuration {
    role_arn           = aws_iam_role.firehose[0].arn
    bucket_arn         = aws_s3_bucket.event_archive[0].arn
    prefix             = "events/year=!{timestamp:yyyy}/month=!{timestamp:MM}/day=!{timestamp:dd}/"
    error_output_prefix = "errors/!{firehose:error-output-type}/year=!{timestamp:yyyy}/month=!{timestamp:MM}/day=!{timestamp:dd}/"
    buffering_size     = 64
    buffering_interval = 60
    compression_format = "GZIP"
  }

  tags = local.common_tags
}

# IAM Role for Firehose
resource "aws_iam_role" "firehose" {
  count = var.s3_bucket_name != "" ? 1 : 0
  name  = "${local.name_prefix}-firehose-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "firehose.amazonaws.com"
        }
      }
    ]
  })

  tags = local.common_tags
}

resource "aws_iam_role_policy" "firehose" {
  count = var.s3_bucket_name != "" ? 1 : 0
  name  = "${local.name_prefix}-firehose-policy"
  role  = aws_iam_role.firehose[0].id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:AbortMultipartUpload",
          "s3:GetBucketLocation",
          "s3:GetObject",
          "s3:ListBucket",
          "s3:ListBucketMultipartUploads",
          "s3:PutObject"
        ]
        Resource = [
          aws_s3_bucket.event_archive[0].arn,
          "${aws_s3_bucket.event_archive[0].arn}/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "kinesis:DescribeStream",
          "kinesis:GetShardIterator",
          "kinesis:GetRecords",
          "kinesis:ListShards"
        ]
        Resource = aws_kinesis_stream.events.arn
      }
    ]
  })
}
