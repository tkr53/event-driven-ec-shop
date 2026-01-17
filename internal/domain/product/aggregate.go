package product

import (
	"context"
	"errors"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/google/uuid"
)

const AggregateType = "Product"

var (
	ErrProductNotFound = errors.New("product not found")
	ErrInvalidPrice    = errors.New("price must be positive")
	ErrInvalidName     = errors.New("name is required")
)

type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       int       `json:"price"`
	Stock       int       `json:"stock"`
	IsDeleted   bool      `json:"is_deleted,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Service struct {
	eventStore store.EventStoreInterface
}

func NewService(es store.EventStoreInterface) *Service {
	return &Service{eventStore: es}
}

func (s *Service) Create(ctx context.Context, name, description string, price, stock int) (*Product, error) {
	if name == "" {
		return nil, ErrInvalidName
	}
	if price <= 0 {
		return nil, ErrInvalidPrice
	}

	productID := uuid.New().String()
	now := time.Now()

	event := ProductCreated{
		ProductID:   productID,
		Name:        name,
		Description: description,
		Price:       price,
		Stock:       stock,
		CreatedAt:   now,
	}

	_, err := s.eventStore.Append(ctx, productID, AggregateType, EventProductCreated, event)
	if err != nil {
		return nil, err
	}

	return &Product{
		ID:          productID,
		Name:        name,
		Description: description,
		Price:       price,
		Stock:       stock,
		CreatedAt:   now,
	}, nil
}

func (s *Service) Update(ctx context.Context, productID, name, description string, price int) error {
	if name == "" {
		return ErrInvalidName
	}
	if price <= 0 {
		return ErrInvalidPrice
	}

	events := s.eventStore.GetEvents(productID)
	if len(events) == 0 {
		return ErrProductNotFound
	}

	event := ProductUpdated{
		ProductID:   productID,
		Name:        name,
		Description: description,
		Price:       price,
		UpdatedAt:   time.Now(),
	}

	_, err := s.eventStore.Append(ctx, productID, AggregateType, EventProductUpdated, event)
	return err
}

func (s *Service) Delete(ctx context.Context, productID string) error {
	events := s.eventStore.GetEvents(productID)
	if len(events) == 0 {
		return ErrProductNotFound
	}

	event := ProductDeleted{
		ProductID: productID,
		DeletedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, productID, AggregateType, EventProductDeleted, event)
	return err
}
