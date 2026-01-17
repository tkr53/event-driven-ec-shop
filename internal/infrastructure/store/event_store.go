package store

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/kafka"
	"github.com/google/uuid"
)

// Event represents a domain event
type Event struct {
	ID            string          `json:"id"`
	AggregateID   string          `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	EventType     string          `json:"event_type"`
	Data          json.RawMessage `json:"data"`
	Timestamp     time.Time       `json:"timestamp"`
	Version       int             `json:"version"`
}

// MarshalJSON returns the JSON encoding of the event
func (e Event) MarshalJSON() ([]byte, error) {
	type Alias Event
	return json.Marshal(&struct{ Alias }{Alias: Alias(e)})
}

// EventStore stores and publishes domain events
type EventStore struct {
	mu       sync.RWMutex
	events   map[string][]Event // aggregateID -> events
	producer *kafka.Producer
}

func NewEventStore(producer *kafka.Producer) *EventStore {
	return &EventStore{
		events:   make(map[string][]Event),
		producer: producer,
	}
}

// Append stores an event and publishes it to Kafka
func (es *EventStore) Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*Event, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	es.mu.Lock()
	version := len(es.events[aggregateID]) + 1
	event := Event{
		ID:            uuid.New().String(),
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          jsonData,
		Timestamp:     time.Now(),
		Version:       version,
	}
	es.events[aggregateID] = append(es.events[aggregateID], event)
	es.mu.Unlock()

	// Publish to Kafka
	if es.producer != nil {
		if err := es.producer.Publish(ctx, aggregateID, event); err != nil {
			return nil, err
		}
	}

	return &event, nil
}

// GetEvents returns all events for an aggregate
func (es *EventStore) GetEvents(aggregateID string) []Event {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.events[aggregateID]
}

// GetAllEvents returns all events
func (es *EventStore) GetAllEvents() []Event {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var all []Event
	for _, events := range es.events {
		all = append(all, events...)
	}
	return all
}
