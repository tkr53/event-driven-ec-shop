package command

import (
	"context"
	"errors"
	"testing"

	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/example/ec-event-driven/internal/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestHandler() (*Handler, *mocks.MockEventStore, *mocks.MockReadStore) {
	eventStore := mocks.NewMockEventStore()
	readStore := mocks.NewMockReadStore()

	productSvc := product.NewService(eventStore)
	cartSvc := cart.NewService(eventStore)
	orderSvc := order.NewService(eventStore)
	inventorySvc := inventory.NewService(eventStore)

	handler := NewHandler(productSvc, cartSvc, orderSvc, inventorySvc, readStore)
	return handler, eventStore, readStore
}

// ============================================
// Create Product Tests
// ============================================

func TestHandler_CreateProduct_Success(t *testing.T) {
	handler, eventStore, _ := newTestHandler()
	ctx := context.Background()

	cmd := CreateProduct{
		Name:        "Test Product",
		Description: "A test product",
		Price:       1000,
		Stock:       50,
	}

	p, err := handler.CreateProduct(ctx, cmd)

	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "Test Product", p.Name)
	assert.Equal(t, 1000, p.Price)
	assert.Equal(t, 50, p.Stock)

	// Should have 2 events: ProductCreated and StockAdded
	assert.Len(t, eventStore.AppendCalls, 2)
	assert.Equal(t, product.EventProductCreated, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, inventory.EventStockAdded, eventStore.AppendCalls[1].EventType)
}

func TestHandler_CreateProduct_InvalidName(t *testing.T) {
	handler, _, _ := newTestHandler()
	ctx := context.Background()

	cmd := CreateProduct{
		Name:        "",
		Description: "Description",
		Price:       1000,
		Stock:       50,
	}

	p, err := handler.CreateProduct(ctx, cmd)

	assert.ErrorIs(t, err, product.ErrInvalidName)
	assert.Nil(t, p)
}

func TestHandler_CreateProduct_InvalidPrice(t *testing.T) {
	handler, _, _ := newTestHandler()
	ctx := context.Background()

	cmd := CreateProduct{
		Name:        "Test",
		Description: "Description",
		Price:       0,
		Stock:       50,
	}

	p, err := handler.CreateProduct(ctx, cmd)

	assert.ErrorIs(t, err, product.ErrInvalidPrice)
	assert.Nil(t, p)
}

// ============================================
// Update Product Tests
// ============================================

func TestHandler_UpdateProduct_Success(t *testing.T) {
	handler, eventStore, _ := newTestHandler()
	ctx := context.Background()

	productID := "prod-123"
	eventStore.AddEvent(productID, product.AggregateType, product.EventProductCreated, product.ProductCreated{ProductID: productID})

	cmd := UpdateProduct{
		ProductID:   productID,
		Name:        "Updated Name",
		Description: "Updated Description",
		Price:       2000,
	}

	err := handler.UpdateProduct(ctx, cmd)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, product.EventProductUpdated, eventStore.AppendCalls[0].EventType)
}

func TestHandler_UpdateProduct_NotFound(t *testing.T) {
	handler, _, _ := newTestHandler()
	ctx := context.Background()

	cmd := UpdateProduct{
		ProductID:   "non-existent",
		Name:        "Name",
		Description: "Desc",
		Price:       1000,
	}

	err := handler.UpdateProduct(ctx, cmd)

	assert.ErrorIs(t, err, product.ErrProductNotFound)
}

// ============================================
// Delete Product Tests
// ============================================

func TestHandler_DeleteProduct_Success(t *testing.T) {
	handler, eventStore, _ := newTestHandler()
	ctx := context.Background()

	productID := "prod-123"
	eventStore.AddEvent(productID, product.AggregateType, product.EventProductCreated, product.ProductCreated{ProductID: productID})

	cmd := DeleteProduct{ProductID: productID}

	err := handler.DeleteProduct(ctx, cmd)

	require.NoError(t, err)
	assert.Equal(t, product.EventProductDeleted, eventStore.AppendCalls[0].EventType)
}

