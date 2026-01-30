package kinesis

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertDynamoDBImage(t *testing.T) {
	tests := []struct {
		name    string
		image   map[string]events.DynamoDBAttributeValue
		want    func(*testing.T, *events.DynamoDBAttributeValue)
		wantErr bool
	}{
		{
			name: "valid event",
			image: map[string]events.DynamoDBAttributeValue{
				"id":             events.NewStringAttribute("event-123"),
				"aggregate_id":   events.NewStringAttribute("product-456"),
				"aggregate_type": events.NewStringAttribute("Product"),
				"event_type":     events.NewStringAttribute("ProductCreated"),
				"data":           events.NewStringAttribute(`{"name":"Test Product"}`),
				"created_at":     events.NewStringAttribute("2024-01-15T10:30:00.123456789Z"),
				"version":        events.NewNumberAttribute("1"),
			},
			wantErr: false,
		},
		{
			name:    "nil image",
			image:   nil,
			wantErr: true,
		},
		{
			name: "missing required fields",
			image: map[string]events.DynamoDBAttributeValue{
				"id": events.NewStringAttribute("event-123"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := convertDynamoDBImage(tt.image)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, event)
			assert.Equal(t, "event-123", event.ID)
			assert.Equal(t, "product-456", event.AggregateID)
			assert.Equal(t, "Product", event.AggregateType)
			assert.Equal(t, "ProductCreated", event.EventType)
			assert.Equal(t, 1, event.Version)
		})
	}
}

func TestConvertFromDynamoDBStreamRecord(t *testing.T) {
	t.Run("INSERT event converts successfully", func(t *testing.T) {
		record := events.DynamoDBEventRecord{
			EventName: "INSERT",
			Change: events.DynamoDBStreamRecord{
				NewImage: map[string]events.DynamoDBAttributeValue{
					"id":             events.NewStringAttribute("event-123"),
					"aggregate_id":   events.NewStringAttribute("product-456"),
					"aggregate_type": events.NewStringAttribute("Product"),
					"event_type":     events.NewStringAttribute("ProductCreated"),
					"data":           events.NewStringAttribute(`{"name":"Test"}`),
					"created_at":     events.NewStringAttribute(time.Now().Format(time.RFC3339Nano)),
					"version":        events.NewNumberAttribute("1"),
				},
			},
		}

		event, err := ConvertFromDynamoDBStreamRecord(record)
		require.NoError(t, err)
		require.NotNil(t, event)
		assert.Equal(t, "event-123", event.ID)
	})

	t.Run("MODIFY event returns nil", func(t *testing.T) {
		record := events.DynamoDBEventRecord{
			EventName: "MODIFY",
		}

		event, err := ConvertFromDynamoDBStreamRecord(record)
		require.NoError(t, err)
		assert.Nil(t, event)
	})

	t.Run("REMOVE event returns nil", func(t *testing.T) {
		record := events.DynamoDBEventRecord{
			EventName: "REMOVE",
		}

		event, err := ConvertFromDynamoDBStreamRecord(record)
		require.NoError(t, err)
		assert.Nil(t, event)
	})
}

func TestConvertFromKinesisRecord(t *testing.T) {
	t.Run("valid Kinesis record", func(t *testing.T) {
		dynamoRecord := events.DynamoDBEventRecord{
			EventName: "INSERT",
			Change: events.DynamoDBStreamRecord{
				NewImage: map[string]events.DynamoDBAttributeValue{
					"id":             events.NewStringAttribute("event-123"),
					"aggregate_id":   events.NewStringAttribute("product-456"),
					"aggregate_type": events.NewStringAttribute("Product"),
					"event_type":     events.NewStringAttribute("ProductCreated"),
					"data":           events.NewStringAttribute(`{"name":"Test"}`),
					"created_at":     events.NewStringAttribute(time.Now().Format(time.RFC3339Nano)),
					"version":        events.NewNumberAttribute("1"),
				},
			},
		}

		dynamoRecordJSON, err := json.Marshal(dynamoRecord)
		require.NoError(t, err)

		kinesisRecord := events.KinesisEventRecord{
			EventID: "kinesis-event-1",
			Kinesis: events.KinesisRecord{
				Data: dynamoRecordJSON,
			},
		}

		event, err := ConvertFromKinesisRecord(kinesisRecord)
		require.NoError(t, err)
		require.NotNil(t, event)
		assert.Equal(t, "event-123", event.ID)
	})
}

func TestBatchConvertFromKinesisEvent(t *testing.T) {
	t.Run("batch conversion with mixed results", func(t *testing.T) {
		validRecord := events.DynamoDBEventRecord{
			EventName: "INSERT",
			Change: events.DynamoDBStreamRecord{
				NewImage: map[string]events.DynamoDBAttributeValue{
					"id":             events.NewStringAttribute("event-1"),
					"aggregate_id":   events.NewStringAttribute("product-1"),
					"aggregate_type": events.NewStringAttribute("Product"),
					"event_type":     events.NewStringAttribute("ProductCreated"),
					"data":           events.NewStringAttribute(`{}`),
					"created_at":     events.NewStringAttribute(time.Now().Format(time.RFC3339Nano)),
					"version":        events.NewNumberAttribute("1"),
				},
			},
		}
		validJSON, _ := json.Marshal(validRecord)

		modifyRecord := events.DynamoDBEventRecord{
			EventName: "MODIFY",
		}
		modifyJSON, _ := json.Marshal(modifyRecord)

		kinesisEvent := events.KinesisEvent{
			Records: []events.KinesisEventRecord{
				{EventID: "1", Kinesis: events.KinesisRecord{Data: validJSON}},
				{EventID: "2", Kinesis: events.KinesisRecord{Data: modifyJSON}},
				{EventID: "3", Kinesis: events.KinesisRecord{Data: []byte("invalid json")}},
			},
		}

		eventList, errors := BatchConvertFromKinesisEvent(kinesisEvent)

		assert.Len(t, eventList, 1)
		assert.Len(t, errors, 1)
		assert.Equal(t, "event-1", eventList[0].ID)
	})
}
