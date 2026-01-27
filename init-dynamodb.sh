#!/bin/bash

# DynamoDB Local table initialization script
# Usage: ./init-dynamodb.sh [endpoint]

ENDPOINT="${1:-http://localhost:8000}"
TABLE_NAME="events"
SNAPSHOT_TABLE_NAME="snapshots"
REGION="ap-northeast-1"

# Wait for DynamoDB Local to be ready
echo "Waiting for DynamoDB Local to be ready at ${ENDPOINT}..."
until curl -s "${ENDPOINT}/shell/" > /dev/null 2>&1; do
    sleep 1
done
echo "DynamoDB Local is ready!"

echo "Creating DynamoDB table: ${TABLE_NAME}"

# Create the events table with GSI for GetAllEvents
aws dynamodb create-table \
    --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    --table-name "${TABLE_NAME}" \
    --attribute-definitions \
        AttributeName=aggregate_id,AttributeType=S \
        AttributeName=version,AttributeType=N \
        AttributeName=gsi1pk,AttributeType=S \
        AttributeName=created_at,AttributeType=S \
    --key-schema \
        AttributeName=aggregate_id,KeyType=HASH \
        AttributeName=version,KeyType=RANGE \
    --global-secondary-indexes \
        '[
            {
                "IndexName": "GSI1",
                "KeySchema": [
                    {"AttributeName": "gsi1pk", "KeyType": "HASH"},
                    {"AttributeName": "created_at", "KeyType": "RANGE"}
                ],
                "Projection": {"ProjectionType": "ALL"},
                "ProvisionedThroughput": {"ReadCapacityUnits": 5, "WriteCapacityUnits": 5}
            }
        ]' \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --no-cli-pager 2>/dev/null

if [ $? -eq 0 ]; then
    echo "Table '${TABLE_NAME}' created successfully!"
else
    echo "Table already exists or creation failed. Checking status..."
fi

# Wait for table to become active
aws dynamodb wait table-exists \
    --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    --table-name "${TABLE_NAME}" \
    --no-cli-pager 2>/dev/null

echo "Table is ready!"

# Show events table info
aws dynamodb describe-table \
    --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    --table-name "${TABLE_NAME}" \
    --query 'Table.{Name:TableName,Status:TableStatus}' \
    --output table \
    --no-cli-pager 2>/dev/null

# Create the snapshots table
echo "Creating DynamoDB table: ${SNAPSHOT_TABLE_NAME}"

aws dynamodb create-table \
    --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    --table-name "${SNAPSHOT_TABLE_NAME}" \
    --attribute-definitions \
        AttributeName=aggregate_id,AttributeType=S \
    --key-schema \
        AttributeName=aggregate_id,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --no-cli-pager 2>/dev/null

if [ $? -eq 0 ]; then
    echo "Table '${SNAPSHOT_TABLE_NAME}' created successfully!"
else
    echo "Table already exists or creation failed. Checking status..."
fi

# Wait for snapshots table to become active
aws dynamodb wait table-exists \
    --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    --table-name "${SNAPSHOT_TABLE_NAME}" \
    --no-cli-pager 2>/dev/null

echo "Snapshots table is ready!"

# Show snapshots table info
aws dynamodb describe-table \
    --endpoint-url "${ENDPOINT}" \
    --region "${REGION}" \
    --table-name "${SNAPSHOT_TABLE_NAME}" \
    --query 'Table.{Name:TableName,Status:TableStatus}' \
    --output table \
    --no-cli-pager 2>/dev/null