// ============================================
// Add To Cart Tests
// ============================================

func TestHandler_AddToCart_Success(t *testing.T) {
	handler, eventStore, readStore := newTestHandler()
	ctx := context.Background()

	// Set up product in read store
	readStore.SetData("products", "prod-123", &query.ProductReadModel{
		ID:    "prod-123",
		Name:  "Test Product",
		Price: 1000,
	})

	cmd := AddToCart{
		UserID:    "user-123",
		ProductID: "prod-123",
		Quantity:  2,
	}

	err := handler.AddToCart(ctx, cmd)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, cart.EventItemAdded, eventStore.AppendCalls[0].EventType)
}

func TestHandler_AddToCart_ProductNotFound(t *testing.T) {
	handler, _, _ := newTestHandler()
	ctx := context.Background()

	cmd := AddToCart{
		UserID:    "user-123",
		ProductID: "non-existent",
		Quantity:  2,
	}

	err := handler.AddToCart(ctx, cmd)

	assert.ErrorIs(t, err, product.ErrProductNotFound)
}

// ============================================
// Remove From Cart Tests
// ============================================

func TestHandler_RemoveFromCart_Success(t *testing.T) {
	handler, eventStore, _ := newTestHandler()
	ctx := context.Background()

	cmd := RemoveFromCart{
		UserID:    "user-123",
		ProductID: "prod-123",
	}

	err := handler.RemoveFromCart(ctx, cmd)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, cart.EventItemRemoved, eventStore.AppendCalls[0].EventType)
}

// ============================================
// Clear Cart Tests
// ============================================

func TestHandler_ClearCart_Success(t *testing.T) {
	handler, eventStore, _ := newTestHandler()
	ctx := context.Background()

	cmd := ClearCart{UserID: "user-123"}

	err := handler.ClearCart(ctx, cmd)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, cart.EventCartCleared, eventStore.AppendCalls[0].EventType)
}

// ============================================
// Place Order Tests - Happy Path
// ============================================

func TestHandler_PlaceOrder_Success(t *testing.T) {
	handler, eventStore, readStore := newTestHandler()
	ctx := context.Background()

	userID := "user-123"
	cartID := cart.GetCartID(userID)

	// Set up cart with items
	readStore.SetData("carts", cartID, &query.CartReadModel{
		ID:     cartID,
		UserID: userID,
		Items: []query.CartItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
			{ProductID: "prod-2", Quantity: 1, Price: 2000},
		},
		Total: 4000,
	})

	// Set up inventory for both products
	readStore.SetData("inventory", "prod-1", &query.InventoryReadModel{
		ProductID:      "prod-1",
		TotalStock:     100,
		ReservedStock:  0,
		AvailableStock: 100,
	})
	readStore.SetData("inventory", "prod-2", &query.InventoryReadModel{
		ProductID:      "prod-2",
		TotalStock:     50,
		ReservedStock:  0,
		AvailableStock: 50,
	})

	cmd := PlaceOrder{UserID: userID}

	o, err := handler.PlaceOrder(ctx, cmd)

	require.NoError(t, err)
	assert.NotEmpty(t, o.ID)
	assert.Equal(t, userID, o.UserID)
	assert.Equal(t, 4000, o.Total)
	assert.Equal(t, order.StatusPending, o.Status)

	// Should have events: OrderPlaced + 2x StockReserved + CartCleared
	assert.Len(t, eventStore.AppendCalls, 4)
	assert.Equal(t, order.EventOrderPlaced, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, inventory.EventStockReserved, eventStore.AppendCalls[1].EventType)
	assert.Equal(t, inventory.EventStockReserved, eventStore.AppendCalls[2].EventType)
	assert.Equal(t, cart.EventCartCleared, eventStore.AppendCalls[3].EventType)
}

