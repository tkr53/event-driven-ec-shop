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
	mu        sync.RWMutex
	events    map[string][]store.Event
	snapshots map[string]*store.Snapshot

	// For tracking calls in tests
	AppendCalls       []AppendCall
	AppendErr         error
	AppendCallback    func(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*store.Event, error)
	SaveSnapshotCalls []SaveSnapshotCall
	SaveSnapshotErr   error
}

// AppendCall records parameters passed to Append
type AppendCall struct {
	AggregateID   string
	AggregateType string
	EventType     string
	Data          any
}

// SaveSnapshotCall records parameters passed to SaveSnapshot
type SaveSnapshotCall struct {
	Snapshot *store.Snapshot
}

// NewMockEventStore creates a new MockEventStore
func NewMockEventStore() *MockEventStore {
	return &MockEventStore{
		events:            make(map[string][]store.Event),
		snapshots:         make(map[string]*store.Snapshot),
		AppendCalls:       make([]AppendCall, 0),
		SaveSnapshotCalls: make([]SaveSnapshotCall, 0),
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
	m.snapshots = make(map[string]*store.Snapshot)
	m.AppendCalls = make([]AppendCall, 0)
	m.SaveSnapshotCalls = make([]SaveSnapshotCall, 0)
	m.AppendErr = nil
	m.AppendCallback = nil
	m.SaveSnapshotErr = nil
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

// SaveSnapshot saves a snapshot for an aggregate
func (m *MockEventStore) SaveSnapshot(ctx context.Context, snapshot *store.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the call
	m.SaveSnapshotCalls = append(m.SaveSnapshotCalls, SaveSnapshotCall{
		Snapshot: snapshot,
	})

	if m.SaveSnapshotErr != nil {
		return m.SaveSnapshotErr
	}

	m.snapshots[snapshot.AggregateID] = snapshot
	return nil
}

// GetSnapshot retrieves the snapshot for an aggregate
func (m *MockEventStore) GetSnapshot(ctx context.Context, aggregateID string) (*store.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshots[aggregateID], nil
}

// GetEventsFromVersion returns events for an aggregate starting from a specific version
func (m *MockEventStore) GetEventsFromVersion(ctx context.Context, aggregateID string, fromVersion int) []store.Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := m.events[aggregateID]
	result := make([]store.Event, 0)
	for _, event := range events {
		if event.Version > fromVersion {
			result = append(result, event)
		}
	}
	return result
}

// SetSnapshot sets a snapshot directly for testing
func (m *MockEventStore) SetSnapshot(snapshot *store.Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots[snapshot.AggregateID] = snapshot
}
