#!/bin/bash
set -e

echo "=== Deploying Lambda Functions to LocalStack ==="

AWS_REGION="ap-northeast-1"
LOCALSTACK_ENDPOINT="http://localhost:4566"
KINESIS_STREAM_NAME="ec-events"

# Use awslocal if available, otherwise use aws with endpoint
if command -v awslocal &> /dev/null; then
  AWS_CMD="awslocal"
else
  AWS_CMD="aws --endpoint-url=${LOCALSTACK_ENDPOINT}"
fi

# Check if dist/lambda directory exists
if [ ! -d "dist/lambda" ]; then
  echo "Error: dist/lambda directory not found. Run 'make build-lambda' first."
  exit 1
fi

# Create IAM role for Lambda
echo "Creating Lambda execution role..."
$AWS_CMD iam create-role \
  --role-name lambda-execution-role \
  --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}' \
  --region "${AWS_REGION}" 2>/dev/null || echo "Role might already exist"

ROLE_ARN="arn:aws:iam::000000000000:role/lambda-execution-role"

# Deploy Projector Lambda
echo "Deploying Lambda Projector..."
cd dist/lambda/projector
zip -j projector.zip bootstrap
$AWS_CMD lambda create-function \
  --function-name ec-projector \
  --runtime provided.al2023 \
  --handler bootstrap \
  --architectures arm64 \
  --role "${ROLE_ARN}" \
  --zip-file fileb://projector.zip \
  --environment "Variables={DATABASE_URL=postgres://ecapp:ecapp@host.docker.internal:5432/ecapp?sslmode=disable}" \
  --region "${AWS_REGION}" 2>/dev/null || \
$AWS_CMD lambda update-function-code \
  --function-name ec-projector \
  --zip-file fileb://projector.zip \
  --region "${AWS_REGION}"
cd -

# Deploy Notifier Lambda
echo "Deploying Lambda Notifier..."
cd dist/lambda/notifier
zip -j notifier.zip bootstrap
$AWS_CMD lambda create-function \
  --function-name ec-notifier \
  --runtime provided.al2023 \
  --handler bootstrap \
  --architectures arm64 \
  --role "${ROLE_ARN}" \
  --zip-file fileb://notifier.zip \
  --environment "Variables={DATABASE_URL=postgres://ecapp:ecapp@host.docker.internal:5432/ecapp?sslmode=disable,SMTP_HOST=host.docker.internal,SMTP_PORT=1025,SMTP_FROM=noreply@example.com}" \
  --region "${AWS_REGION}" 2>/dev/null || \
$AWS_CMD lambda update-function-code \
  --function-name ec-notifier \
  --zip-file fileb://notifier.zip \
  --region "${AWS_REGION}"
cd -

# Get Kinesis Stream ARN
KINESIS_STREAM_ARN=$($AWS_CMD kinesis describe-stream \
  --stream-name "${KINESIS_STREAM_NAME}" \
  --region "${AWS_REGION}" \
  --query 'StreamDescription.StreamARN' \
  --output text)

# Create Event Source Mapping for Projector
echo "Creating Kinesis event source mapping for Projector..."
$AWS_CMD lambda create-event-source-mapping \
  --function-name ec-projector \
  --event-source-arn "${KINESIS_STREAM_ARN}" \
  --starting-position LATEST \
  --batch-size 100 \
  --region "${AWS_REGION}" 2>/dev/null || echo "Event source mapping might already exist"

# Create Event Source Mapping for Notifier
echo "Creating Kinesis event source mapping for Notifier..."
$AWS_CMD lambda create-event-source-mapping \
  --function-name ec-notifier \
  --event-source-arn "${KINESIS_STREAM_ARN}" \
  --starting-position LATEST \
  --batch-size 100 \
  --region "${AWS_REGION}" 2>/dev/null || echo "Event source mapping might already exist"

echo "=== Lambda Deployment Complete ==="
echo ""
echo "Deployed functions:"
$AWS_CMD lambda list-functions \
  --region "${AWS_REGION}" \
  --query 'Functions[*].FunctionName' \
  --output text
