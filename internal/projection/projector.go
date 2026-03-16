package projection

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/category"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/domain/user"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
	cartpb "github.com/example/ec-event-driven/proto/domain/cartpb"
	categorypb "github.com/example/ec-event-driven/proto/domain/categorypb"
	inventorypb "github.com/example/ec-event-driven/proto/domain/inventorypb"
	orderpb "github.com/example/ec-event-driven/proto/domain/orderpb"
	productpb "github.com/example/ec-event-driven/proto/domain/productpb"
	userpb "github.com/example/ec-event-driven/proto/domain/userpb"
	"google.golang.org/protobuf/proto"
)

type Projector struct {
	readStore store.ReadStoreInterface
}

func NewProjector(readStore store.ReadStoreInterface) *Projector {
	return &Projector{readStore: readStore}
}

// decodeProto extracts protobuf binary from event.Data (base64-encoded JSON string)
// and unmarshals into the given proto.Message.
func decodeProto(data json.RawMessage, msg proto.Message) error {
	var binaryData []byte
	if err := json.Unmarshal(data, &binaryData); err != nil {
		return err
	}
	return proto.Unmarshal(binaryData, msg)
}

func (p *Projector) HandleEvent(ctx context.Context, key, value []byte) error {
	var event store.Event
	if err := json.Unmarshal(value, &event); err != nil {
		return err
	}

	slog.InfoContext(ctx, "processing event",
		"event_type", event.EventType,
		"aggregate_type", event.AggregateType,
		"aggregate_id", event.AggregateID,
	)

	switch event.AggregateType {
	case product.AggregateType:
		return p.handleProductEvent(event)
	case cart.AggregateType:
		return p.handleCartEvent(event)
	case order.AggregateType:
		return p.handleOrderEvent(event)
	case inventory.AggregateType:
		return p.handleInventoryEvent(event)
	case user.AggregateType:
		return p.handleUserEvent(event)
	case category.AggregateType:
		return p.handleCategoryEvent(event)
	}

	return nil
}

func (p *Projector) handleProductEvent(event store.Event) error {
	switch event.EventType {
	case product.EventProductCreated:
		var e productpb.ProductCreatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, e.CreatedAt)
		_ = p.readStore.Set("products", e.ProductId, &readmodel.ProductReadModel{
			ID:          e.ProductId,
			Name:        e.Name,
			Description: e.Description,
			Price:       int(e.Price),
			Stock:       0,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		})

	case product.EventProductUpdated:
		var e productpb.ProductUpdatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		updatedAt, _ := time.Parse(time.RFC3339Nano, e.UpdatedAt)
		_, _ = p.readStore.Update("products", e.ProductId, func(current any) any {
			prod, ok := current.(*readmodel.ProductReadModel)
			if !ok {
				return current
			}
			prod.Name = e.Name
			prod.Description = e.Description
			prod.Price = int(e.Price)
			prod.UpdatedAt = updatedAt
			return prod
		})

	case product.EventProductDeleted:
		var e productpb.ProductDeletedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		_ = p.readStore.Delete("products", e.ProductId)

	case product.EventProductCategoryAssigned:
		var e product.ProductCategoryAssigned
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		if pgStore, ok := p.readStore.(*store.PostgresReadStore); ok {
			pgStore.AddProductCategory(e.ProductID, e.CategoryID)
		}

	case product.EventProductCategoryRemoved:
		var e product.ProductCategoryRemoved
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		if pgStore, ok := p.readStore.(*store.PostgresReadStore); ok {
			pgStore.RemoveProductCategory(e.ProductID, e.CategoryID)
		}

	case product.EventProductImageUpdated:
		var e product.ProductImageUpdated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		_, _ = p.readStore.Update("products", e.ProductID, func(current any) any {
			prod, ok := current.(*readmodel.ProductReadModel)
			if !ok {
				return current
			}
			prod.ImageURL = e.ImageURL
			prod.UpdatedAt = e.UpdatedAt
			return prod
		})
	}

	return nil
}

