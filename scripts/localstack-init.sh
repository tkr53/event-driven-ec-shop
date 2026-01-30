#!/bin/bash
set -e

echo "=== LocalStack Initialization ==="
echo "Setting up AWS resources for local development..."

AWS_REGION="ap-northeast-1"
EVENTS_TABLE_NAME="${DYNAMODB_TABLE_NAME:-events}"
SNAPSHOTS_TABLE_NAME="${DYNAMODB_SNAPSHOT_TABLE_NAME:-snapshots}"
KINESIS_STREAM_NAME="ec-events"
ENDPOINT_URL="${LOCALSTACK_ENDPOINT:-http://localhost:4566}"

# Use awslocal if available, otherwise use aws with endpoint
if command -v awslocal &> /dev/null; then
  AWS_CMD="awslocal"
else
  AWS_CMD="aws --endpoint-url=${ENDPOINT_URL}"
fi

# DynamoDB Events Table
echo "Creating DynamoDB Events table: ${EVENTS_TABLE_NAME}"
$AWS_CMD dynamodb create-table \
  --table-name "${EVENTS_TABLE_NAME}" \
  --attribute-definitions \
    AttributeName=aggregate_id,AttributeType=S \
    AttributeName=version,AttributeType=N \
    AttributeName=gsi1pk,AttributeType=S \
    AttributeName=created_at,AttributeType=S \
  --key-schema \
    AttributeName=aggregate_id,KeyType=HASH \
    AttributeName=version,KeyType=RANGE \
  --global-secondary-indexes \
    "[{\"IndexName\":\"GSI1\",\"KeySchema\":[{\"AttributeName\":\"gsi1pk\",\"KeyType\":\"HASH\"},{\"AttributeName\":\"created_at\",\"KeyType\":\"RANGE\"}],\"Projection\":{\"ProjectionType\":\"ALL\"}}]" \
  --billing-mode PAY_PER_REQUEST \
  --region "${AWS_REGION}" || echo "Events table might already exist"

# DynamoDB Snapshots Table
echo "Creating DynamoDB Snapshots table: ${SNAPSHOTS_TABLE_NAME}"
$AWS_CMD dynamodb create-table \
  --table-name "${SNAPSHOTS_TABLE_NAME}" \
  --attribute-definitions \
    AttributeName=aggregate_id,AttributeType=S \
  --key-schema \
    AttributeName=aggregate_id,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --region "${AWS_REGION}" || echo "Snapshots table might already exist"

# Kinesis Data Stream
echo "Creating Kinesis Data Stream: ${KINESIS_STREAM_NAME}"
$AWS_CMD kinesis create-stream \
  --stream-name "${KINESIS_STREAM_NAME}" \
  --shard-count 1 \
  --region "${AWS_REGION}" || echo "Kinesis stream might already exist"

# Wait for stream to be active
echo "Waiting for Kinesis stream to be active..."
$AWS_CMD kinesis wait stream-exists \
  --stream-name "${KINESIS_STREAM_NAME}" \
  --region "${AWS_REGION}"

# Enable Kinesis Data Stream for DynamoDB
echo "Enabling Kinesis streaming for DynamoDB..."
KINESIS_STREAM_ARN=$($AWS_CMD kinesis describe-stream \
  --stream-name "${KINESIS_STREAM_NAME}" \
  --region "${AWS_REGION}" \
  --query 'StreamDescription.StreamARN' \
  --output text)

$AWS_CMD dynamodb enable-kinesis-streaming-destination \
  --table-name "${EVENTS_TABLE_NAME}" \
  --stream-arn "${KINESIS_STREAM_ARN}" \
  --region "${AWS_REGION}" || echo "Kinesis streaming might already be enabled"

echo "=== LocalStack Initialization Complete ==="
echo "  DynamoDB Events Table: ${EVENTS_TABLE_NAME}"
echo "  DynamoDB Snapshots Table: ${SNAPSHOTS_TABLE_NAME}"
echo "  Kinesis Stream: ${KINESIS_STREAM_NAME}"
echo ""
echo "Endpoint: http://localhost:4566"
