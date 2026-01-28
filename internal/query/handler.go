package query

import (
	"log"

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
	data, ok, err := h.readStore.Get("products", id)
	if err != nil {
		log.Printf("[Query] Error getting product %s: %v", id, err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return data.(*ProductReadModel), true
}

func (h *Handler) ListProducts() []*ProductReadModel {
	items, err := h.readStore.GetAll("products")
	if err != nil {
		log.Printf("[Query] Error listing products: %v", err)
		return nil
	}
	products := make([]*ProductReadModel, 0, len(items))
	for _, item := range items {
		products = append(products, item.(*ProductReadModel))
	}
	return products
}

// Cart
func (h *Handler) GetCart(userID string) (*CartReadModel, bool) {
	cartID := cart.GetCartID(userID)
	data, ok, err := h.readStore.Get("carts", cartID)
	if err != nil {
		log.Printf("[Query] Error getting cart %s: %v", cartID, err)
		return nil, false
	}
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
	data, ok, err := h.readStore.Get("orders", id)
	if err != nil {
		log.Printf("[Query] Error getting order %s: %v", id, err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return data.(*OrderReadModel), true
}

func (h *Handler) ListOrdersByUser(userID string) []*OrderReadModel {
	items, err := h.readStore.GetAll("orders")
	if err != nil {
		log.Printf("[Query] Error listing orders: %v", err)
		return nil
	}
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
	items, err := h.readStore.GetAll("orders")
	if err != nil {
		log.Printf("[Query] Error listing all orders: %v", err)
		return nil
	}
	orders := make([]*OrderReadModel, 0, len(items))
	for _, item := range items {
		orders = append(orders, item.(*OrderReadModel))
	}
	return orders
}

// Inventory
func (h *Handler) GetInventory(productID string) (*InventoryReadModel, bool) {
	data, ok, err := h.readStore.Get("inventory", productID)
	if err != nil {
		log.Printf("[Query] Error getting inventory %s: %v", productID, err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return data.(*InventoryReadModel), true
}
