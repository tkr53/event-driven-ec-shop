package mocks

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/google/uuid"
)

// MockEventStore is a mock implementation of EventStoreInterface for testing
type MockEventStore struct {
	mu     sync.RWMutex
	events map[string][]store.Event

	// For tracking calls in tests
	AppendCalls    []AppendCall
	AppendErr      error
	AppendCallback func(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*store.Event, error)
}

// AppendCall records parameters passed to Append
type AppendCall struct {
	AggregateID   string
	AggregateType string
	EventType     string
	Data          any
}

// NewMockEventStore creates a new MockEventStore
func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		events:      make(map[string][]store.Event),
		AppendCalls: make([]AppendCall, 0),
	}
}

// Append stores an event in memory
func (m *MockEventStore) Append(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*store.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.AppendCalls = append(m.AppendCalls, AppendCall{
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          data,
	})

	// Use callback if provided
	if m.AppendCallback != nil {
		return m.AppendCallback(ctx, aggregateID, aggregateType, eventType, data)
	}

	// Return error if set
	if m.AppendErr != nil {
		return nil, m.AppendErr
	}

	// Create event
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	version := len(m.events[aggregateID]) + 1
	event := store.Event{
		ID:            uuid.New().String(),
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          jsonData,
		Timestamp:     time.Now(),
		Version:       version,
	}

	m.events[aggregateID] = append(m.events[aggregateID], event)
	return &event, nil
}

// GetEvents returns events for an aggregate
func (m *MockEventStore) GetEvents(aggregateID string) []store.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.events[aggregateID]
}

// GetAllEvents returns all events
func (m *MockEventStore) GetAllEvents() []store.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []store.Event
	for _, events := range m.events {
		all = append(all, events...)
	}
	return all
}

// Reset clears all events and recorded calls
func (m *MockEventStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = make(map[string][]store.Event)
	m.AppendCalls = make([]AppendCall, 0)
	m.AppendErr = nil
	m.AppendCallback = nil
}

// SetEvents sets events directly for testing
func (m *MockEventStore) SetEvents(aggregateID string, events []store.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events[aggregateID] = events
}

// AddEvent adds a single event for testing
func (m *MockEventStore) AddEvent(aggregateID, aggregateType, eventType string, data any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	version := len(m.events[aggregateID]) + 1
	event := store.Event{
		ID:            uuid.New().String(),
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          jsonData,
		Timestamp:     time.Now(),
		Version:       version,
	}

	m.events[aggregateID] = append(m.events[aggregateID], event)
	return nil
}
