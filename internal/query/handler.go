package query

import (
	"context"
	"log/slog"

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
func (h *Handler) GetProduct(ctx context.Context, id string) (*ProductReadModel, bool) {
	data, ok, err := h.readStore.Get("products", id)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get product", "product_id", id, "error", err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return data.(*ProductReadModel), true
}

func (h *Handler) ListProducts(ctx context.Context) []*ProductReadModel {
	items, err := h.readStore.GetAll("products")
	if err != nil {
		slog.ErrorContext(ctx, "failed to list products", "error", err)
		return nil
	}
	products := make([]*ProductReadModel, 0, len(items))
	for _, item := range items {
		products = append(products, item.(*ProductReadModel))
	}
	return products
}

// Cart
func (h *Handler) GetCart(ctx context.Context, userID string) (*CartReadModel, bool) {
	cartID := cart.GetCartID(userID)
	data, ok, err := h.readStore.Get("carts", cartID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get cart", "cart_id", cartID, "error", err)
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
func (h *Handler) GetOrder(ctx context.Context, id string) (*OrderReadModel, bool) {
	data, ok, err := h.readStore.Get("orders", id)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get order", "order_id", id, "error", err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return data.(*OrderReadModel), true
}

func (h *Handler) ListOrdersByUser(ctx context.Context, userID string) []*OrderReadModel {
	items, err := h.readStore.GetAll("orders")
	if err != nil {
		slog.ErrorContext(ctx, "failed to list orders", "error", err)
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
func (h *Handler) ListAllOrders(ctx context.Context) []*OrderReadModel {
	items, err := h.readStore.GetAll("orders")
	if err != nil {
		slog.ErrorContext(ctx, "failed to list all orders", "error", err)
		return nil
	}
	orders := make([]*OrderReadModel, 0, len(items))
	for _, item := range items {
		orders = append(orders, item.(*OrderReadModel))
	}
	return orders
}

// Inventory
func (h *Handler) GetInventory(ctx context.Context, productID string) (*InventoryReadModel, bool) {
	data, ok, err := h.readStore.Get("inventory", productID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get inventory", "product_id", productID, "error", err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return data.(*InventoryReadModel), true
}
