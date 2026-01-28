package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/example/ec-event-driven/internal/domain/aggregate"
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

// validTransitions defines allowed state transitions
var validTransitions = map[Status][]Status{
	StatusPending:   {StatusPaid, StatusCancelled},
	StatusPaid:      {StatusShipped, StatusCancelled},
	StatusShipped:   {}, // terminal state
	StatusCancelled: {}, // terminal state
}

// CanTransitionTo checks if the order can transition to the target status
func (o *Order) CanTransitionTo(target Status) bool {
	allowed, exists := validTransitions[o.Status]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

// transitionError returns an appropriate error for an invalid transition
func (o *Order) transitionError(target Status) error {
	switch {
	case o.Status == StatusCancelled:
		return ErrOrderCancelled
	case o.Status == StatusShipped && target == StatusCancelled:
		return ErrOrderShipped
	case (o.Status == StatusPaid || o.Status == StatusShipped) && target == StatusPaid:
		return ErrOrderAlreadyPaid
	case o.Status == StatusPending && target == StatusShipped:
		return ErrOrderNotPaid
	default:
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStatus, o.Status, target)
	}
}

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

// Aggregate interface implementation
func (o *Order) GetID() string      { return o.ID }
func (o *Order) GetVersion() int    { return o.Version }
func (o *Order) SetVersion(v int)   { o.Version = v }

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

// ApplyEvent applies a single event to the order state (implements aggregate.Aggregate)
func (o *Order) ApplyEvent(event store.Event) error {
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
	order, found, err := aggregate.LoadAggregate(ctx, s.eventStore, orderID, func() *Order {
		return &Order{}
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrOrderNotFound
	}
	return order, nil
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
	if err := aggregate.MaybeCreateSnapshot(ctx, s.eventStore, order, AggregateType); err != nil {
		log.Printf("[Order] Failed to create snapshot for order %s: %v", order.ID, err)
	}

	return order, nil
}

func (s *Service) Pay(ctx context.Context, orderID string) error {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}

	if !order.CanTransitionTo(StatusPaid) {
		return order.transitionError(StatusPaid)
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
	if err := aggregate.MaybeCreateSnapshot(ctx, s.eventStore, order, AggregateType); err != nil {
		log.Printf("[Order] Failed to create snapshot for order %s: %v", order.ID, err)
	}

	return nil
}

func (s *Service) Ship(ctx context.Context, orderID string) error {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}

	if !order.CanTransitionTo(StatusShipped) {
		return order.transitionError(StatusShipped)
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
	if err := aggregate.MaybeCreateSnapshot(ctx, s.eventStore, order, AggregateType); err != nil {
		log.Printf("[Order] Failed to create snapshot for order %s: %v", order.ID, err)
	}

	return nil
}

func (s *Service) Cancel(ctx context.Context, orderID, reason string) error {
	order, err := s.loadOrder(ctx, orderID)
	if err != nil {
		return err
	}

	if !order.CanTransitionTo(StatusCancelled) {
		return order.transitionError(StatusCancelled)
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
	if err := aggregate.MaybeCreateSnapshot(ctx, s.eventStore, order, AggregateType); err != nil {
		log.Printf("[Order] Failed to create snapshot for order %s: %v", order.ID, err)
	}

	return nil
}
