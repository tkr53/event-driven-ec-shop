package store

// ReadStoreInterface defines the interface for read model storage
type ReadStoreInterface interface {
	// Set stores a read model
	Set(collection, id string, data any)

	// Get retrieves a read model by id
	Get(collection, id string) (any, bool)

	// GetAll retrieves all items in a collection
	GetAll(collection string) []any

	// Delete removes a read model
	Delete(collection, id string)

	// Update modifies a read model using an update function
	Update(collection, id string, updateFn func(current any) any) bool
}
