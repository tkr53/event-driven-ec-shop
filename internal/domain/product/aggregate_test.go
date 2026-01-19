package product

import (
	"context"
	"testing"

	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProductService() (*Service, *mocks.MockEventStore) {
	eventStore := mocks.NewMockEventStore()
	service := NewService(eventStore)
	return service, eventStore
}

// ============================================
// Create Product Tests
// ============================================

func TestService_Create_ValidProduct(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	product, err := service.Create(ctx, "Test Product", "A great product", 1000, 50)

	require.NoError(t, err)
	assert.NotEmpty(t, product.ID)
	assert.Equal(t, "Test Product", product.Name)
	assert.Equal(t, "A great product", product.Description)
	assert.Equal(t, 1000, product.Price)
	assert.Equal(t, 50, product.Stock)
	assert.False(t, product.IsDeleted)

	// Verify event was stored
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventProductCreated, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, AggregateType, eventStore.AppendCalls[0].AggregateType)
}

func TestService_Create_EmptyDescription(t *testing.T) {
	service, _ := newTestProductService()
	ctx := context.Background()

	product, err := service.Create(ctx, "Test Product", "", 1000, 50)

	require.NoError(t, err)
	assert.Equal(t, "", product.Description)
}

func TestService_Create_ZeroStock(t *testing.T) {
	service, _ := newTestProductService()
	ctx := context.Background()

	product, err := service.Create(ctx, "Test Product", "Description", 1000, 0)

	require.NoError(t, err)
	assert.Equal(t, 0, product.Stock)
}

func TestService_Create_EmptyName(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	product, err := service.Create(ctx, "", "Description", 1000, 50)

	assert.ErrorIs(t, err, ErrInvalidName)
	assert.Nil(t, product)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Create_ZeroPrice(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	product, err := service.Create(ctx, "Test Product", "Description", 0, 50)

	assert.ErrorIs(t, err, ErrInvalidPrice)
	assert.Nil(t, product)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Create_NegativePrice(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	product, err := service.Create(ctx, "Test Product", "Description", -100, 50)

	assert.ErrorIs(t, err, ErrInvalidPrice)
	assert.Nil(t, product)
	assert.Empty(t, eventStore.AppendCalls)
}

// ============================================
// Update Product Tests
// ============================================

func TestService_Update_Success(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	productID := "prod-123"
	eventStore.AddEvent(productID, AggregateType, EventProductCreated, ProductCreated{ProductID: productID})

	err := service.Update(ctx, productID, "Updated Name", "Updated Description", 2000)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventProductUpdated, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(ProductUpdated)
	assert.Equal(t, "Updated Name", data.Name)
	assert.Equal(t, "Updated Description", data.Description)
	assert.Equal(t, 2000, data.Price)
}

func TestService_Update_NotFound(t *testing.T) {
	service, _ := newTestProductService()
	ctx := context.Background()

	err := service.Update(ctx, "non-existent", "Name", "Desc", 1000)

	assert.ErrorIs(t, err, ErrProductNotFound)
}

func TestService_Update_EmptyName(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	productID := "prod-123"
	eventStore.AddEvent(productID, AggregateType, EventProductCreated, ProductCreated{ProductID: productID})

	err := service.Update(ctx, productID, "", "Description", 1000)

	assert.ErrorIs(t, err, ErrInvalidName)
}

func TestService_Update_ZeroPrice(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	productID := "prod-123"
	eventStore.AddEvent(productID, AggregateType, EventProductCreated, ProductCreated{ProductID: productID})

	err := service.Update(ctx, productID, "Name", "Description", 0)

	assert.ErrorIs(t, err, ErrInvalidPrice)
}

func TestService_Update_NegativePrice(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	productID := "prod-123"
	eventStore.AddEvent(productID, AggregateType, EventProductCreated, ProductCreated{ProductID: productID})

	err := service.Update(ctx, productID, "Name", "Description", -500)

	assert.ErrorIs(t, err, ErrInvalidPrice)
}

// ============================================
// Delete Product Tests
// ============================================

func TestService_Delete_Success(t *testing.T) {
	service, eventStore := newTestProductService()
	ctx := context.Background()

	productID := "prod-123"
	eventStore.AddEvent(productID, AggregateType, EventProductCreated, ProductCreated{ProductID: productID})

	err := service.Delete(ctx, productID)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventProductDeleted, eventStore.AppendCalls[0].EventType)
}

func TestService_Delete_NotFound(t *testing.T) {
	service, _ := newTestProductService()
	ctx := context.Background()

	err := service.Delete(ctx, "non-existent")

	assert.ErrorIs(t, err, ErrProductNotFound)
}
