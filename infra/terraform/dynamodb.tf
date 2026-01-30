# DynamoDB Events Table
resource "aws_dynamodb_table" "events" {
  name         = "${local.name_prefix}-events"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "aggregate_id"
  range_key    = "version"

  attribute {
    name = "aggregate_id"
    type = "S"
  }

  attribute {
    name = "version"
    type = "N"
  }

  attribute {
    name = "gsi1pk"
    type = "S"
  }

  attribute {
    name = "created_at"
    type = "S"
  }

  global_secondary_index {
    name            = "GSI1"
    hash_key        = "gsi1pk"
    range_key       = "created_at"
    projection_type = "ALL"
  }

  tags = local.common_tags
}

# DynamoDB Snapshots Table
resource "aws_dynamodb_table" "snapshots" {
  name         = "${local.name_prefix}-snapshots"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "aggregate_id"

  attribute {
    name = "aggregate_id"
    type = "S"
  }

  tags = local.common_tags
}

# Enable Kinesis Data Streams for DynamoDB
resource "aws_dynamodb_kinesis_streaming_destination" "events_stream" {
  stream_arn = aws_kinesis_stream.events.arn
  table_name = aws_dynamodb_table.events.name

  depends_on = [aws_kinesis_stream.events]
}
