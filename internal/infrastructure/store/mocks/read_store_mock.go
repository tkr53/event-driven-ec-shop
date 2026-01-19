package mocks

import (
	"sync"
)

// MockReadStore is a mock implementation of ReadStoreInterface for testing
type MockReadStore struct {
	mu   sync.RWMutex
	data map[string]map[string]any // collection -> id -> data

	// For tracking calls in tests
	SetCalls    []SetCall
	GetCalls    []GetCall
	DeleteCalls []DeleteCall
	UpdateCalls []UpdateCall
}

// SetCall records parameters passed to Set
type SetCall struct {
	Collection string
	ID         string
	Data       any
}

// GetCall records parameters passed to Get
type GetCall struct {
	Collection string
	ID         string
}

// DeleteCall records parameters passed to Delete
type DeleteCall struct {
	Collection string
	ID         string
}

// UpdateCall records parameters passed to Update
type UpdateCall struct {
	Collection string
	ID         string
}

// NewMockReadStore creates a new MockReadStore
func NewMockReadStore() *MockReadStore {
	return &MockReadStore{
		data:        make(map[string]map[string]any),
		SetCalls:    make([]SetCall, 0),
		GetCalls:    make([]GetCall, 0),
		DeleteCalls: make([]DeleteCall, 0),
		UpdateCalls: make([]UpdateCall, 0),
	}
}

// Set stores a read model
func (m *MockReadStore) Set(collection, id string, data any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetCalls = append(m.SetCalls, SetCall{
		Collection: collection,
		ID:         id,
		Data:       data,
	})

	if m.data[collection] == nil {
		m.data[collection] = make(map[string]any)
	}
	m.data[collection][id] = data
}

// Get retrieves a read model by id
func (m *MockReadStore) Get(collection, id string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetCalls = append(m.GetCalls, GetCall{
		Collection: collection,
		ID:         id,
	})

	if m.data[collection] == nil {
		return nil, false
	}
	data, ok := m.data[collection][id]
	return data, ok
}

// GetAll retrieves all items in a collection
func (m *MockReadStore) GetAll(collection string) []any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.data[collection] == nil {
		return []any{}
	}

	items := make([]any, 0, len(m.data[collection]))
	for _, item := range m.data[collection] {
		items = append(items, item)
	}
	return items
}

// Delete removes a read model
func (m *MockReadStore) Delete(collection, id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteCalls = append(m.DeleteCalls, DeleteCall{
		Collection: collection,
		ID:         id,
	})

	if m.data[collection] != nil {
		delete(m.data[collection], id)
	}
}

// Update modifies a read model using an update function
func (m *MockReadStore) Update(collection, id string, updateFn func(current any) any) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.UpdateCalls = append(m.UpdateCalls, UpdateCall{
		Collection: collection,
		ID:         id,
	})

	if m.data[collection] == nil {
		return false
	}
	current, ok := m.data[collection][id]
	if !ok {
		return false
	}
	m.data[collection][id] = updateFn(current)
	return true
}

// Reset clears all data and recorded calls
func (m *MockReadStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]map[string]any)
	m.SetCalls = make([]SetCall, 0)
	m.GetCalls = make([]GetCall, 0)
	m.DeleteCalls = make([]DeleteCall, 0)
	m.UpdateCalls = make([]UpdateCall, 0)
}

// SetData sets data directly for testing
func (m *MockReadStore) SetData(collection, id string, data any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data[collection] == nil {
		m.data[collection] = make(map[string]any)
	}
	m.data[collection][id] = data
}

// GetData gets data directly for testing (without recording the call)
func (m *MockReadStore) GetData(collection, id string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data[collection] == nil {
		return nil, false
	}
	data, ok := m.data[collection][id]
	return data, ok
}
