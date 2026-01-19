package order

import (
	"context"
	"errors"
	"fmt"
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
	ErrOrderNotFound      = errors.New("order not found")
	ErrEmptyOrder         = errors.New("order must have at least one item")
	ErrInvalidStatus      = errors.New("invalid order status transition")
	ErrOrderAlreadyPaid   = errors.New("order is already paid")
	ErrOrderNotPaid       = errors.New("order must be paid before shipping")
	ErrOrderShipped       = errors.New("cannot cancel shipped order")
	ErrOrderCancelled     = errors.New("order is already cancelled")
)

type Order struct {
	ID        string      `json:"id"`
	UserID    string      `json:"user_id"`
	Items     []OrderItem `json:"items"`
	Total     int         `json:"total"`
	Status    Status      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
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

	_, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderPlaced, event)
	if err != nil {
		return nil, err
	}

	return &Order{
		ID:        orderID,
		UserID:    userID,
		Items:     items,
		Total:     total,
		Status:    StatusPending,
		CreatedAt: now,
	}, nil
}

func (s *Service) Pay(ctx context.Context, orderID string) error {
	events := s.eventStore.GetEvents(orderID)
	if len(events) == 0 {
		return ErrOrderNotFound
	}

	// Validate current status
	currentStatus := s.rebuildStatus(events)
	switch currentStatus {
	case StatusPending:
		// Valid transition
	case StatusPaid:
		return ErrOrderAlreadyPaid
	case StatusShipped:
		return ErrOrderAlreadyPaid
	case StatusCancelled:
		return ErrOrderCancelled
	default:
		return fmt.Errorf("%w: cannot pay order in %s status", ErrInvalidStatus, currentStatus)
	}

	event := OrderPaid{
		OrderID: orderID,
		PaidAt:  time.Now(),
	}

	_, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderPaid, event)
	return err
}

func (s *Service) Ship(ctx context.Context, orderID string) error {
	events := s.eventStore.GetEvents(orderID)
	if len(events) == 0 {
		return ErrOrderNotFound
	}

	// Validate current status - can only ship paid orders
	currentStatus := s.rebuildStatus(events)
	switch currentStatus {
	case StatusPaid:
		// Valid transition
	case StatusPending:
		return ErrOrderNotPaid
	case StatusShipped:
		return fmt.Errorf("%w: order is already shipped", ErrInvalidStatus)
	case StatusCancelled:
		return ErrOrderCancelled
	default:
		return fmt.Errorf("%w: cannot ship order in %s status", ErrInvalidStatus, currentStatus)
	}

	event := OrderShipped{
		OrderID:   orderID,
		ShippedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderShipped, event)
	return err
}

func (s *Service) Cancel(ctx context.Context, orderID, reason string) error {
	events := s.eventStore.GetEvents(orderID)
	if len(events) == 0 {
		return ErrOrderNotFound
	}

	// Validate current status - cannot cancel shipped orders
	currentStatus := s.rebuildStatus(events)
	switch currentStatus {
	case StatusPending, StatusPaid:
		// Valid - can cancel pending or paid orders (with refund if paid)
	case StatusShipped:
		return ErrOrderShipped
	case StatusCancelled:
		return ErrOrderCancelled
	default:
		return fmt.Errorf("%w: cannot cancel order in %s status", ErrInvalidStatus, currentStatus)
	}

	event := OrderCancelled{
		OrderID:     orderID,
		Reason:      reason,
		CancelledAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderCancelled, event)
	return err
}
