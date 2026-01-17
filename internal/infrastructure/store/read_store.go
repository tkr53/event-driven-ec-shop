package store

import (
	"sync"
)

// ReadStore is an in-memory read model store
type ReadStore struct {
	mu   sync.RWMutex
	data map[string]map[string]any // collection -> id -> data
}

func NewReadStore() *ReadStore {
	return &ReadStore{
		data: make(map[string]map[string]any),
	}
}

// Set stores a read model
func (rs *ReadStore) Set(collection, id string, data any) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.data[collection] == nil {
		rs.data[collection] = make(map[string]any)
	}
	rs.data[collection][id] = data
}

// Get retrieves a read model by id
func (rs *ReadStore) Get(collection, id string) (any, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.data[collection] == nil {
		return nil, false
	}
	data, ok := rs.data[collection][id]
	return data, ok
}

// GetAll retrieves all items in a collection
func (rs *ReadStore) GetAll(collection string) []any {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.data[collection] == nil {
		return nil
	}

	var items []any
	for _, item := range rs.data[collection] {
		items = append(items, item)
	}
	return items
}

// Delete removes a read model
func (rs *ReadStore) Delete(collection, id string) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.data[collection] != nil {
		delete(rs.data[collection], id)
	}
}

// Update modifies a read model using an update function
func (rs *ReadStore) Update(collection, id string, updateFn func(current any) any) bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.data[collection] == nil {
		return false
	}
	current, ok := rs.data[collection][id]
	if !ok {
		return false
	}
	rs.data[collection][id] = updateFn(current)
	return true
}
