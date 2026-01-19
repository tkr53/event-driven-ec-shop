package cart

import (
	"context"
	"testing"

	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCartService() (*Service, *mocks.MockEventStore) {
	eventStore := mocks.NewMockEventStore()
	service := NewService(eventStore)
	return service, eventStore
}

// ============================================
// GetCartID Tests
// ============================================

func TestGetCartID(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		expectedID string
	}{
		{"normal user ID", "user-123", "cart-user-123"},
		{"UUID user ID", "550e8400-e29b-41d4-a716-446655440000", "cart-550e8400-e29b-41d4-a716-446655440000"},
		{"empty user ID", "", "cart-"},
		{"user with special chars", "user@example.com", "cart-user@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCartID(tt.userID)
			assert.Equal(t, tt.expectedID, result)
		})
	}
}

// ============================================
// Add Item Tests
// ============================================

func TestService_AddItem_Success(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	err := service.AddItem(ctx, "user-123", "prod-456", 2, 1000)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventItemAdded, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, AggregateType, eventStore.AppendCalls[0].AggregateType)

	// Verify cart ID format
	assert.Equal(t, "cart-user-123", eventStore.AppendCalls[0].AggregateID)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(ItemAddedToCart)
	assert.Equal(t, "cart-user-123", data.CartID)
	assert.Equal(t, "user-123", data.UserID)
	assert.Equal(t, "prod-456", data.ProductID)
	assert.Equal(t, 2, data.Quantity)
	assert.Equal(t, 1000, data.Price)
}

func TestService_AddItem_SingleQuantity(t *testing.T) {
	service, _ := newTestCartService()
	ctx := context.Background()

	err := service.AddItem(ctx, "user-123", "prod-456", 1, 500)

	require.NoError(t, err)
}

func TestService_AddItem_EmptyProductID(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	err := service.AddItem(ctx, "user-123", "", 2, 1000)

	assert.ErrorIs(t, err, ErrInvalidProduct)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_AddItem_ZeroQuantity(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	err := service.AddItem(ctx, "user-123", "prod-456", 0, 1000)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_AddItem_NegativeQuantity(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	err := service.AddItem(ctx, "user-123", "prod-456", -1, 1000)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_AddItem_ZeroPrice(t *testing.T) {
	service, _ := newTestCartService()
	ctx := context.Background()

	// Zero price is allowed (free items)
	err := service.AddItem(ctx, "user-123", "prod-456", 1, 0)

	require.NoError(t, err)
}

// ============================================
// Remove Item Tests
// ============================================

func TestService_RemoveItem_Success(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	err := service.RemoveItem(ctx, "user-123", "prod-456")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventItemRemoved, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(ItemRemovedFromCart)
	assert.Equal(t, "cart-user-123", data.CartID)
	assert.Equal(t, "user-123", data.UserID)
	assert.Equal(t, "prod-456", data.ProductID)
}

func TestService_RemoveItem_EmptyProductID(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	err := service.RemoveItem(ctx, "user-123", "")

	assert.ErrorIs(t, err, ErrInvalidProduct)
	assert.Empty(t, eventStore.AppendCalls)
}

// ============================================
// Clear Cart Tests
// ============================================

func TestService_Clear_Success(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	err := service.Clear(ctx, "user-123")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventCartCleared, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(CartCleared)
	assert.Equal(t, "cart-user-123", data.CartID)
	assert.Equal(t, "user-123", data.UserID)
}

func TestService_Clear_EmptyCart(t *testing.T) {
	service, _ := newTestCartService()
	ctx := context.Background()

	// Clearing an empty cart should still succeed
	err := service.Clear(ctx, "user-123")

	require.NoError(t, err)
}

// ============================================
// Cart Operations Sequence Test
// ============================================

func TestCartOperations_Sequence(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	userID := "user-123"

	// 1. Add first item
	err := service.AddItem(ctx, userID, "prod-1", 2, 1000)
	require.NoError(t, err)

	// 2. Add second item
	err = service.AddItem(ctx, userID, "prod-2", 1, 2000)
	require.NoError(t, err)

	// 3. Remove first item
	err = service.RemoveItem(ctx, userID, "prod-1")
	require.NoError(t, err)

	// 4. Clear cart
	err = service.Clear(ctx, userID)
	require.NoError(t, err)

	// Verify all events were recorded
	assert.Len(t, eventStore.AppendCalls, 4)
	assert.Equal(t, EventItemAdded, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, EventItemAdded, eventStore.AppendCalls[1].EventType)
	assert.Equal(t, EventItemRemoved, eventStore.AppendCalls[2].EventType)
	assert.Equal(t, EventCartCleared, eventStore.AppendCalls[3].EventType)
}

func TestAddMultipleItemsSameProduct(t *testing.T) {
	service, eventStore := newTestCartService()
	ctx := context.Background()

	userID := "user-123"

	// Add same product twice
	err := service.AddItem(ctx, userID, "prod-1", 2, 1000)
	require.NoError(t, err)

	err = service.AddItem(ctx, userID, "prod-1", 3, 1000)
	require.NoError(t, err)

	// Both events should be recorded (projection handles merging)
	assert.Len(t, eventStore.AppendCalls, 2)
}
