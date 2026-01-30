package kinesis

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

// ConvertFromKinesisRecord converts a Kinesis record (DynamoDB Streams format) to store.Event.
// DynamoDB Kinesis integration sends records in DynamoDB Streams format.
func ConvertFromKinesisRecord(record events.KinesisEventRecord) (*store.Event, error) {
	// Parse the DynamoDB stream record from Kinesis data
	var dynamoDBRecord events.DynamoDBEventRecord
	if err := json.Unmarshal(record.Kinesis.Data, &dynamoDBRecord); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DynamoDB record: %w", err)
	}

	// Only process INSERT events (new events added to the event store)
	if dynamoDBRecord.EventName != "INSERT" {
		return nil, nil
	}

	return convertDynamoDBImage(dynamoDBRecord.Change.NewImage)
}

// ConvertFromDynamoDBStreamRecord converts a DynamoDB Stream record to store.Event.
// This is used when directly consuming from DynamoDB Streams (e.g., in tests).
func ConvertFromDynamoDBStreamRecord(record events.DynamoDBEventRecord) (*store.Event, error) {
	if record.EventName != "INSERT" {
		return nil, nil
	}

	return convertDynamoDBImage(record.Change.NewImage)
}

// convertDynamoDBImage extracts event data from DynamoDB attribute values.
func convertDynamoDBImage(image map[string]events.DynamoDBAttributeValue) (*store.Event, error) {
	if image == nil {
		return nil, fmt.Errorf("DynamoDB image is nil")
	}

	event := &store.Event{}

	// Extract required fields
	if v, ok := image["id"]; ok {
		event.ID = v.String()
	}
	if v, ok := image["aggregate_id"]; ok {
		event.AggregateID = v.String()
	}
	if v, ok := image["aggregate_type"]; ok {
		event.AggregateType = v.String()
	}
	if v, ok := image["event_type"]; ok {
		event.EventType = v.String()
	}
	if v, ok := image["data"]; ok {
		event.Data = json.RawMessage(v.String())
	}
	if v, ok := image["created_at"]; ok {
		t, err := time.Parse(time.RFC3339Nano, v.String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at: %w", err)
		}
		event.Timestamp = t
	}
	if v, ok := image["version"]; ok {
		version, err := v.Integer()
		if err != nil {
			return nil, fmt.Errorf("failed to parse version: %w", err)
		}
		event.Version = int(version)
	}

	// Validate required fields
	if event.ID == "" || event.AggregateID == "" || event.EventType == "" {
		return nil, fmt.Errorf("missing required fields: id=%s, aggregate_id=%s, event_type=%s",
			event.ID, event.AggregateID, event.EventType)
	}

	return event, nil
}

// BatchConvertFromKinesisEvent converts all records from a Kinesis event to store.Events.
// Returns successfully converted events and any errors encountered.
func BatchConvertFromKinesisEvent(kinesisEvent events.KinesisEvent) ([]*store.Event, []error) {
	var eventList []*store.Event
	var errors []error

	for _, record := range kinesisEvent.Records {
		event, err := ConvertFromKinesisRecord(record)
		if err != nil {
			errors = append(errors, fmt.Errorf("record %s: %w", record.EventID, err))
			continue
		}
		if event != nil {
			eventList = append(eventList, event)
		}
	}

	return eventList, errors
}