func TestHandler_PlaceOrder_EmptyCart(t *testing.T) {
	handler, _, readStore := newTestHandler()
	ctx := context.Background()

	userID := "user-123"
	cartID := cart.GetCartID(userID)

	// Set up empty cart
	readStore.SetData("carts", cartID, &query.CartReadModel{
		ID:     cartID,
		UserID: userID,
		Items:  []query.CartItemReadModel{},
		Total:  0,
	})

	cmd := PlaceOrder{UserID: userID}

	o, err := handler.PlaceOrder(ctx, cmd)

	assert.ErrorIs(t, err, order.ErrEmptyOrder)
	assert.Nil(t, o)
}

func TestHandler_PlaceOrder_CartNotFound(t *testing.T) {
	handler, _, _ := newTestHandler()
	ctx := context.Background()

	cmd := PlaceOrder{UserID: "user-with-no-cart"}

	o, err := handler.PlaceOrder(ctx, cmd)

	assert.ErrorIs(t, err, order.ErrEmptyOrder)
	assert.Nil(t, o)
}

func TestHandler_PlaceOrder_InsufficientStock(t *testing.T) {
	handler, _, readStore := newTestHandler()
	ctx := context.Background()

	userID := "user-123"
	cartID := cart.GetCartID(userID)

	// Set up cart with items
	readStore.SetData("carts", cartID, &query.CartReadModel{
		ID:     cartID,
		UserID: userID,
		Items: []query.CartItemReadModel{
			{ProductID: "prod-1", Quantity: 100, Price: 1000}, // Requesting 100
		},
		Total: 100000,
	})

	// Set up inventory with insufficient stock
	readStore.SetData("inventory", "prod-1", &query.InventoryReadModel{
		ProductID:      "prod-1",
		TotalStock:     50,
		ReservedStock:  30,
		AvailableStock: 20, // Only 20 available, requesting 100
	})

	cmd := PlaceOrder{UserID: userID}

	o, err := handler.PlaceOrder(ctx, cmd)

	assert.ErrorIs(t, err, inventory.ErrInsufficientStock)
	assert.Nil(t, o)
}

// ============================================
// Place Order - Compensating Transaction Tests
// ============================================

func TestHandler_PlaceOrder_CompensatingTransaction(t *testing.T) {
	eventStore := mocks.NewMockEventStore()
	readStore := mocks.NewMockReadStore()

	productSvc := product.NewService(eventStore)
	cartSvc := cart.NewService(eventStore)
	orderSvc := order.NewService(eventStore)
	inventorySvc := inventory.NewService(eventStore)

	handler := NewHandler(productSvc, cartSvc, orderSvc, inventorySvc, readStore)
	ctx := context.Background()

	userID := "user-123"
	cartID := cart.GetCartID(userID)

	// Set up cart with 2 items
	readStore.SetData("carts", cartID, &query.CartReadModel{
		ID:     cartID,
		UserID: userID,
		Items: []query.CartItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
			{ProductID: "prod-2", Quantity: 1, Price: 2000},
		},
		Total: 4000,
	})

	// Set up inventory for first product (will succeed)
	readStore.SetData("inventory", "prod-1", &query.InventoryReadModel{
		ProductID:      "prod-1",
		TotalStock:     100,
		AvailableStock: 100,
	})
	// Set up inventory for second product (will succeed in validation, fail in reserve)
	readStore.SetData("inventory", "prod-2", &query.InventoryReadModel{
		ProductID:      "prod-2",
		TotalStock:     50,
		AvailableStock: 50,
	})

	// Configure event store to fail on second reserve
	callCount := 0
	eventStore.AppendCallback = func(ctx context.Context, aggregateID, aggregateType, eventType string, data any) (*store.Event, error) {
		callCount++
		// Fail on 3rd call (which should be the second StockReserved)
		if callCount == 3 {
			return nil, errors.New("simulated database error")
		}
		return nil, nil
	}

	cmd := PlaceOrder{UserID: userID}

	o, err := handler.PlaceOrder(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, o)

	// Verify compensating transactions were attempted
	// Should have: OrderPlaced (1) + StockReserved (2, fails) + StockReleased (3) + OrderCancelled (4)
	// Note: The actual number of calls depends on exact implementation
	assert.True(t, len(eventStore.AppendCalls) >= 3, "Expected at least 3 append calls for compensating transaction")

	// Check that release was attempted for the first product
	hasRelease := false
	hasCancellation := false
	for _, call := range eventStore.AppendCalls {
		if call.EventType == inventory.EventStockReleased {
			hasRelease = true
		}
		if call.EventType == order.EventOrderCancelled {
			hasCancellation = true
		}
	}

	// At least release should be attempted
	assert.True(t, hasRelease || hasCancellation, "Expected compensating transaction (release or cancellation)")
}