func (p *Projector) handleCartEvent(event store.Event) error {
	switch event.EventType {
	case cart.EventItemAdded:
		var e cartpb.ItemAddedToCartEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}

		productName := ""
		if prod, ok, _ := p.readStore.Get("products", e.ProductId); ok {
			if p, ok := prod.(*readmodel.ProductReadModel); ok {
				productName = p.Name
			}
		}

		_, ok, _ := p.readStore.Get("carts", e.CartId)
		if !ok {
			_ = p.readStore.Set("carts", e.CartId, &readmodel.CartReadModel{
				ID:     e.CartId,
				UserID: e.UserId,
				Items: []readmodel.CartItemReadModel{
					{ProductID: e.ProductId, Name: productName, Quantity: int(e.Quantity), Price: int(e.Price)},
				},
				Total: int(e.Price) * int(e.Quantity),
			})
		} else {
			_, _ = p.readStore.Update("carts", e.CartId, func(current any) any {
				c, ok := current.(*readmodel.CartReadModel)
				if !ok {
					return current
				}
				found := false
				for i, item := range c.Items {
					if item.ProductID == e.ProductId {
						c.Items[i].Quantity += int(e.Quantity)
						found = true
						break
					}
				}
				if !found {
					c.Items = append(c.Items, readmodel.CartItemReadModel{
						ProductID: e.ProductId,
						Name:      productName,
						Quantity:  int(e.Quantity),
						Price:     int(e.Price),
					})
				}
				c.Total = calculateCartTotal(c.Items)
				return c
			})
		}

	case cart.EventItemRemoved:
		var e cartpb.ItemRemovedFromCartEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		_, _ = p.readStore.Update("carts", e.CartId, func(current any) any {
			c, ok := current.(*readmodel.CartReadModel)
			if !ok {
				return current
			}
			newItems := make([]readmodel.CartItemReadModel, 0)
			for _, item := range c.Items {
				if item.ProductID != e.ProductId {
					newItems = append(newItems, item)
				}
			}
			c.Items = newItems
			c.Total = calculateCartTotal(c.Items)
			return c
		})

	case cart.EventCartCleared:
		var e cartpb.CartClearedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		_ = p.readStore.Set("carts", e.CartId, &readmodel.CartReadModel{
			ID:     e.CartId,
			UserID: e.UserId,
			Items:  []readmodel.CartItemReadModel{},
			Total:  0,
		})
	}

	return nil
}

func (p *Projector) handleOrderEvent(event store.Event) error {
	switch event.EventType {
	case order.EventOrderPlaced:
		var e orderpb.OrderPlacedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		items := make([]readmodel.OrderItemReadModel, len(e.Items))
		for i, item := range e.Items {
			items[i] = readmodel.OrderItemReadModel{
				ProductID: item.ProductId,
				Name:      item.Name,
				Quantity:  int(item.Quantity),
				Price:     int(item.Price),
			}
		}
		placedAt, _ := time.Parse(time.RFC3339Nano, e.PlacedAt)
		_ = p.readStore.Set("orders", e.OrderId, &readmodel.OrderReadModel{
			ID:        e.OrderId,
			UserID:    e.UserId,
			Items:     items,
			Total:     int(e.Total),
			Status:    "pending",
			CreatedAt: placedAt,
			UpdatedAt: placedAt,
		})

	case order.EventOrderPaid:
		var e orderpb.OrderPaidEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		paidAt, _ := time.Parse(time.RFC3339Nano, e.PaidAt)
		_, _ = p.readStore.Update("orders", e.OrderId, func(current any) any {
			o, ok := current.(*readmodel.OrderReadModel)
			if !ok {
				return current
			}
			o.Status = "paid"
			o.UpdatedAt = paidAt
			return o
		})

	case order.EventOrderShipped:
		var e orderpb.OrderShippedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		shippedAt, _ := time.Parse(time.RFC3339Nano, e.ShippedAt)
		_, _ = p.readStore.Update("orders", e.OrderId, func(current any) any {
			o, ok := current.(*readmodel.OrderReadModel)
			if !ok {
				return current
			}
			o.Status = "shipped"
			o.UpdatedAt = shippedAt
			return o
		})

	case order.EventOrderCancelled:
		var e orderpb.OrderCancelledEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		cancelledAt, _ := time.Parse(time.RFC3339Nano, e.CancelledAt)
		_, _ = p.readStore.Update("orders", e.OrderId, func(current any) any {
			o, ok := current.(*readmodel.OrderReadModel)
			if !ok {
				return current
			}
			o.Status = "cancelled"
			o.UpdatedAt = cancelledAt
			return o
		})
	}

	return nil
}

