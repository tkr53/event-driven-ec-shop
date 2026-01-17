package store

import "context"

// EventStoreInterface defines the interface for event stores
type EventStoreInterface interface {
	Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*Event, error)
	GetEvents(aggregateID string) []Event
	GetAllEvents() []Event
}