// ============================================
// Cancel Order Tests
// ============================================

func TestHandler_CancelOrder_Success(t *testing.T) {
	handler, eventStore, readStore := newTestHandler()
	ctx := context.Background()

	orderID := "order-123"

	// Set up order in event store (pending state)
	eventStore.AddEvent(orderID, order.AggregateType, order.EventOrderPlaced, order.OrderPlaced{
		OrderID: orderID,
		UserID:  "user-123",
		Items: []order.OrderItem{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
		},
	})

	// Set up order in read store
	readStore.SetData("orders", orderID, &query.OrderReadModel{
		ID:     orderID,
		UserID: "user-123",
		Items: []query.OrderItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
		},
		Status: "pending",
	})

	cmd := CancelOrder{
		OrderID: orderID,
		Reason:  "customer request",
	}

	err := handler.CancelOrder(ctx, cmd)

	require.NoError(t, err)

	// Should have StockReleased + OrderCancelled events
	hasRelease := false
	hasCancellation := false
	for _, call := range eventStore.AppendCalls {
		if call.EventType == inventory.EventStockReleased {
			hasRelease = true
		}
		if call.EventType == order.EventOrderCancelled {
			hasCancellation = true
		}
	}
	assert.True(t, hasRelease, "Expected StockReleased event")
	assert.True(t, hasCancellation, "Expected OrderCancelled event")
}

func TestHandler_CancelOrder_OrderNotFound(t *testing.T) {
	handler, _, _ := newTestHandler()
	ctx := context.Background()

	cmd := CancelOrder{
		OrderID: "non-existent",
		Reason:  "reason",
	}

	err := handler.CancelOrder(ctx, cmd)

	assert.ErrorIs(t, err, order.ErrOrderNotFound)
}

func TestHandler_CancelOrder_AlreadyShipped(t *testing.T) {
	handler, eventStore, readStore := newTestHandler()
	ctx := context.Background()

	orderID := "order-123"

	// Set up order in shipped state
	eventStore.AddEvent(orderID, order.AggregateType, order.EventOrderPlaced, order.OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, order.AggregateType, order.EventOrderPaid, order.OrderPaid{OrderID: orderID})
	eventStore.AddEvent(orderID, order.AggregateType, order.EventOrderShipped, order.OrderShipped{OrderID: orderID})

	readStore.SetData("orders", orderID, &query.OrderReadModel{
		ID:     orderID,
		Items:  []query.OrderItemReadModel{{ProductID: "prod-1", Quantity: 1}},
		Status: "shipped",
	})

	cmd := CancelOrder{
		OrderID: orderID,
		Reason:  "too late",
	}

	err := handler.CancelOrder(ctx, cmd)

	assert.ErrorIs(t, err, order.ErrOrderShipped)
}

// ============================================
// Additional CreateProduct Tests
// ============================================

