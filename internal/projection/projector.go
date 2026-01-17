package projection

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/example/ec-event-driven/internal/domain/cart"
	"github.com/example/ec-event-driven/internal/domain/category"
	"github.com/example/ec-event-driven/internal/domain/inventory"
	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/domain/product"
	"github.com/example/ec-event-driven/internal/domain/user"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
)

type Projector struct {
	readStore store.ReadStoreInterface
}

func NewProjector(readStore store.ReadStoreInterface) *Projector {
	return &Projector{readStore: readStore}
}

func (p *Projector) HandleEvent(ctx context.Context, key, value []byte) error {
	var event store.Event
	if err := json.Unmarshal(value, &event); err != nil {
		return err
	}

	log.Printf("[Projector] Received event: %s (aggregate: %s)", event.EventType, event.AggregateType)

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
		var e product.ProductCreated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Set("products", e.ProductID, &readmodel.ProductReadModel{
			ID:          e.ProductID,
			Name:        e.Name,
			Description: e.Description,
			Price:       e.Price,
			Stock:       e.Stock,
			CreatedAt:   e.CreatedAt,
			UpdatedAt:   e.CreatedAt,
		})

	case product.EventProductUpdated:
		var e product.ProductUpdated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("products", e.ProductID, func(current any) any {
			prod := current.(*readmodel.ProductReadModel)
			prod.Name = e.Name
			prod.Description = e.Description
			prod.Price = e.Price
			prod.UpdatedAt = e.UpdatedAt
			return prod
		})

	case product.EventProductDeleted:
		var e product.ProductDeleted
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Delete("products", e.ProductID)

	case product.EventProductCategoryAssigned:
		var e product.ProductCategoryAssigned
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		// Use type assertion to access PostgresReadStore methods
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
		p.readStore.Update("products", e.ProductID, func(current any) any {
			prod := current.(*readmodel.ProductReadModel)
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
		var e cart.ItemAddedToCart
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}

		// Get product name
		productName := ""
		if prod, ok := p.readStore.Get("products", e.ProductID); ok {
			productName = prod.(*readmodel.ProductReadModel).Name
		}

		_, ok := p.readStore.Get("carts", e.CartID)
		if !ok {
			// Create new cart
			p.readStore.Set("carts", e.CartID, &readmodel.CartReadModel{
				ID:     e.CartID,
				UserID: e.UserID,
				Items: []readmodel.CartItemReadModel{
					{ProductID: e.ProductID, Name: productName, Quantity: e.Quantity, Price: e.Price},
				},
				Total: e.Price * e.Quantity,
			})
		} else {
			// Update existing cart
			p.readStore.Update("carts", e.CartID, func(current any) any {
				c := current.(*readmodel.CartReadModel)
				// Check if item already exists
				found := false
				for i, item := range c.Items {
					if item.ProductID == e.ProductID {
						c.Items[i].Quantity += e.Quantity
						found = true
						break
					}
				}
				if !found {
					c.Items = append(c.Items, readmodel.CartItemReadModel{
						ProductID: e.ProductID,
						Name:      productName,
						Quantity:  e.Quantity,
						Price:     e.Price,
					})
				}
				c.Total = calculateCartTotal(c.Items)
				return c
			})
		}

	case cart.EventItemRemoved:
		var e cart.ItemRemovedFromCart
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("carts", e.CartID, func(current any) any {
			c := current.(*readmodel.CartReadModel)
			newItems := make([]readmodel.CartItemReadModel, 0)
			for _, item := range c.Items {
				if item.ProductID != e.ProductID {
					newItems = append(newItems, item)
				}
			}
			c.Items = newItems
			c.Total = calculateCartTotal(c.Items)
			return c
		})

	case cart.EventCartCleared:
		var e cart.CartCleared
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Set("carts", e.CartID, &readmodel.CartReadModel{
			ID:     e.CartID,
			UserID: e.UserID,
			Items:  []readmodel.CartItemReadModel{},
			Total:  0,
		})
	}

	return nil
}

func (p *Projector) handleOrderEvent(event store.Event) error {
	switch event.EventType {
	case order.EventOrderPlaced:
		var e order.OrderPlaced
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		items := make([]readmodel.OrderItemReadModel, len(e.Items))
		for i, item := range e.Items {
			items[i] = readmodel.OrderItemReadModel{
				ProductID: item.ProductID,
				Quantity:  item.Quantity,
				Price:     item.Price,
			}
		}
		p.readStore.Set("orders", e.OrderID, &readmodel.OrderReadModel{
			ID:        e.OrderID,
			UserID:    e.UserID,
			Items:     items,
			Total:     e.Total,
			Status:    "pending",
			CreatedAt: e.PlacedAt,
			UpdatedAt: e.PlacedAt,
		})

	case order.EventOrderPaid:
		var e order.OrderPaid
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("orders", e.OrderID, func(current any) any {
			o := current.(*readmodel.OrderReadModel)
			o.Status = "paid"
			o.UpdatedAt = e.PaidAt
			return o
		})

	case order.EventOrderShipped:
		var e order.OrderShipped
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("orders", e.OrderID, func(current any) any {
			o := current.(*readmodel.OrderReadModel)
			o.Status = "shipped"
			o.UpdatedAt = e.ShippedAt
			return o
		})

	case order.EventOrderCancelled:
		var e order.OrderCancelled
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("orders", e.OrderID, func(current any) any {
			o := current.(*readmodel.OrderReadModel)
			o.Status = "cancelled"
			o.UpdatedAt = e.CancelledAt
			return o
		})
	}

	return nil
}

