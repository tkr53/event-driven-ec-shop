# CloudWatch Alarms for monitoring

# Kinesis - Iterator Age Alarm (measures lag)
resource "aws_cloudwatch_metric_alarm" "kinesis_iterator_age" {
  alarm_name          = "${local.name_prefix}-kinesis-iterator-age"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "GetRecords.IteratorAgeMilliseconds"
  namespace           = "AWS/Kinesis"
  period              = 60
  statistic           = "Maximum"
  threshold           = 60000 # 1 minute
  alarm_description   = "Kinesis consumer lag is too high"

  dimensions = {
    StreamName = aws_kinesis_stream.events.name
  }

  tags = local.common_tags
}

# Lambda Projector - Error Rate Alarm
resource "aws_cloudwatch_metric_alarm" "projector_errors" {
  alarm_name          = "${local.name_prefix}-projector-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 5
  alarm_description   = "Lambda Projector error rate is too high"

  dimensions = {
    FunctionName = aws_lambda_function.projector.function_name
  }

  tags = local.common_tags
}

# Lambda Projector - Duration Alarm
resource "aws_cloudwatch_metric_alarm" "projector_duration" {
  alarm_name          = "${local.name_prefix}-projector-duration"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  metric_name         = "Duration"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Average"
  threshold           = var.lambda_timeout * 1000 * 0.8 # 80% of timeout
  alarm_description   = "Lambda Projector is approaching timeout"

  dimensions = {
    FunctionName = aws_lambda_function.projector.function_name
  }

  tags = local.common_tags
}

# Lambda Notifier - Error Rate Alarm
resource "aws_cloudwatch_metric_alarm" "notifier_errors" {
  alarm_name          = "${local.name_prefix}-notifier-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 5
  alarm_description   = "Lambda Notifier error rate is too high"

  dimensions = {
    FunctionName = aws_lambda_function.notifier.function_name
  }

  tags = local.common_tags
}

# DynamoDB - Throttled Requests Alarm
resource "aws_cloudwatch_metric_alarm" "dynamodb_throttles" {
  alarm_name          = "${local.name_prefix}-dynamodb-throttles"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "ThrottledRequests"
  namespace           = "AWS/DynamoDB"
  period              = 60
  statistic           = "Sum"
  threshold           = 0
  alarm_description   = "DynamoDB is experiencing throttling"

  dimensions = {
    TableName = aws_dynamodb_table.events.name
  }

  tags = local.common_tags
}