func (p *Projector) handleInventoryEvent(event store.Event) error {
	switch event.EventType {
	case inventory.EventStockAdded:
		var e inventorypb.StockAddedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		existing, ok, _ := p.readStore.Get("inventory", e.ProductId)
		if !ok {
			_ = p.readStore.Set("inventory", e.ProductId, &readmodel.InventoryReadModel{
				ProductID:      e.ProductId,
				TotalStock:     int(e.Quantity),
				ReservedStock:  0,
				AvailableStock: int(e.Quantity),
			})
		} else {
			inv := existing.(*readmodel.InventoryReadModel)
			inv.TotalStock += int(e.Quantity)
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			_ = p.readStore.Set("inventory", e.ProductId, inv)
		}

		_, _ = p.readStore.Update("products", e.ProductId, func(current any) any {
			prod, ok := current.(*readmodel.ProductReadModel)
			if !ok {
				return current
			}
			prod.Stock += int(e.Quantity)
			prod.UpdatedAt = time.Now()
			return prod
		})

	case inventory.EventStockReserved:
		var e inventorypb.StockReservedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		_, _ = p.readStore.Update("inventory", e.ProductId, func(current any) any {
			inv, ok := current.(*readmodel.InventoryReadModel)
			if !ok {
				return current
			}
			inv.ReservedStock += int(e.Quantity)
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			return inv
		})
		_, _ = p.readStore.Update("products", e.ProductId, func(current any) any {
			prod, ok := current.(*readmodel.ProductReadModel)
			if !ok {
				return current
			}
			prod.Stock -= int(e.Quantity)
			prod.UpdatedAt = time.Now()
			return prod
		})

	case inventory.EventStockReleased:
		var e inventorypb.StockReleasedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		_, _ = p.readStore.Update("inventory", e.ProductId, func(current any) any {
			inv, ok := current.(*readmodel.InventoryReadModel)
			if !ok {
				return current
			}
			inv.ReservedStock -= int(e.Quantity)
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			return inv
		})
		_, _ = p.readStore.Update("products", e.ProductId, func(current any) any {
			prod, ok := current.(*readmodel.ProductReadModel)
			if !ok {
				return current
			}
			prod.Stock += int(e.Quantity)
			prod.UpdatedAt = time.Now()
			return prod
		})

	case inventory.EventStockDeducted:
		var e inventorypb.StockDeductedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		_, _ = p.readStore.Update("inventory", e.ProductId, func(current any) any {
			inv, ok := current.(*readmodel.InventoryReadModel)
			if !ok {
				return current
			}
			inv.TotalStock -= int(e.Quantity)
			inv.ReservedStock -= int(e.Quantity)
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			return inv
		})
	}

	return nil
}

