package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	_ "github.com/lib/pq"
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

// EventStoreInterface defines the interface for event stores
type EventStoreInterface interface {
	Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*Event, error)
	GetEvents(aggregateID string) []Event
	GetAllEvents() []Event
}

// ConnectPostgres establishes a connection to PostgreSQL
func ConnectPostgres(connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
