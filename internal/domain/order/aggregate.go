package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/google/uuid"
)

const AggregateType = "Order"

type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusShipped   Status = "shipped"
	StatusCancelled Status = "cancelled"
)

var (
	ErrOrderNotFound    = errors.New("order not found")
	ErrEmptyOrder       = errors.New("order must have at least one item")
	ErrInvalidStatus    = errors.New("invalid order status transition")
	ErrOrderAlreadyPaid = errors.New("order is already paid")
	ErrOrderNotPaid     = errors.New("order must be paid before shipping")
	ErrOrderShipped     = errors.New("cannot cancel shipped order")
	ErrOrderCancelled   = errors.New("order is already cancelled")
)

type Order struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Total     int         `json:"total"`
	Status    Status      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	Version   int         `json:"version"` // Current event version
}

type Service struct {
	eventStore store.EventStoreInterface
}

func NewService(es store.EventStoreInterface) *Service {
	return &Service{eventStore: es}
}

// rebuildStatus reconstructs the current order status from events
func (s *Service) rebuildStatus(events []store.Event) Status {
	status := StatusPending
	for _, event := range events {
		switch event.EventType {
		case EventOrderPlaced:
			status = StatusPending
		case EventOrderPaid:
			status = StatusPaid
		case EventOrderShipped:
			status = StatusShipped
		case EventOrderCancelled:
			status = StatusCancelled
		}
	}
	return status
}

// applyEvent applies a single event to the order state
func (o *Order) applyEvent(event store.Event) error {
	switch event.EventType {
	case EventOrderPlaced:
		var data OrderPlaced
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		o.ID = data.OrderID
		o.UserID = data.UserID
		o.Items = data.Items
		o.Total = data.Total
		o.Status = StatusPending
		o.CreatedAt = data.PlacedAt
		o.UpdatedAt = data.PlacedAt
	case EventOrderPaid:
		var data OrderPaid
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		o.Status = StatusPaid
		o.UpdatedAt = data.PaidAt
	case EventOrderShipped:
		var data OrderShipped
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		o.Status = StatusShipped
		o.UpdatedAt = data.ShippedAt
	case EventOrderCancelled:
		var data OrderCancelled
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		o.Status = StatusCancelled
		o.UpdatedAt = data.CancelledAt
	}
	o.Version = event.Version
	return nil
}

// loadOrder loads an order by replaying events, using snapshot if available
func (s *Service) loadOrder(ctx context.Context, orderID string) (*Order, error) {
	order := &Order{}

	// Try to load from snapshot first
	snapshot, err := s.eventStore.GetSnapshot(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	var events []store.Event
	if snapshot != nil {
		// Restore state from snapshot
		if err := json.Unmarshal(snapshot.State, order); err != nil {
			return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
		}
		// Get only events after the snapshot
		events = s.eventStore.GetEventsFromVersion(ctx, orderID, snapshot.Version)
	} else {
		// No snapshot, get all events
		events = s.eventStore.GetEvents(orderID)
	}

	if snapshot == nil && len(events) == 0 {
		return nil, ErrOrderNotFound
	}

	// Apply remaining events
	for _, event := range events {
		if err := order.applyEvent(event); err != nil {
			return nil, fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return order, nil
}

// maybeCreateSnapshot creates a snapshot if the threshold is exceeded
func (s *Service) maybeCreateSnapshot(ctx context.Context, order *Order) error {
	if order.Version > 0 && order.Version%store.SnapshotThreshold == 0 {
		state, err := json.Marshal(order)
		if err != nil {
			return fmt.Errorf("failed to marshal order state: %w", err)
		}

		snapshot := &store.Snapshot{
			AggregateID:   order.ID,
			AggregateType: AggregateType,
			Version:       order.Version,
			State:         state,
			CreatedAt:     time.Now(),
		}

		if err := s.eventStore.SaveSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}
	}
	return nil
}

func (s *Service) Place(ctx context.Context, userID string, items []OrderItem) (*Order, error) {
	if len(items) == 0 {
		return nil, ErrEmptyOrder
	}

	orderID := uuid.New().String()
	now := time.Now()

	var total int
	for _, item := range items {
		total += item.Price * item.Quantity
	}

	event := OrderPlaced{
		OrderID:  orderID,
		UserID:   userID,
		Items:    items,
		Total:    total,
		PlacedAt: now,
	}

	storedEvent, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderPlaced, event)
	if err != nil {
		return nil, err
	}

	version := 0
	if storedEvent != nil {
		version = storedEvent.Version
	}

	order := &Order{
		ID:        orderID,
		UserID:    userID,
		Items:     items,
		Total:     total,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   version,
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, order); err != nil {
		log.Printf("[Order] Failed to create snapshot for order %s: %v", order.ID, err)
	}

	return order, nil
}

func (s *Service) Pay(ctx context.Context, orderID string) error {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}

	// Validate current status
	switch order.Status {
	case StatusPending:
		// Valid transition
	case StatusPaid:
		return ErrOrderAlreadyPaid
	case StatusShipped:
		return ErrOrderAlreadyPaid
	case StatusCancelled:
		return ErrOrderCancelled
	default:
		return fmt.Errorf("%w: cannot pay order in %s status", ErrInvalidStatus, order.Status)
	}

	event := OrderPaid{
		OrderID: orderID,
		PaidAt:  time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderPaid, event)
	if err != nil {
		return err
	}

	// Update order for snapshot check
	order.Status = StatusPaid
	if storedEvent != nil {
		order.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, order); err != nil {
		_ = err
	}

	return nil
}

func (s *Service) Ship(ctx context.Context, orderID string) error {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}

	// Validate current status - can only ship paid orders
	switch order.Status {
	case StatusPaid:
		// Valid transition
	case StatusPending:
		return ErrOrderNotPaid
	case StatusShipped:
		return fmt.Errorf("%w: order is already shipped", ErrInvalidStatus)
	case StatusCancelled:
		return ErrOrderCancelled
	default:
		return fmt.Errorf("%w: cannot ship order in %s status", ErrInvalidStatus, order.Status)
	}

	event := OrderShipped{
		OrderID:   orderID,
		ShippedAt: time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderShipped, event)
	if err != nil {
		return err
	}

	// Update order for snapshot check
	order.Status = StatusShipped
	if storedEvent != nil {
		order.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, order); err != nil {
		_ = err
	}

	return nil
}

func (s *Service) Cancel(ctx context.Context, orderID, reason string) error {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}

	// Validate current status - cannot cancel shipped orders
	switch order.Status {
	case StatusPending, StatusPaid:
		// Valid - can cancel pending or paid orders (with refund if paid)
	case StatusShipped:
		return ErrOrderShipped
	case StatusCancelled:
		return ErrOrderCancelled
	default:
		return fmt.Errorf("%w: cannot cancel order in %s status", ErrInvalidStatus, order.Status)
	}

	event := OrderCancelled{
		OrderID:     orderID,
		Reason:      reason,
		CancelledAt: time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderCancelled, event)
	if err != nil {
		return err
	}

	// Update order for snapshot check
	order.Status = StatusCancelled
	if storedEvent != nil {
		order.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, order); err != nil {
		_ = err
	}

	return nil
}
