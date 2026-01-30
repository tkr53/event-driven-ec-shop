package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// DynamoEventStore stores events in DynamoDB.
// Events are automatically streamed to Kinesis Data Streams via DynamoDB Kinesis integration.
type DynamoEventStore struct {
	client            *dynamodb.Client
	tableName         string
	snapshotTableName string
}

// dynamoEvent represents the DynamoDB item structure
type dynamoEvent struct {
	AggregateID   string `dynamodbav:"aggregate_id"`
	Version       int    `dynamodbav:"version"`
	ID            string `dynamodbav:"id"`
	AggregateType string `dynamodbav:"aggregate_type"`
	EventType     string `dynamodbav:"event_type"`
	Data          string `dynamodbav:"data"`
	CreatedAt     string `dynamodbav:"created_at"`
	GSI1PK        string `dynamodbav:"gsi1pk"`
}

func NewDynamoEventStore(client *dynamodb.Client, tableName, snapshotTableName string) *DynamoEventStore {
	return &DynamoEventStore{
		client:            client,
		tableName:         tableName,
		snapshotTableName: snapshotTableName,
	}
}

// Append stores an event in DynamoDB.
// Events are automatically streamed to Kinesis via DynamoDB Kinesis integration.
func (es *DynamoEventStore) Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*Event, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	eventID := uuid.New().String()
	timestamp := time.Now()

	// Get current max version for the aggregate
	version, err := es.getNextVersion(ctx, aggregateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next version: %w", err)
	}

	item := dynamoEvent{
		AggregateID:   aggregateID,
		Version:       version,
		ID:            eventID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          string(jsonData),
		CreatedAt:     timestamp.Format(time.RFC3339Nano),
		GSI1PK:        "EVENTS", // Fixed value for GSI1 to enable GetAllEvents
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Use conditional write to prevent duplicate versions (optimistic locking)
	_, err = es.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(es.tableName),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(aggregate_id) AND attribute_not_exists(version)"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to put event: %w", err)
	}

	return &Event{
		ID:            eventID,
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          jsonData,
		Timestamp:     timestamp,
		Version:       version,
	}, nil
}

// getNextVersion queries for the current max version and returns the next one
func (es *DynamoEventStore) getNextVersion(ctx context.Context, aggregateID string) (int, error) {
	result, err := es.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		KeyConditionExpression: aws.String("aggregate_id = :aid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":aid": &types.AttributeValueMemberS{Value: aggregateID},
		},
		ScanIndexForward: aws.Bool(false), // Descending order
		Limit:            aws.Int32(1),
		ProjectionExpression: aws.String("version"),
	})
	if err != nil {
		return 0, err
	}

	if len(result.Items) == 0 {
		return 1, nil
	}

	var item struct {
		Version int `dynamodbav:"version"`
	}
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		return 0, err
	}

	return item.Version + 1, nil
}

// GetEvents returns all events for an aggregate from DynamoDB
func (es *DynamoEventStore) GetEvents(aggregateID string) []Event {
	ctx := context.Background()

	result, err := es.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		KeyConditionExpression: aws.String("aggregate_id = :aid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":aid": &types.AttributeValueMemberS{Value: aggregateID},
		},
		ScanIndexForward: aws.Bool(true), // Ascending order by version
	})
	if err != nil {
		return nil
	}

	return es.unmarshalEvents(result.Items)
}

// GetAllEvents returns all events from DynamoDB using GSI1
func (es *DynamoEventStore) GetAllEvents() []Event {
	ctx := context.Background()

	result, err := es.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("gsi1pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "EVENTS"},
		},
		ScanIndexForward: aws.Bool(true), // Ascending order by created_at
	})
	if err != nil {
		return nil
	}

	return es.unmarshalEvents(result.Items)
}

// unmarshalEvents converts DynamoDB items to Event slice
func (es *DynamoEventStore) unmarshalEvents(items []map[string]types.AttributeValue) []Event {
	events := make([]Event, 0, len(items))

	for _, item := range items {
		var de dynamoEvent
		if err := attributevalue.UnmarshalMap(item, &de); err != nil {
			continue
		}

		timestamp, _ := time.Parse(time.RFC3339Nano, de.CreatedAt)

		events = append(events, Event{
			ID:            de.ID,
			AggregateID:   de.AggregateID,
			AggregateType: de.AggregateType,
			EventType:     de.EventType,
			Data:          json.RawMessage(de.Data),
			Timestamp:     timestamp,
			Version:       de.Version,
		})
	}

	return events
}

// dynamoSnapshot represents the DynamoDB item structure for snapshots
// Stored in a separate snapshots table with aggregate_id as partition key
type dynamoSnapshot struct {
	AggregateID   string `dynamodbav:"aggregate_id"`
	AggregateType string `dynamodbav:"aggregate_type"`
	Version       int    `dynamodbav:"version"`   // Event version at snapshot time
	State         string `dynamodbav:"state"`     // Serialized aggregate state
	CreatedAt     string `dynamodbav:"created_at"`
}

// SaveSnapshot stores a snapshot in the dedicated snapshots table
func (es *DynamoEventStore) SaveSnapshot(ctx context.Context, snapshot *Snapshot) error {
	stateJSON, err := json.Marshal(snapshot.State)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot state: %w", err)
	}

	item := dynamoSnapshot{
		AggregateID:   snapshot.AggregateID,
		AggregateType: snapshot.AggregateType,
		Version:       snapshot.Version,
		State:         string(stateJSON),
		CreatedAt:     snapshot.CreatedAt.Format(time.RFC3339Nano),
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Overwrite existing snapshot (no condition)
	_, err = es.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(es.snapshotTableName),
		Item:      av,
	})
	if err != nil {
		return fmt.Errorf("failed to put snapshot: %w", err)
	}

	return nil
}

// GetSnapshot retrieves the latest snapshot for an aggregate from the snapshots table
func (es *DynamoEventStore) GetSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error) {
	result, err := es.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(es.snapshotTableName),
		Key: map[string]types.AttributeValue{
			"aggregate_id": &types.AttributeValueMemberS{Value: aggregateID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	if result.Item == nil {
		return nil, nil // No snapshot exists
	}

	var ds dynamoSnapshot
	if err := attributevalue.UnmarshalMap(result.Item, &ds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	createdAt, _ := time.Parse(time.RFC3339Nano, ds.CreatedAt)

	return &Snapshot{
		AggregateID:   ds.AggregateID,
		AggregateType: ds.AggregateType,
		Version:       ds.Version,
		State:         json.RawMessage(ds.State),
		CreatedAt:     createdAt,
	}, nil
}

// GetEventsFromVersion returns events for an aggregate starting from a specific version
func (es *DynamoEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) []Event {
	result, err := es.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(es.tableName),
		KeyConditionExpression: aws.String("aggregate_id = :aid AND version > :ver"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":aid": &types.AttributeValueMemberS{Value: aggregateID},
			":ver": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", fromVersion)},
		},
		ScanIndexForward: aws.Bool(true), // Ascending order by version
	})
	if err != nil {
		return nil
	}

	return es.unmarshalEvents(result.Items)
}
