package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

const AggregateType = "Inventory"

var (
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrInvalidQuantity   = errors.New("quantity must be positive")
)

type Inventory struct {
	ProductID     string `json:"product_id"`
	TotalStock    int    `json:"total_stock"`
	ReservedStock int    `json:"reserved_stock"`
	Version       int    `json:"version"`
}

func (i *Inventory) AvailableStock() int {
	return i.TotalStock - i.ReservedStock
}

type Service struct {
	eventStore store.EventStoreInterface
}

func NewService(es store.EventStoreInterface) *Service {
	return &Service{eventStore: es}
}

// applyEvent applies a single event to the inventory state
func (i *Inventory) applyEvent(event store.Event) error {
	switch event.EventType {
	case EventStockAdded:
		var data StockAdded
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		i.ProductID = data.ProductID
		i.TotalStock += data.Quantity
	case EventStockReserved:
		var data StockReserved
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		i.ReservedStock += data.Quantity
	case EventStockReleased:
		var data StockReleased
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		i.ReservedStock -= data.Quantity
		if i.ReservedStock < 0 {
			i.ReservedStock = 0
		}
	case EventStockDeducted:
		var data StockDeducted
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		i.TotalStock -= data.Quantity
		i.ReservedStock -= data.Quantity
		if i.TotalStock < 0 {
			i.TotalStock = 0
		}
		if i.ReservedStock < 0 {
			i.ReservedStock = 0
		}
	}
	i.Version = event.Version
	return nil
}

// loadInventory loads inventory by replaying events, using snapshot if available
func (s *Service) loadInventory(ctx context.Context, productID string) (*Inventory, error) {
	inv := &Inventory{ProductID: productID}

	// Try to load from snapshot first
	snapshot, err := s.eventStore.GetSnapshot(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	var events []store.Event
	if snapshot != nil {
		// Restore state from snapshot
		if err := json.Unmarshal(snapshot.State, inv); err != nil {
			return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
		}
		// Get only events after the snapshot
		events = s.eventStore.GetEventsFromVersion(ctx, productID, snapshot.Version)
	} else {
		// No snapshot, get all events
		events = s.eventStore.GetEvents(productID)
	}

	// Apply remaining events
	for _, event := range events {
		if err := inv.applyEvent(event); err != nil {
			return nil, fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return inv, nil
}

// maybeCreateSnapshot creates a snapshot if the threshold is exceeded
func (s *Service) maybeCreateSnapshot(ctx context.Context, inv *Inventory) error {
	if inv.Version > 0 && inv.Version%store.SnapshotThreshold == 0 {
		state, err := json.Marshal(inv)
		if err != nil {
			return fmt.Errorf("failed to marshal inventory state: %w", err)
		}

		snapshot := &store.Snapshot{
			AggregateID:   inv.ProductID,
			AggregateType: AggregateType,
			Version:       inv.Version,
			State:         state,
			CreatedAt:     time.Now(),
		}

		if err := s.eventStore.SaveSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}
	}
	return nil
}

func (s *Service) AddStock(ctx context.Context, productID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	// Load current inventory state for snapshot check
	inv, err := s.loadInventory(ctx, productID)
	if err != nil {
		inv = &Inventory{ProductID: productID}
	}

	event := StockAdded{
		ProductID: productID,
		Quantity:  quantity,
		AddedAt:   time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockAdded, event)
	if err != nil {
		return err
	}

	// Update inventory for snapshot check
	inv.TotalStock += quantity
	if storedEvent != nil {
		inv.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, inv); err != nil {
		_ = err
	}

	return nil
}

func (s *Service) Reserve(ctx context.Context, productID, orderID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	// Load current inventory state for snapshot check
	inv, err := s.loadInventory(ctx, productID)
	if err != nil {
		inv = &Inventory{ProductID: productID}
	}

	event := StockReserved{
		ProductID:  productID,
		OrderID:    orderID,
		Quantity:   quantity,
		ReservedAt: time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockReserved, event)
	if err != nil {
		return err
	}

	// Update inventory for snapshot check
	inv.ReservedStock += quantity
	if storedEvent != nil {
		inv.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, inv); err != nil {
		_ = err
	}

	return nil
}

func (s *Service) Release(ctx context.Context, productID, orderID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	// Load current inventory state for snapshot check
	inv, err := s.loadInventory(ctx, productID)
	if err != nil {
		inv = &Inventory{ProductID: productID}
	}

	event := StockReleased{
		ProductID:  productID,
		OrderID:    orderID,
		Quantity:   quantity,
		ReleasedAt: time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockReleased, event)
	if err != nil {
		return err
	}

	// Update inventory for snapshot check
	inv.ReservedStock -= quantity
	if inv.ReservedStock < 0 {
		inv.ReservedStock = 0
	}
	if storedEvent != nil {
		inv.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, inv); err != nil {
		_ = err
	}

	return nil
}

func (s *Service) Deduct(ctx context.Context, productID, orderID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	// Load current inventory state for snapshot check
	inv, err := s.loadInventory(ctx, productID)
	if err != nil {
		inv = &Inventory{ProductID: productID}
	}

	event := StockDeducted{
		ProductID:  productID,
		OrderID:    orderID,
		Quantity:   quantity,
		DeductedAt: time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockDeducted, event)
	if err != nil {
		return err
	}

	// Update inventory for snapshot check
	inv.TotalStock -= quantity
	inv.ReservedStock -= quantity
	if inv.TotalStock < 0 {
		inv.TotalStock = 0
	}
	if inv.ReservedStock < 0 {
		inv.ReservedStock = 0
	}
	if storedEvent != nil {
		inv.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, inv); err != nil {
		_ = err
	}

	return nil
}
