package projection

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/category"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/domain/user"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/example/ec-event-driven/internal/readmodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProjector() (*Projector, *mocks.MockReadStore) {
	readStore := mocks.NewMockReadStore()
	projector := NewProjector(readStore)
	return projector, readStore
}

func makeEvent(aggregateType, eventType string, data any) []byte {
	jsonData, _ := json.Marshal(data)
	event := store.Event{
		ID:            "event-123",
		AggregateID:   "agg-123",
		AggregateType: aggregateType,
		EventType:     eventType,
		Data:          jsonData,
		Timestamp:     time.Now(),
	}
	result, _ := json.Marshal(event)
	return result
}

// ============================================
// Product Event Tests
// ============================================

func TestProjector_HandleProductCreated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	eventData := product.ProductCreated{
		ProductID:   "prod-123",
		Name:        "Test Product",
		Description: "A test product",
		Price:       1000,
		Stock:       50,
		CreatedAt:   time.Now(),
	}

	value := makeEvent(product.AggregateType, product.EventProductCreated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, ok := readStore.GetData("products", "prod-123")
	assert.True(t, ok)

	prod := data.(*readmodel.ProductReadModel)
	assert.Equal(t, "prod-123", prod.ID)
	assert.Equal(t, "Test Product", prod.Name)
	assert.Equal(t, 1000, prod.Price)
}

func TestProjector_HandleProductUpdated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	// Set up existing product
	readStore.SetData("products", "prod-123", &readmodel.ProductReadModel{
		ID:    "prod-123",
		Name:  "Old Name",
		Price: 500,
	})

	eventData := product.ProductUpdated{
		ProductID:   "prod-123",
		Name:        "New Name",
		Description: "Updated description",
		Price:       2000,
		UpdatedAt:   time.Now(),
	}

	value := makeEvent(product.AggregateType, product.EventProductUpdated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("products", "prod-123")
	prod := data.(*readmodel.ProductReadModel)
	assert.Equal(t, "New Name", prod.Name)
	assert.Equal(t, 2000, prod.Price)
}