func (p *Projector) handleUserEvent(event store.Event) error {
	switch event.EventType {
	case user.EventUserCreated:
		var e userpb.UserCreatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, e.CreatedAt)
		_ = p.readStore.Set("users", e.UserId, &readmodel.UserReadModel{
			ID:           e.UserId,
			Email:        e.Email,
			PasswordHash: e.PasswordHash,
			Name:         e.Name,
			Role:         e.Role,
			IsActive:     true,
			CreatedAt:    createdAt,
			UpdatedAt:    createdAt,
		})

	case user.EventUserUpdated:
		var e userpb.UserUpdatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		updatedAt, _ := time.Parse(time.RFC3339Nano, e.UpdatedAt)
		_, _ = p.readStore.Update("users", e.UserId, func(current any) any {
			u, ok := current.(*readmodel.UserReadModel)
			if !ok {
				return current
			}
			u.Name = e.Name
			u.UpdatedAt = updatedAt
			return u
		})

	case user.EventUserPasswordChanged:
		var e userpb.UserPasswordChangedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		changedAt, _ := time.Parse(time.RFC3339Nano, e.ChangedAt)
		_, _ = p.readStore.Update("users", e.UserId, func(current any) any {
			u, ok := current.(*readmodel.UserReadModel)
			if !ok {
				return current
			}
			u.PasswordHash = e.PasswordHash
			u.UpdatedAt = changedAt
			return u
		})

	case user.EventUserDeactivated:
		var e userpb.UserDeactivatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		deactivatedAt, _ := time.Parse(time.RFC3339Nano, e.DeactivatedAt)
		_, _ = p.readStore.Update("users", e.UserId, func(current any) any {
			u, ok := current.(*readmodel.UserReadModel)
			if !ok {
				return current
			}
			u.IsActive = false
			u.UpdatedAt = deactivatedAt
			return u
		})

	case user.EventUserActivated:
		var e userpb.UserActivatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		activatedAt, _ := time.Parse(time.RFC3339Nano, e.ActivatedAt)
		_, _ = p.readStore.Update("users", e.UserId, func(current any) any {
			u, ok := current.(*readmodel.UserReadModel)
			if !ok {
				return current
			}
			u.IsActive = true
			u.UpdatedAt = activatedAt
			return u
		})
	}

	return nil
}

func (p *Projector) handleCategoryEvent(event store.Event) error {
	switch event.EventType {
	case category.EventCategoryCreated:
		var e categorypb.CategoryCreatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, e.CreatedAt)
		_ = p.readStore.Set("categories", e.CategoryId, &readmodel.CategoryReadModel{
			ID:          e.CategoryId,
			Name:        e.Name,
			Slug:        e.Slug,
			Description: e.Description,
			ParentID:    e.ParentId,
			SortOrder:   int(e.SortOrder),
			IsActive:    true,
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		})

	case category.EventCategoryUpdated:
		var e categorypb.CategoryUpdatedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		updatedAt, _ := time.Parse(time.RFC3339Nano, e.UpdatedAt)
		_, _ = p.readStore.Update("categories", e.CategoryId, func(current any) any {
			c, ok := current.(*readmodel.CategoryReadModel)
			if !ok {
				return current
			}
			c.Name = e.Name
			c.Slug = e.Slug
			c.Description = e.Description
			c.ParentID = e.ParentId
			c.SortOrder = int(e.SortOrder)
			c.UpdatedAt = updatedAt
			return c
		})

	case category.EventCategoryDeleted:
		var e categorypb.CategoryDeletedEvent
		if err := decodeProto(event.Data, &e); err != nil {
			return err
		}
		deletedAt, _ := time.Parse(time.RFC3339Nano, e.DeletedAt)
		_, _ = p.readStore.Update("categories", e.CategoryId, func(current any) any {
			c, ok := current.(*readmodel.CategoryReadModel)
			if !ok {
				return current
			}
			c.IsActive = false
			c.UpdatedAt = deletedAt
			return c
		})
	}

	return nil
}

func calculateCartTotal(items []readmodel.CartItemReadModel) int {
	total := 0
	for _, item := range items {
		total += item.Price * item.Quantity
	}
	return total
}
