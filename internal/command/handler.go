package command

import (
	"context"

	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/query"
)

type Handler struct {
	productSvc   *product.Service
	cartSvc      *cart.Service
	orderSvc     *order.Service
	inventorySvc *inventory.Service
	readStore    store.ReadStoreInterface
}

func NewHandler(
	productSvc *product.Service,
	cartSvc *cart.Service,
	orderSvc *order.Service,
	inventorySvc *inventory.Service,
	readStore store.ReadStoreInterface,
) *Handler {
	return &Handler{
		productSvc:   productSvc,
		cartSvc:      cartSvc,
		orderSvc:     orderSvc,
		inventorySvc: inventorySvc,
		readStore:    readStore,
	}
}

// CreateProduct creates a new product (async projection - updates via Kafka)
func (h *Handler) CreateProduct(ctx context.Context, cmd CreateProduct) (*product.Product, error) {
	// 1. Create product (emits ProductCreated event)
	p, err := h.productSvc.Create(ctx, cmd.Name, cmd.Description, cmd.Price, cmd.Stock)
	if err != nil {
		return nil, err
	}

	// 2. Initialize inventory (emits StockAdded event)
	if err := h.inventorySvc.AddStock(ctx, p.ID, cmd.Stock); err != nil {
		return nil, err
	}

	// Read Store is updated asynchronously via Kafka consumer
	return p, nil
}

// UpdateProduct updates a product
func (h *Handler) UpdateProduct(ctx context.Context, cmd UpdateProduct) error {
	return h.productSvc.Update(ctx, cmd.ProductID, cmd.Name, cmd.Description, cmd.Price)
}

// DeleteProduct deletes a product
func (h *Handler) DeleteProduct(ctx context.Context, cmd DeleteProduct) error {
	return h.productSvc.Delete(ctx, cmd.ProductID)
}

// AddToCart adds an item to cart
func (h *Handler) AddToCart(ctx context.Context, cmd AddToCart) error {
	// Get product price from read store
	p, ok := h.readStore.Get("products", cmd.ProductID)
	if !ok {
		return product.ErrProductNotFound
	}
	prod := p.(*query.ProductReadModel)

	// Emit ItemAddedToCart event
	return h.cartSvc.AddItem(ctx, cmd.UserID, cmd.ProductID, cmd.Quantity, prod.Price)
}

// RemoveFromCart removes an item from cart
func (h *Handler) RemoveFromCart(ctx context.Context, cmd RemoveFromCart) error {
	return h.cartSvc.RemoveItem(ctx, cmd.UserID, cmd.ProductID)
}

// ClearCart clears all items from cart
func (h *Handler) ClearCart(ctx context.Context, cmd ClearCart) error {
	return h.cartSvc.Clear(ctx, cmd.UserID)
}

// PlaceOrder creates an order from cart
func (h *Handler) PlaceOrder(ctx context.Context, cmd PlaceOrder) (*order.Order, error) {
	// Get cart from read store
	cartID := cart.GetCartID(cmd.UserID)
	c, ok := h.readStore.Get("carts", cartID)
	if !ok || len(c.(*query.CartReadModel).Items) == 0 {
		return nil, order.ErrEmptyOrder
	}
	cartModel := c.(*query.CartReadModel)

	// Convert cart items to order items
	var items []order.OrderItem
	for _, item := range cartModel.Items {
		items = append(items, order.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}

	// Place order (emits OrderPlaced event)
	o, err := h.orderSvc.Place(ctx, cmd.UserID, items)
	if err != nil {
		return nil, err
	}

	// Reserve inventory for each item (emits StockReserved events)
	for _, item := range items {
		if err := h.inventorySvc.Reserve(ctx, item.ProductID, o.ID, item.Quantity); err != nil {
			return nil, err
		}
	}

	// Clear cart (emits CartCleared event)
	if err := h.cartSvc.Clear(ctx, cmd.UserID); err != nil {
		return nil, err
	}

	return o, nil
}

// CancelOrder cancels an order
func (h *Handler) CancelOrder(ctx context.Context, cmd CancelOrder) error {
	// Get order from read store to release inventory
	o, ok := h.readStore.Get("orders", cmd.OrderID)
	if !ok {
		return order.ErrOrderNotFound
	}
	orderModel := o.(*query.OrderReadModel)

	// Release inventory (emits StockReleased events)
	for _, item := range orderModel.Items {
		if err := h.inventorySvc.Release(ctx, item.ProductID, cmd.OrderID, item.Quantity); err != nil {
			return err
		}
	}

	// Cancel order (emits OrderCancelled event)
	return h.orderSvc.Cancel(ctx, cmd.OrderID, cmd.Reason)
}