func TestProjector_HandleProductDeleted(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	// Set up existing product
	readStore.SetData("products", "prod-123", &readmodel.ProductReadModel{ID: "prod-123"})

	eventData := product.ProductDeleted{
		ProductID: "prod-123",
		DeletedAt: time.Now(),
	}

	value := makeEvent(product.AggregateType, product.EventProductDeleted, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	_, ok := readStore.GetData("products", "prod-123")
	assert.False(t, ok)
}

// ============================================
// Order Event Tests
// ============================================

func TestProjector_HandleOrderPlaced(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	eventData := order.OrderPlaced{
		OrderID:  "order-123",
		UserID:   "user-123",
		Items:    []order.OrderItem{{ProductID: "prod-1", Quantity: 2, Price: 1000}},
		Total:    2000,
		PlacedAt: time.Now(),
	}

	value := makeEvent(order.AggregateType, order.EventOrderPlaced, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, ok := readStore.GetData("orders", "order-123")
	assert.True(t, ok)

	o := data.(*readmodel.OrderReadModel)
	assert.Equal(t, "order-123", o.ID)
	assert.Equal(t, "user-123", o.UserID)
	assert.Equal(t, "pending", o.Status)
	assert.Equal(t, 2000, o.Total)
}

func TestProjector_HandleOrderPaid(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	// Set up existing order
	readStore.SetData("orders", "order-123", &readmodel.OrderReadModel{
		ID:     "order-123",
		Status: "pending",
	})

	eventData := order.OrderPaid{
		OrderID: "order-123",
		PaidAt:  time.Now(),
	}

	value := makeEvent(order.AggregateType, order.EventOrderPaid, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("orders", "order-123")
	o := data.(*readmodel.OrderReadModel)
	assert.Equal(t, "paid", o.Status)
}

func TestProjector_HandleOrderShipped(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("orders", "order-123", &readmodel.OrderReadModel{
		ID:     "order-123",
		Status: "paid",
	})

	eventData := order.OrderShipped{
		OrderID:   "order-123",
		ShippedAt: time.Now(),
	}

	value := makeEvent(order.AggregateType, order.EventOrderShipped, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("orders", "order-123")
	o := data.(*readmodel.OrderReadModel)
	assert.Equal(t, "shipped", o.Status)
}

func TestProjector_HandleOrderCancelled(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("orders", "order-123", &readmodel.OrderReadModel{
		ID:     "order-123",
		Status: "pending",
	})

	eventData := order.OrderCancelled{
		OrderID:     "order-123",
		Reason:      "customer request",
		CancelledAt: time.Now(),
	}

	value := makeEvent(order.AggregateType, order.EventOrderCancelled, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("orders", "order-123")
	o := data.(*readmodel.OrderReadModel)
	assert.Equal(t, "cancelled", o.Status)
}

// ============================================
// Inventory Event Tests
// ============================================

func TestProjector_HandleStockAdded_NewInventory(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	eventData := inventory.StockAdded{
		ProductID: "prod-123",
		Quantity:  100,
		AddedAt:   time.Now(),
	}

	value := makeEvent(inventory.AggregateType, inventory.EventStockAdded, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, ok := readStore.GetData("inventory", "prod-123")
	assert.True(t, ok)

	inv := data.(*readmodel.InventoryReadModel)
	assert.Equal(t, "prod-123", inv.ProductID)
	assert.Equal(t, 100, inv.TotalStock)
	assert.Equal(t, 0, inv.ReservedStock)
	assert.Equal(t, 100, inv.AvailableStock)
}

func TestProjector_HandleStockAdded_ExistingInventory(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	// Set up existing inventory
	readStore.SetData("inventory", "prod-123", &readmodel.InventoryReadModel{
		ProductID:      "prod-123",
		TotalStock:     50,
		ReservedStock:  10,
		AvailableStock: 40,
	})

	eventData := inventory.StockAdded{
		ProductID: "prod-123",
		Quantity:  30,
		AddedAt:   time.Now(),
	}

	value := makeEvent(inventory.AggregateType, inventory.EventStockAdded, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("inventory", "prod-123")
	inv := data.(*readmodel.InventoryReadModel)
	assert.Equal(t, 80, inv.TotalStock)     // 50 + 30
	assert.Equal(t, 10, inv.ReservedStock)  // unchanged
	assert.Equal(t, 70, inv.AvailableStock) // 80 - 10
}

func TestProjector_HandleStockReserved(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("inventory", "prod-123", &readmodel.InventoryReadModel{
		ProductID:      "prod-123",
		TotalStock:     100,
		ReservedStock:  0,
		AvailableStock: 100,
	})
	readStore.SetData("products", "prod-123", &readmodel.ProductReadModel{
		ID:    "prod-123",
		Stock: 100,
	})

	eventData := inventory.StockReserved{
		ProductID:  "prod-123",
		OrderID:    "order-123",
		Quantity:   20,
		ReservedAt: time.Now(),
	}

	value := makeEvent(inventory.AggregateType, inventory.EventStockReserved, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("inventory", "prod-123")
	inv := data.(*readmodel.InventoryReadModel)
	assert.Equal(t, 100, inv.TotalStock)
	assert.Equal(t, 20, inv.ReservedStock)
	assert.Equal(t, 80, inv.AvailableStock)
}

func TestProjector_HandleStockReleased(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("inventory", "prod-123", &readmodel.InventoryReadModel{
		ProductID:      "prod-123",
		TotalStock:     100,
		ReservedStock:  30,
		AvailableStock: 70,
	})
	readStore.SetData("products", "prod-123", &readmodel.ProductReadModel{
		ID:    "prod-123",
		Stock: 70,
	})

	eventData := inventory.StockReleased{
		ProductID:  "prod-123",
		OrderID:    "order-123",
		Quantity:   10,
		ReleasedAt: time.Now(),
	}

	value := makeEvent(inventory.AggregateType, inventory.EventStockReleased, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("inventory", "prod-123")
	inv := data.(*readmodel.InventoryReadModel)
	assert.Equal(t, 100, inv.TotalStock)
	assert.Equal(t, 20, inv.ReservedStock)  // 30 - 10
	assert.Equal(t, 80, inv.AvailableStock) // 100 - 20
}

// ============================================
// Cart Event Tests
// ============================================

func TestProjector_HandleItemAdded_NewCart(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	// Set up product for name lookup
	readStore.SetData("products", "prod-123", &readmodel.ProductReadModel{
		ID:   "prod-123",
		Name: "Test Product",
	})

	eventData := cart.ItemAddedToCart{
		CartID:    "cart-user-123",
		UserID:    "user-123",
		ProductID: "prod-123",
		Quantity:  2,
		Price:     1000,
		AddedAt:   time.Now(),
	}

	value := makeEvent(cart.AggregateType, cart.EventItemAdded, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, ok := readStore.GetData("carts", "cart-user-123")
	assert.True(t, ok)

	c := data.(*readmodel.CartReadModel)
	assert.Equal(t, "cart-user-123", c.ID)
	assert.Equal(t, "user-123", c.UserID)
	assert.Len(t, c.Items, 1)
	assert.Equal(t, 2000, c.Total)
}

func TestProjector_HandleItemAdded_ExistingCart(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("products", "prod-456", &readmodel.ProductReadModel{ID: "prod-456", Name: "Another Product"})
	readStore.SetData("carts", "cart-user-123", &readmodel.CartReadModel{
		ID:     "cart-user-123",
		UserID: "user-123",
		Items: []readmodel.CartItemReadModel{
			{ProductID: "prod-123", Quantity: 1, Price: 500},
		},
		Total: 500,
	})

	eventData := cart.ItemAddedToCart{
		CartID:    "cart-user-123",
		UserID:    "user-123",
		ProductID: "prod-456",
		Quantity:  2,
		Price:     1000,
		AddedAt:   time.Now(),
	}

	value := makeEvent(cart.AggregateType, cart.EventItemAdded, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("carts", "cart-user-123")
	c := data.(*readmodel.CartReadModel)
	assert.Len(t, c.Items, 2)
	assert.Equal(t, 2500, c.Total) // 500 + 2000
}

func TestProjector_HandleItemRemoved(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("carts", "cart-user-123", &readmodel.CartReadModel{
		ID:     "cart-user-123",
		UserID: "user-123",
		Items: []readmodel.CartItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
			{ProductID: "prod-2", Quantity: 1, Price: 500},
		},
		Total: 2500,
	})

	eventData := cart.ItemRemovedFromCart{
		CartID:    "cart-user-123",
		UserID:    "user-123",
		ProductID: "prod-1",
		RemovedAt: time.Now(),
	}

	value := makeEvent(cart.AggregateType, cart.EventItemRemoved, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("carts", "cart-user-123")
	c := data.(*readmodel.CartReadModel)
	assert.Len(t, c.Items, 1)
	assert.Equal(t, "prod-2", c.Items[0].ProductID)
	assert.Equal(t, 500, c.Total)
}

func TestProjector_HandleCartCleared(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("carts", "cart-user-123", &readmodel.CartReadModel{
		ID:     "cart-user-123",
		UserID: "user-123",
		Items: []readmodel.CartItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
		},
		Total: 2000,
	})

	eventData := cart.CartCleared{
		CartID:    "cart-user-123",
		UserID:    "user-123",
		ClearedAt: time.Now(),
	}

	value := makeEvent(cart.AggregateType, cart.EventCartCleared, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("carts", "cart-user-123")
	c := data.(*readmodel.CartReadModel)
	assert.Empty(t, c.Items)
	assert.Equal(t, 0, c.Total)
}

// ============================================
// User Event Tests
// ============================================

func TestProjector_HandleUserCreated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	eventData := user.UserCreated{
		UserID:       "user-123",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		Name:         "Test User",
		Role:         "customer",
		CreatedAt:    time.Now(),
	}

	value := makeEvent(user.AggregateType, user.EventUserCreated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, ok := readStore.GetData("users", "user-123")
	assert.True(t, ok)

	u := data.(*readmodel.UserReadModel)
	assert.Equal(t, "user-123", u.ID)
	assert.Equal(t, "test@example.com", u.Email)
	assert.Equal(t, "customer", u.Role)
	assert.True(t, u.IsActive)
}

func TestProjector_HandleUserDeactivated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("users", "user-123", &readmodel.UserReadModel{
		ID:       "user-123",
		IsActive: true,
	})

	eventData := user.UserDeactivated{
		UserID:        "user-123",
		DeactivatedAt: time.Now(),
	}

	value := makeEvent(user.AggregateType, user.EventUserDeactivated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("users", "user-123")
	u := data.(*readmodel.UserReadModel)
	assert.False(t, u.IsActive)
}

// ============================================
// Category Event Tests
// ============================================

func TestProjector_HandleCategoryCreated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	eventData := category.CategoryCreated{
		CategoryID:  "cat-123",
		Name:        "Electronics",
		Slug:        "electronics",
		Description: "Electronic devices",
		SortOrder:   1,
		CreatedAt:   time.Now(),
	}

	value := makeEvent(category.AggregateType, category.EventCategoryCreated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, ok := readStore.GetData("categories", "cat-123")
	assert.True(t, ok)

	c := data.(*readmodel.CategoryReadModel)
	assert.Equal(t, "cat-123", c.ID)
	assert.Equal(t, "Electronics", c.Name)
	assert.Equal(t, "electronics", c.Slug)
	assert.True(t, c.IsActive)
}

func TestProjector_HandleCategoryDeleted(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("categories", "cat-123", &readmodel.CategoryReadModel{
		ID:       "cat-123",
		IsActive: true,
	})

	eventData := category.CategoryDeleted{
		CategoryID: "cat-123",
		DeletedAt:  time.Now(),
	}

	value := makeEvent(category.AggregateType, category.EventCategoryDeleted, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("categories", "cat-123")
	c := data.(*readmodel.CategoryReadModel)
	assert.False(t, c.IsActive) // Soft delete
}

// ============================================
// Calculate Cart Total Test
// ============================================

func TestCalculateCartTotal(t *testing.T) {
	tests := []struct {
		name     string
		items    []readmodel.CartItemReadModel
		expected int
	}{
		{
			name:     "empty cart",
			items:    []readmodel.CartItemReadModel{},
			expected: 0,
		},
		{
			name: "single item",
			items: []readmodel.CartItemReadModel{
				{ProductID: "prod-1", Quantity: 2, Price: 1000},
			},
			expected: 2000,
		},
		{
			name: "multiple items",
			items: []readmodel.CartItemReadModel{
				{ProductID: "prod-1", Quantity: 2, Price: 1000},
				{ProductID: "prod-2", Quantity: 3, Price: 500},
				{ProductID: "prod-3", Quantity: 1, Price: 2000},
			},
			expected: 5500, // 2000 + 1500 + 2000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateCartTotal(tt.items)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================
// Additional Product Event Tests
// ============================================

func TestProjector_HandleProductImageUpdated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("products", "prod-123", &readmodel.ProductReadModel{
		ID:       "prod-123",
		Name:     "Test Product",
		ImageURL: "",
	})

	eventData := product.ProductImageUpdated{
		ProductID: "prod-123",
		ImageURL:  "https://example.com/image.jpg",
		UpdatedAt: time.Now(),
	}

	value := makeEvent(product.AggregateType, product.EventProductImageUpdated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("products", "prod-123")
	prod := data.(*readmodel.ProductReadModel)
	assert.Equal(t, "https://example.com/image.jpg", prod.ImageURL)
}

// ============================================
// Additional User Event Tests
// ============================================

func TestProjector_HandleUserUpdated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("users", "user-123", &readmodel.UserReadModel{
		ID:   "user-123",
		Name: "Old Name",
	})

	eventData := user.UserUpdated{
		UserID:    "user-123",
		Name:      "New Name",
		UpdatedAt: time.Now(),
	}

	value := makeEvent(user.AggregateType, user.EventUserUpdated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("users", "user-123")
	u := data.(*readmodel.UserReadModel)
	assert.Equal(t, "New Name", u.Name)
}

func TestProjector_HandleUserPasswordChanged(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("users", "user-123", &readmodel.UserReadModel{
		ID:           "user-123",
		PasswordHash: "old-hash",
	})

	eventData := user.UserPasswordChanged{
		UserID:       "user-123",
		PasswordHash: "new-hash",
		ChangedAt:    time.Now(),
	}

	value := makeEvent(user.AggregateType, user.EventUserPasswordChanged, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("users", "user-123")
	u := data.(*readmodel.UserReadModel)
	assert.Equal(t, "new-hash", u.PasswordHash)
}

func TestProjector_HandleUserActivated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("users", "user-123", &readmodel.UserReadModel{
		ID:       "user-123",
		IsActive: false,
	})

	eventData := user.UserActivated{
		UserID:      "user-123",
		ActivatedAt: time.Now(),
	}

	value := makeEvent(user.AggregateType, user.EventUserActivated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("users", "user-123")
	u := data.(*readmodel.UserReadModel)
	assert.True(t, u.IsActive)
}

// ============================================
// Additional Inventory Event Tests
// ============================================

func TestProjector_HandleStockDeducted(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("inventory", "prod-123", &readmodel.InventoryReadModel{
		ProductID:      "prod-123",
		TotalStock:     100,
		ReservedStock:  20,
		AvailableStock: 80,
	})

	eventData := inventory.StockDeducted{
		ProductID:  "prod-123",
		OrderID:    "order-123",
		Quantity:   10,
		DeductedAt: time.Now(),
	}

	value := makeEvent(inventory.AggregateType, inventory.EventStockDeducted, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("inventory", "prod-123")
	inv := data.(*readmodel.InventoryReadModel)
	assert.Equal(t, 90, inv.TotalStock)     // 100 - 10
	assert.Equal(t, 10, inv.ReservedStock)  // 20 - 10
	assert.Equal(t, 80, inv.AvailableStock) // 90 - 10
}

// ============================================
// Additional Category Event Tests
// ============================================

func TestProjector_HandleCategoryUpdated(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("categories", "cat-123", &readmodel.CategoryReadModel{
		ID:   "cat-123",
		Name: "Old Name",
		Slug: "old-slug",
	})

	eventData := category.CategoryUpdated{
		CategoryID:  "cat-123",
		Name:        "New Name",
		Slug:        "new-slug",
		Description: "Updated description",
		SortOrder:   2,
		UpdatedAt:   time.Now(),
	}

	value := makeEvent(category.AggregateType, category.EventCategoryUpdated, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("categories", "cat-123")
	c := data.(*readmodel.CategoryReadModel)
	assert.Equal(t, "New Name", c.Name)
	assert.Equal(t, "new-slug", c.Slug)
	assert.Equal(t, 2, c.SortOrder)
}

// ============================================
// Additional Cart Event Tests
// ============================================

func TestProjector_HandleItemAdded_SameProductIncreasesQuantity(t *testing.T) {
	projector, readStore := newTestProjector()
	ctx := context.Background()

	readStore.SetData("products", "prod-123", &readmodel.ProductReadModel{ID: "prod-123", Name: "Test"})
	readStore.SetData("carts", "cart-user-123", &readmodel.CartReadModel{
		ID:     "cart-user-123",
		UserID: "user-123",
		Items: []readmodel.CartItemReadModel{
			{ProductID: "prod-123", Quantity: 2, Price: 1000},
		},
		Total: 2000,
	})

	eventData := cart.ItemAddedToCart{
		CartID:    "cart-user-123",
		UserID:    "user-123",
		ProductID: "prod-123",
		Quantity:  3,
		Price:     1000,
		AddedAt:   time.Now(),
	}

	value := makeEvent(cart.AggregateType, cart.EventItemAdded, eventData)

	err := projector.HandleEvent(ctx, nil, value)

	require.NoError(t, err)
	data, _ := readStore.GetData("carts", "cart-user-123")
	c := data.(*readmodel.CartReadModel)
	assert.Len(t, c.Items, 1)
	assert.Equal(t, 5, c.Items[0].Quantity) // 2 + 3
	assert.Equal(t, 5000, c.Total)          // 5 * 1000
}

// ============================================
// Error Handling Tests
// ============================================

func TestProjector_HandleEvent_InvalidJSON(t *testing.T) {
	projector, _ := newTestProjector()
	ctx := context.Background()

	invalidJSON := []byte(`{invalid json`)

	err := projector.HandleEvent(ctx, nil, invalidJSON)

	assert.Error(t, err)
}

// ============================================
// Unknown Event Type Test
// ============================================

func TestProjector_HandleUnknownEventType(t *testing.T) {
	projector, _ := newTestProjector()
	ctx := context.Background()

	value := makeEvent("UnknownAggregate", "UnknownEvent", struct{}{})

	// Should not error on unknown event types
	err := projector.HandleEvent(ctx, nil, value)

	assert.NoError(t, err)
}
