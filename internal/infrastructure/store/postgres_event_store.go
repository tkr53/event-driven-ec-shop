package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/kafka"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// PostgresEventStore stores events in PostgreSQL
type PostgresEventStore struct {
	db       *sql.DB
	producer *kafka.Producer
}

func NewPostgresEventStore(db *sql.DB, producer *kafka.Producer) *PostgresEventStore {
	return &PostgresEventStore{
		db:       db,
		producer: producer,
	}
}

// Append stores an event in PostgreSQL and publishes to Kafka
func (es *PostgresEventStore) Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*Event, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	eventID := uuid.New().String()
	timestamp := time.Now()

	// Use atomic INSERT ... SELECT to avoid race condition on version
	// This calculates and inserts the version in a single operation
	var version int
	err = es.db.QueryRowContext(ctx,
		`INSERT INTO events (id, aggregate_id, aggregate_type, event_type, data, version, created_at)
		 SELECT $1, $2, $3, $4, $5, COALESCE(MAX(version), 0) + 1, $6
		 FROM events
		 WHERE aggregate_id = $2
		 RETURNING version`,
		eventID,
		aggregateID,
		aggregateType,
		eventType,
		jsonData,
		timestamp,
	).Scan(&version)
	if err != nil {
		return nil, err
	}

	event := Event{
		ID:            eventID,
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          jsonData,
		Timestamp:     timestamp,
		Version:       version,
	}

	// Publish to Kafka
	if es.producer != nil {
		if err := es.producer.Publish(ctx, aggregateID, event); err != nil {
			return nil, err
		}
	}

	return &event, nil
}

// GetEvents returns all events for an aggregate from PostgreSQL
func (es *PostgresEventStore) GetEvents(aggregateID string) []Event {
	ctx := context.Background()
	rows, err := es.db.QueryContext(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, data, version, created_at
		 FROM events
		 WHERE aggregate_id = $1
		 ORDER BY version ASC`,
		aggregateID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.AggregateType, &e.EventType, &e.Data, &e.Version, &e.Timestamp); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events
}

// GetAllEvents returns all events from PostgreSQL
func (es *PostgresEventStore) GetAllEvents() []Event {
	ctx := context.Background()
	rows, err := es.db.QueryContext(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, data, version, created_at
		 FROM events
		 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.AggregateType, &e.EventType, &e.Data, &e.Version, &e.Timestamp); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events
}

// GetEventsByType returns all events of a specific aggregate type
func (es *PostgresEventStore) GetEventsByType(aggregateType string) []Event {
	ctx := context.Background()
	rows, err := es.db.QueryContext(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, data, version, created_at
		 FROM events
		 WHERE aggregate_type = $1
		 ORDER BY created_at ASC`,
		aggregateType,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.AggregateType, &e.EventType, &e.Data, &e.Version, &e.Timestamp); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events
}

// GetEventsAfter returns events created after a specific time (for replay)
func (es *PostgresEventStore) GetEventsAfter(after time.Time) []Event {
	ctx := context.Background()
	rows, err := es.db.QueryContext(ctx,
		`SELECT id, aggregate_id, aggregate_type, event_type, data, version, created_at
		 FROM events
		 WHERE created_at > $1
		 ORDER BY created_at ASC`,
		after,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.AggregateID, &e.AggregateType, &e.EventType, &e.Data, &e.Version, &e.Timestamp); err != nil {
			continue
		}
		events = append(events, e)
	}
	return events
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
