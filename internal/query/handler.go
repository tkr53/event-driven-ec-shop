package query

import (
	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

type Handler struct {
	readStore store.ReadStoreInterface
}

func NewHandler(readStore store.ReadStoreInterface) *Handler {
	return &Handler{readStore: readStore}
}

// Products
func (h *Handler) GetProduct(id string) (*ProductReadModel, bool) {
	data, ok := h.readStore.Get("products", id)
	if !ok {
		return nil, false
	}
	return data.(*ProductReadModel), true
}

func (h *Handler) ListProducts() []*ProductReadModel {
	items := h.readStore.GetAll("products")
	products := make([]*ProductReadModel, 0, len(items))
	for _, item := range items {
		products = append(products, item.(*ProductReadModel))
	}
	return products
}

// Cart
func (h *Handler) GetCart(userID string) (*CartReadModel, bool) {
	cartID := cart.GetCartID(userID)
	data, ok := h.readStore.Get("carts", cartID)
	if !ok {
		// Return empty cart
		return &CartReadModel{
			ID:     cartID,
			UserID: userID,
			Items:  []CartItemReadModel{},
			Total:  0,
		}, true
	}
	return data.(*CartReadModel), true
}

// Orders
func (h *Handler) GetOrder(id string) (*OrderReadModel, bool) {
	data, ok := h.readStore.Get("orders", id)
	if !ok {
		return nil, false
	}
	return data.(*OrderReadModel), true
}

func (h *Handler) ListOrdersByUser(userID string) []*OrderReadModel {
	items := h.readStore.GetAll("orders")
	orders := make([]*OrderReadModel, 0)
	for _, item := range items {
		o := item.(*OrderReadModel)
		if o.UserID == userID {
			orders = append(orders, o)
		}
	}
	return orders
}

// ListAllOrders returns all orders (for admin use)
func (h *Handler) ListAllOrders() []*OrderReadModel {
	items := h.readStore.GetAll("orders")
	orders := make([]*OrderReadModel, 0, len(items))
	for _, item := range items {
		orders = append(orders, item.(*OrderReadModel))
	}
	return orders
}

// Inventory
func (h *Handler) GetInventory(productID string) (*InventoryReadModel, bool) {
	data, ok := h.readStore.Get("inventory", productID)
	if !ok {
		return nil, false
	}
	return data.(*InventoryReadModel), true
}
