# IAM Role for Lambda functions
resource "aws_iam_role" "lambda" {
  name = "${local.name_prefix}-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  tags = local.common_tags
}

# IAM Policy for Lambda functions
resource "aws_iam_role_policy" "lambda" {
  name = "${local.name_prefix}-lambda-policy"
  role = aws_iam_role.lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:${var.aws_region}:${data.aws_caller_identity.current.account_id}:*"
      },
      {
        Effect = "Allow"
        Action = [
          "kinesis:DescribeStream",
          "kinesis:DescribeStreamSummary",
          "kinesis:GetRecords",
          "kinesis:GetShardIterator",
          "kinesis:ListShards",
          "kinesis:ListStreams",
          "kinesis:SubscribeToShard"
        ]
        Resource = aws_kinesis_stream.events.arn
      },
      {
        Effect = "Allow"
        Action = [
          "ec2:CreateNetworkInterface",
          "ec2:DescribeNetworkInterfaces",
          "ec2:DeleteNetworkInterface"
        ]
        Resource = "*"
      }
    ]
  })
}

# Lambda Projector
resource "aws_lambda_function" "projector" {
  function_name = "${local.name_prefix}-projector"
  role          = aws_iam_role.lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]

  filename         = data.archive_file.projector.output_path
  source_code_hash = data.archive_file.projector.output_base64sha256

  memory_size = var.lambda_memory_size
  timeout     = var.lambda_timeout

  environment {
    variables = {
      DATABASE_URL = var.database_url
    }
  }

  tags = local.common_tags
}

data "archive_file" "projector" {
  type        = "zip"
  source_file = "${path.module}/../../dist/lambda/projector/bootstrap"
  output_path = "${path.module}/../../dist/lambda/projector.zip"
}

# Lambda Projector - Kinesis Event Source Mapping
resource "aws_lambda_event_source_mapping" "projector" {
  event_source_arn                   = aws_kinesis_stream.events.arn
  function_name                      = aws_lambda_function.projector.arn
  starting_position                  = "LATEST"
  batch_size                         = 100
  maximum_batching_window_in_seconds = 5
  parallelization_factor             = 1

  function_response_types = ["ReportBatchItemFailures"]
}

# CloudWatch Log Group for Projector
resource "aws_cloudwatch_log_group" "projector" {
  name              = "/aws/lambda/${aws_lambda_function.projector.function_name}"
  retention_in_days = 14

  tags = local.common_tags
}

# Lambda Notifier
resource "aws_lambda_function" "notifier" {
  function_name = "${local.name_prefix}-notifier"
  role          = aws_iam_role.lambda.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  architectures = ["arm64"]

  filename         = data.archive_file.notifier.output_path
  source_code_hash = data.archive_file.notifier.output_base64sha256

  memory_size = var.lambda_memory_size
  timeout     = var.lambda_timeout

  environment {
    variables = {
      DATABASE_URL = var.database_url
      SMTP_HOST    = var.smtp_host
      SMTP_PORT    = var.smtp_port
      SMTP_FROM    = var.smtp_from
    }
  }

  tags = local.common_tags
}

data "archive_file" "notifier" {
  type        = "zip"
  source_file = "${path.module}/../../dist/lambda/notifier/bootstrap"
  output_path = "${path.module}/../../dist/lambda/notifier.zip"
}

# Lambda Notifier - Kinesis Event Source Mapping
resource "aws_lambda_event_source_mapping" "notifier" {
  event_source_arn                   = aws_kinesis_stream.events.arn
  function_name                      = aws_lambda_function.notifier.arn
  starting_position                  = "LATEST"
  batch_size                         = 100
  maximum_batching_window_in_seconds = 5
  parallelization_factor             = 1

  function_response_types = ["ReportBatchItemFailures"]
}

# CloudWatch Log Group for Notifier
resource "aws_cloudwatch_log_group" "notifier" {
  name              = "/aws/lambda/${aws_lambda_function.notifier.function_name}"
  retention_in_days = 14

  tags = local.common_tags
}
