package command

import (
	"context"
	"log/slog"

	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
)

// HybridConfig controls which aggregates use the actor-based handler.
// Allows gradual migration from legacy service-based handler to actors.
type HybridConfig struct {
	UseActorForProduct   bool
	UseActorForCart       bool
	UseActorForOrder     bool
	UseActorForInventory bool
}

// HybridHandler delegates commands to either the legacy handler or the actor handler
// based on configuration, enabling gradual migration.
type HybridHandler struct {
	legacy *Handler
	actor  *ActorHandler
	config HybridConfig
}

func NewHybridHandler(legacy *Handler, actor *ActorHandler, config HybridConfig) *HybridHandler {
	if config.UseActorForProduct {
		slog.Info("using actor handler for Product aggregate")
	}
	if config.UseActorForCart {
		slog.Info("using actor handler for Cart aggregate")
	}
	if config.UseActorForOrder {
		slog.Info("using actor handler for Order aggregate")
	}
	if config.UseActorForInventory {
		slog.Info("using actor handler for Inventory aggregate")
	}
	return &HybridHandler{legacy: legacy, actor: actor, config: config}
}

// Product commands

func (h *HybridHandler) CreateProduct(ctx context.Context, cmd CreateProduct) (*product.Product, error) {
	if h.config.UseActorForProduct && h.actor != nil {
		return h.actor.CreateProduct(ctx, cmd)
	}
	return h.legacy.CreateProduct(ctx, cmd)
}

func (h *HybridHandler) UpdateProduct(ctx context.Context, cmd UpdateProduct) error {
	if h.config.UseActorForProduct && h.actor != nil {
		return h.actor.UpdateProduct(ctx, cmd)
	}
	return h.legacy.UpdateProduct(ctx, cmd)
}

func (h *HybridHandler) DeleteProduct(ctx context.Context, cmd DeleteProduct) error {
	if h.config.UseActorForProduct && h.actor != nil {
		return h.actor.DeleteProduct(ctx, cmd)
	}
	return h.legacy.DeleteProduct(ctx, cmd)
}

// Cart commands (delegated to legacy for now)

func (h *HybridHandler) AddToCart(ctx context.Context, cmd AddToCart) error {
	return h.legacy.AddToCart(ctx, cmd)
}

func (h *HybridHandler) RemoveFromCart(ctx context.Context, cmd RemoveFromCart) error {
	return h.legacy.RemoveFromCart(ctx, cmd)
}

func (h *HybridHandler) ClearCart(ctx context.Context, cmd ClearCart) error {
	return h.legacy.ClearCart(ctx, cmd)
}

// Order commands (delegated to legacy for now)

func (h *HybridHandler) PlaceOrder(ctx context.Context, cmd PlaceOrder) (*order.Order, error) {
	return h.legacy.PlaceOrder(ctx, cmd)
}

func (h *HybridHandler) PayOrder(ctx context.Context, cmd PayOrder) error {
	return h.legacy.PayOrder(ctx, cmd)
}

func (h *HybridHandler) ShipOrder(ctx context.Context, cmd ShipOrder) error {
	return h.legacy.ShipOrder(ctx, cmd)
}

func (h *HybridHandler) CancelOrder(ctx context.Context, cmd CancelOrder) error {
	return h.legacy.CancelOrder(ctx, cmd)
}
