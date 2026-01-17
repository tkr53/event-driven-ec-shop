package order

import (
	"context"
	"errors"
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
	ErrOrderNotFound  = errors.New("order not found")
	ErrEmptyOrder     = errors.New("order must have at least one item")
	ErrInvalidStatus  = errors.New("invalid order status transition")
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

	event := OrderCancelled{
		OrderID:     orderID,
		Reason:      reason,
		CancelledAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, orderID, AggregateType, EventOrderCancelled, event)
	return err
}