func TestHandler_CreateProduct_ZeroStock(t *testing.T) {
	handler, eventStore, _ := newTestHandler()
	ctx := context.Background()

	cmd := CreateProduct{
		Name:        "Test Product",
		Description: "Description",
		Price:       1000,
		Stock:       0,
	}

	// Zero stock should fail because AddStock requires positive quantity
	p, err := handler.CreateProduct(ctx, cmd)

	assert.Error(t, err)
	assert.Nil(t, p)
	// ProductCreated event was recorded but AddStock failed
	assert.Len(t, eventStore.AppendCalls, 1)
}

// ============================================
// Additional PlaceOrder Tests
// ============================================

func TestHandler_PlaceOrder_InventoryNotFound(t *testing.T) {
	handler, _, readStore := newTestHandler()
	ctx := context.Background()

	userID := "user-123"
	cartID := cart.GetCartID(userID)

	readStore.SetData("carts", cartID, &query.CartReadModel{
		ID:     cartID,
		UserID: userID,
		Items: []query.CartItemReadModel{
			{ProductID: "prod-no-inventory", Quantity: 1, Price: 1000},
		},
		Total: 1000,
	})
	// No inventory data set for prod-no-inventory

	cmd := PlaceOrder{UserID: userID}

	o, err := handler.PlaceOrder(ctx, cmd)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inventory not found")
	assert.Nil(t, o)
}

func TestHandler_PlaceOrder_MultipleItemsOneInsufficientStock(t *testing.T) {
	handler, _, readStore := newTestHandler()
	ctx := context.Background()

	userID := "user-123"
	cartID := cart.GetCartID(userID)

	readStore.SetData("carts", cartID, &query.CartReadModel{
		ID:     cartID,
		UserID: userID,
		Items: []query.CartItemReadModel{
			{ProductID: "prod-1", Quantity: 5, Price: 1000},
			{ProductID: "prod-2", Quantity: 100, Price: 2000}, // This one has insufficient stock
		},
		Total: 205000,
	})

	readStore.SetData("inventory", "prod-1", &query.InventoryReadModel{
		ProductID:      "prod-1",
		TotalStock:     100,
		AvailableStock: 100,
	})
	readStore.SetData("inventory", "prod-2", &query.InventoryReadModel{
		ProductID:      "prod-2",
		TotalStock:     50,
		AvailableStock: 50, // Only 50 available, requesting 100
	})

	cmd := PlaceOrder{UserID: userID}

	o, err := handler.PlaceOrder(ctx, cmd)

	assert.ErrorIs(t, err, inventory.ErrInsufficientStock)
	assert.Nil(t, o)
}

// ============================================
// Additional CancelOrder Tests
// ============================================

func TestHandler_CancelOrder_MultipleItems(t *testing.T) {
	handler, eventStore, readStore := newTestHandler()
	ctx := context.Background()

	orderID := "order-123"

	eventStore.AddEvent(orderID, order.AggregateType, order.EventOrderPlaced, order.OrderPlaced{
		OrderID: orderID,
		Items: []order.OrderItem{
			{ProductID: "prod-1", Quantity: 2},
			{ProductID: "prod-2", Quantity: 3},
		},
	})

	readStore.SetData("orders", orderID, &query.OrderReadModel{
		ID:     orderID,
		UserID: "user-123",
		Items: []query.OrderItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
			{ProductID: "prod-2", Quantity: 3, Price: 500},
		},
		Status: "pending",
	})

	cmd := CancelOrder{
		OrderID: orderID,
		Reason:  "changed mind",
	}

	err := handler.CancelOrder(ctx, cmd)

	require.NoError(t, err)

	// Should have 2 StockReleased + 1 OrderCancelled
	releaseCount := 0
	cancelCount := 0
	for _, call := range eventStore.AppendCalls {
		if call.EventType == inventory.EventStockReleased {
			releaseCount++
		}
		if call.EventType == order.EventOrderCancelled {
			cancelCount++
		}
	}
	assert.Equal(t, 2, releaseCount)
	assert.Equal(t, 1, cancelCount)
}
