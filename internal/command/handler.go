package command

import (
	"context"
	"fmt"
	"log"

	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
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
	p, ok, err := h.readStore.Get("products", cmd.ProductID)
	if err != nil {
		log.Printf("[Command] Error getting product %s: %v", cmd.ProductID, err)
		return product.ErrProductNotFound
	}
	if !ok {
		return product.ErrProductNotFound
	}
	prod := p.(*readmodel.ProductReadModel)

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

// PlaceOrder creates an order from cart with stock validation and compensating transactions
func (h *Handler) PlaceOrder(ctx context.Context, cmd PlaceOrder) (*order.Order, error) {
	// Get cart from read store
	cartID := cart.GetCartID(cmd.UserID)
	c, ok, err := h.readStore.Get("carts", cartID)
	if err != nil {
		log.Printf("[Command] Error getting cart %s: %v", cartID, err)
		return nil, order.ErrEmptyOrder
	}
	if !ok || len(c.(*readmodel.CartReadModel).Items) == 0 {
		return nil, order.ErrEmptyOrder
	}
	cartModel := c.(*readmodel.CartReadModel)

	// Convert cart items to order items
	var items []order.OrderItem
	for _, item := range cartModel.Items {
		items = append(items, order.OrderItem{
			ProductID: item.ProductID,
			Name:      item.Name,
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}

	// Validate stock availability for all items before placing order
	for _, item := range items {
		inv, ok, err := h.readStore.Get("inventory", item.ProductID)
		if err != nil {
			log.Printf("[Command] Error getting inventory for product %s: %v", item.ProductID, err)
			return nil, fmt.Errorf("inventory not found for product %s", item.ProductID)
		}
		if !ok {
			return nil, fmt.Errorf("inventory not found for product %s", item.ProductID)
		}
		invModel := inv.(*readmodel.InventoryReadModel)
		if invModel.AvailableStock < item.Quantity {
			return nil, fmt.Errorf("%w: product %s has only %d available, requested %d",
				inventory.ErrInsufficientStock, item.ProductID, invModel.AvailableStock, item.Quantity)
		}
	}

	// Place order (emits OrderPlaced event)
	o, err := h.orderSvc.Place(ctx, cmd.UserID, items)
	if err != nil {
		return nil, err
	}

	// Reserve inventory for each item (emits StockReserved events)
	// Track successfully reserved items for potential rollback
	var reservedItems []order.OrderItem
	for _, item := range items {
		if err := h.inventorySvc.Reserve(ctx, item.ProductID, o.ID, item.Quantity); err != nil {
			// Compensating transaction: release already reserved inventory
			for _, reserved := range reservedItems {
				if releaseErr := h.inventorySvc.Release(ctx, reserved.ProductID, o.ID, reserved.Quantity); releaseErr != nil {
					log.Printf("[PlaceOrder] Failed to release inventory for product %s: %v", reserved.ProductID, releaseErr)
				}
			}
			// Cancel the order
			if cancelErr := h.orderSvc.Cancel(ctx, o.ID, "inventory reservation failed"); cancelErr != nil {
				log.Printf("[PlaceOrder] Failed to cancel order %s: %v", o.ID, cancelErr)
			}
			return nil, fmt.Errorf("failed to reserve inventory for product %s: %w", item.ProductID, err)
		}
		reservedItems = append(reservedItems, item)
	}

	// Clear cart (emits CartCleared event)
	// This is not critical - if it fails, user can manually clear
	if err := h.cartSvc.Clear(ctx, cmd.UserID); err != nil {
		log.Printf("[PlaceOrder] Failed to clear cart for user %s: %v", cmd.UserID, err)
	}

	return o, nil
}

// CancelOrder cancels an order
func (h *Handler) CancelOrder(ctx context.Context, cmd CancelOrder) error {
	// Get order from read store to release inventory
	o, ok, err := h.readStore.Get("orders", cmd.OrderID)
	if err != nil {
		log.Printf("[Command] Error getting order %s: %v", cmd.OrderID, err)
		return order.ErrOrderNotFound
	}
	if !ok {
		return order.ErrOrderNotFound
	}
	orderModel := o.(*readmodel.OrderReadModel)

	// Release inventory (emits StockReleased events)
	for _, item := range orderModel.Items {
		if err := h.inventorySvc.Release(ctx, item.ProductID, cmd.OrderID, item.Quantity); err != nil {
			return err
		}
	}

	// Cancel order (emits OrderCancelled event)
	return h.orderSvc.Cancel(ctx, cmd.OrderID, cmd.Reason)
}
