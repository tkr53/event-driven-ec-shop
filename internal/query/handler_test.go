package query

import (
	"testing"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/stretchr/testify/assert"
)

func newTestQueryHandler() (*Handler, *mocks.MockReadStore) {
	readStore := mocks.NewMockReadStore()
	handler := NewHandler(readStore)
	return handler, readStore
}

// ============================================
// Product Query Tests
// ============================================

func TestHandler_GetProduct_Found(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	expectedProduct := &ProductReadModel{
		ID:          "prod-123",
		Name:        "Test Product",
		Description: "A great product",
		Price:       1000,
		Stock:       50,
		CreatedAt:   time.Now(),
	}
	readStore.SetData("products", "prod-123", expectedProduct)

	product, found := handler.GetProduct("prod-123")

	assert.True(t, found)
	assert.Equal(t, expectedProduct.ID, product.ID)
	assert.Equal(t, expectedProduct.Name, product.Name)
	assert.Equal(t, expectedProduct.Price, product.Price)
}

func TestHandler_GetProduct_NotFound(t *testing.T) {
	handler, _ := newTestQueryHandler()

	product, found := handler.GetProduct("non-existent")

	assert.False(t, found)
	assert.Nil(t, product)
}

func TestHandler_ListProducts_WithProducts(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	readStore.SetData("products", "prod-1", &ProductReadModel{ID: "prod-1", Name: "Product 1"})
	readStore.SetData("products", "prod-2", &ProductReadModel{ID: "prod-2", Name: "Product 2"})
	readStore.SetData("products", "prod-3", &ProductReadModel{ID: "prod-3", Name: "Product 3"})

	products := handler.ListProducts()

	assert.Len(t, products, 3)
}

func TestHandler_ListProducts_Empty(t *testing.T) {
	handler, _ := newTestQueryHandler()

	products := handler.ListProducts()

	assert.Empty(t, products)
}

// ============================================
// Cart Query Tests
// ============================================

func TestHandler_GetCart_Found(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	expectedCart := &CartReadModel{
		ID:     "cart-user-123",
		UserID: "user-123",
		Items: []CartItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
		},
		Total: 2000,
	}
	readStore.SetData("carts", "cart-user-123", expectedCart)

	cart, found := handler.GetCart("user-123")

	assert.True(t, found)
	assert.Equal(t, expectedCart.ID, cart.ID)
	assert.Equal(t, expectedCart.UserID, cart.UserID)
	assert.Len(t, cart.Items, 1)
	assert.Equal(t, 2000, cart.Total)
}

func TestHandler_GetCart_NotFound_ReturnsEmptyCart(t *testing.T) {
	handler, _ := newTestQueryHandler()

	cart, found := handler.GetCart("user-with-no-cart")

	// GetCart returns an empty cart when not found
	assert.True(t, found)
	assert.Equal(t, "cart-user-with-no-cart", cart.ID)
	assert.Equal(t, "user-with-no-cart", cart.UserID)
	assert.Empty(t, cart.Items)
	assert.Equal(t, 0, cart.Total)
}

// ============================================
// Order Query Tests
// ============================================

func TestHandler_GetOrder_Found(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	expectedOrder := &OrderReadModel{
		ID:     "order-123",
		UserID: "user-123",
		Items: []OrderItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
		},
		Total:     2000,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	readStore.SetData("orders", "order-123", expectedOrder)

	order, found := handler.GetOrder("order-123")

	assert.True(t, found)
	assert.Equal(t, expectedOrder.ID, order.ID)
	assert.Equal(t, expectedOrder.Status, order.Status)
}

func TestHandler_GetOrder_NotFound(t *testing.T) {
	handler, _ := newTestQueryHandler()

	order, found := handler.GetOrder("non-existent")

	assert.False(t, found)
	assert.Nil(t, order)
}

func TestHandler_ListOrdersByUser_WithOrders(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	readStore.SetData("orders", "order-1", &OrderReadModel{ID: "order-1", UserID: "user-123"})
	readStore.SetData("orders", "order-2", &OrderReadModel{ID: "order-2", UserID: "user-123"})
	readStore.SetData("orders", "order-3", &OrderReadModel{ID: "order-3", UserID: "user-456"})

	orders := handler.ListOrdersByUser("user-123")

	assert.Len(t, orders, 2)
	for _, order := range orders {
		assert.Equal(t, "user-123", order.UserID)
	}
}

func TestHandler_ListOrdersByUser_NoOrders(t *testing.T) {
	handler, _ := newTestQueryHandler()

	orders := handler.ListOrdersByUser("user-with-no-orders")

	assert.Empty(t, orders)
}

func TestHandler_ListAllOrders_WithOrders(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	readStore.SetData("orders", "order-1", &OrderReadModel{ID: "order-1", UserID: "user-123"})
	readStore.SetData("orders", "order-2", &OrderReadModel{ID: "order-2", UserID: "user-456"})

	orders := handler.ListAllOrders()

	assert.Len(t, orders, 2)
}

func TestHandler_ListAllOrders_Empty(t *testing.T) {
	handler, _ := newTestQueryHandler()

	orders := handler.ListAllOrders()

	assert.Empty(t, orders)
}

// ============================================
// Inventory Query Tests
// ============================================

func TestHandler_GetInventory_Found(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	expectedInventory := &InventoryReadModel{
		ProductID:      "prod-123",
		TotalStock:     100,
		ReservedStock:  20,
		AvailableStock: 80,
	}
	readStore.SetData("inventory", "prod-123", expectedInventory)

	inventory, found := handler.GetInventory("prod-123")

	assert.True(t, found)
	assert.Equal(t, expectedInventory.ProductID, inventory.ProductID)
	assert.Equal(t, expectedInventory.TotalStock, inventory.TotalStock)
	assert.Equal(t, expectedInventory.AvailableStock, inventory.AvailableStock)
}

func TestHandler_GetInventory_NotFound(t *testing.T) {
	handler, _ := newTestQueryHandler()

	inventory, found := handler.GetInventory("non-existent")

	assert.False(t, found)
	assert.Nil(t, inventory)
}

// ============================================
// Cart Total Calculation Test
// ============================================

func TestCartTotal(t *testing.T) {
	handler, readStore := newTestQueryHandler()

	// The cart ID should match the format "cart-{userID}"
	cart := &CartReadModel{
		ID:     "cart-user-123",
		UserID: "user-123",
		Items: []CartItemReadModel{
			{ProductID: "prod-1", Quantity: 2, Price: 1000},
			{ProductID: "prod-2", Quantity: 3, Price: 500},
			{ProductID: "prod-3", Quantity: 1, Price: 2000},
		},
		Total: 5500, // 2*1000 + 3*500 + 1*2000
	}
	readStore.SetData("carts", "cart-user-123", cart)

	result, found := handler.GetCart("user-123")

	assert.True(t, found)
	assert.Equal(t, 5500, result.Total)
}