func (p *Projector) handleInventoryEvent(event store.Event) error {
	switch event.EventType {
	case inventory.EventStockAdded:
		var e inventory.StockAdded
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		existing, ok := p.readStore.Get("inventory", e.ProductID)
		if !ok {
			p.readStore.Set("inventory", e.ProductID, &readmodel.InventoryReadModel{
				ProductID:      e.ProductID,
				TotalStock:     e.Quantity,
				ReservedStock:  0,
				AvailableStock: e.Quantity,
			})
		} else {
			inv := existing.(*readmodel.InventoryReadModel)
			inv.TotalStock += e.Quantity
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			p.readStore.Set("inventory", e.ProductID, inv)
		}

		// Also update product stock
		p.readStore.Update("products", e.ProductID, func(current any) any {
			prod := current.(*readmodel.ProductReadModel)
			prod.Stock += e.Quantity
			prod.UpdatedAt = time.Now()
			return prod
		})

	case inventory.EventStockReserved:
		var e inventory.StockReserved
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("inventory", e.ProductID, func(current any) any {
			inv := current.(*readmodel.InventoryReadModel)
			inv.ReservedStock += e.Quantity
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			return inv
		})
		p.readStore.Update("products", e.ProductID, func(current any) any {
			prod := current.(*readmodel.ProductReadModel)
			prod.Stock -= e.Quantity
			prod.UpdatedAt = time.Now()
			return prod
		})

	case inventory.EventStockReleased:
		var e inventory.StockReleased
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("inventory", e.ProductID, func(current any) any {
			inv := current.(*readmodel.InventoryReadModel)
			inv.ReservedStock -= e.Quantity
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			return inv
		})
		p.readStore.Update("products", e.ProductID, func(current any) any {
			prod := current.(*readmodel.ProductReadModel)
			prod.Stock += e.Quantity
			prod.UpdatedAt = time.Now()
			return prod
		})

	case inventory.EventStockDeducted:
		var e inventory.StockDeducted
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("inventory", e.ProductID, func(current any) any {
			inv := current.(*readmodel.InventoryReadModel)
			inv.TotalStock -= e.Quantity
			inv.ReservedStock -= e.Quantity
			inv.AvailableStock = inv.TotalStock - inv.ReservedStock
			return inv
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

func (p *Projector) handleUserEvent(event store.Event) error {
	switch event.EventType {
	case user.EventUserCreated:
		var e user.UserCreated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Set("users", e.UserID, &readmodel.UserReadModel{
			ID:           e.UserID,
			Email:        e.Email,
			PasswordHash: e.PasswordHash,
			Name:         e.Name,
			Role:         e.Role,
			IsActive:     true,
			CreatedAt:    e.CreatedAt,
			UpdatedAt:    e.CreatedAt,
		})

	case user.EventUserUpdated:
		var e user.UserUpdated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("users", e.UserID, func(current any) any {
			u := current.(*readmodel.UserReadModel)
			u.Name = e.Name
			u.UpdatedAt = e.UpdatedAt
			return u
		})

	case user.EventUserPasswordChanged:
		var e user.UserPasswordChanged
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("users", e.UserID, func(current any) any {
			u := current.(*readmodel.UserReadModel)
			u.PasswordHash = e.PasswordHash
			u.UpdatedAt = e.ChangedAt
			return u
		})

	case user.EventUserDeactivated:
		var e user.UserDeactivated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("users", e.UserID, func(current any) any {
			u := current.(*readmodel.UserReadModel)
			u.IsActive = false
			u.UpdatedAt = e.DeactivatedAt
			return u
		})

	case user.EventUserActivated:
		var e user.UserActivated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("users", e.UserID, func(current any) any {
			u := current.(*readmodel.UserReadModel)
			u.IsActive = true
			u.UpdatedAt = e.ActivatedAt
			return u
		})
	}

	return nil
}

func (p *Projector) handleCategoryEvent(event store.Event) error {
	switch event.EventType {
	case category.EventCategoryCreated:
		var e category.CategoryCreated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Set("categories", e.CategoryID, &readmodel.CategoryReadModel{
			ID:          e.CategoryID,
			Name:        e.Name,
			Slug:        e.Slug,
			Description: e.Description,
			ParentID:    e.ParentID,
			SortOrder:   e.SortOrder,
			IsActive:    true,
			CreatedAt:   e.CreatedAt,
			UpdatedAt:   e.CreatedAt,
		})

	case category.EventCategoryUpdated:
		var e category.CategoryUpdated
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		p.readStore.Update("categories", e.CategoryID, func(current any) any {
			c := current.(*readmodel.CategoryReadModel)
			c.Name = e.Name
			c.Slug = e.Slug
			c.Description = e.Description
			c.ParentID = e.ParentID
			c.SortOrder = e.SortOrder
			c.UpdatedAt = e.UpdatedAt
			return c
		})

	case category.EventCategoryDeleted:
		var e category.CategoryDeleted
		if err := json.Unmarshal(event.Data, &e); err != nil {
			return err
		}
		// Soft delete by marking as inactive
		p.readStore.Update("categories", e.CategoryID, func(current any) any {
			c := current.(*readmodel.CategoryReadModel)
			c.IsActive = false
			c.UpdatedAt = e.DeletedAt
			return c
		})
	}

	return nil
}
