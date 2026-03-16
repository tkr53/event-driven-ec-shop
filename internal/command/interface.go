package command

import (
	"context"

	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
)

// CommandHandler defines the interface for all command operations.
// Both Handler (legacy) and HybridHandler implement this interface.
type CommandHandler interface {
	CreateProduct(ctx context.Context, cmd CreateProduct) (*product.Product, error)
	UpdateProduct(ctx context.Context, cmd UpdateProduct) error
	DeleteProduct(ctx context.Context, cmd DeleteProduct) error

	AddToCart(ctx context.Context, cmd AddToCart) error
	RemoveFromCart(ctx context.Context, cmd RemoveFromCart) error
	ClearCart(ctx context.Context, cmd ClearCart) error

	PlaceOrder(ctx context.Context, cmd PlaceOrder) (*order.Order, error)
	PayOrder(ctx context.Context, cmd PayOrder) error
	ShipOrder(ctx context.Context, cmd ShipOrder) error
	CancelOrder(ctx context.Context, cmd CancelOrder) error
}
