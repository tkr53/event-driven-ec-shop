package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

const AggregateType = "Inventory"

var (
	ErrInsufficientStock = errors.New("insufficient stock")
	ErrInvalidQuantity   = errors.New("quantity must be positive")
)

type Inventory struct {
	ProductID     string
	TotalStock    int
	ReservedStock int
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

func (s *Service) AddStock(ctx context.Context, productID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	event := StockAdded{
		ProductID: productID,
		Quantity:  quantity,
		AddedAt:   time.Now(),
	}

	_, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockAdded, event)
	return err
}

func (s *Service) Reserve(ctx context.Context, productID, orderID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	event := StockReserved{
		ProductID:  productID,
		OrderID:    orderID,
		Quantity:   quantity,
		ReservedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockReserved, event)
	return err
}

func (s *Service) Release(ctx context.Context, productID, orderID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	event := StockReleased{
		ProductID:  productID,
		OrderID:    orderID,
		Quantity:   quantity,
		ReleasedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockReleased, event)
	return err
}

func (s *Service) Deduct(ctx context.Context, productID, orderID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	event := StockDeducted{
		ProductID:  productID,
		OrderID:    orderID,
		Quantity:   quantity,
		DeductedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, productID, AggregateType, EventStockDeducted, event)
	return err
}
