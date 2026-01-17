package cart

import (
	"context"
	"errors"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

const AggregateType = "Cart"

var (
	ErrInvalidQuantity = errors.New("quantity must be positive")
	ErrInvalidProduct  = errors.New("product_id is required")
)

type CartItem struct {
	ProductID string
	Quantity  int
	Price     int
}

type Cart struct {
	ID     string
	UserID string
	Items  map[string]CartItem // productID -> item
}

type Service struct {
	eventStore store.EventStoreInterface
}

func NewService(es store.EventStoreInterface) *Service {
	return &Service{eventStore: es}
}

// GetCartID returns the cart ID for a user (using userID as cartID for simplicity)
func GetCartID(userID string) string {
	return "cart-" + userID
}

func (s *Service) AddItem(ctx context.Context, userID, productID string, quantity, price int) error {
	if productID == "" {
		return ErrInvalidProduct
	}
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	cartID := GetCartID(userID)
	event := ItemAddedToCart{
		CartID:    cartID,
		UserID:    userID,
		ProductID: productID,
		Quantity:  quantity,
		Price:     price,
		AddedAt:   time.Now(),
	}

	_, err := s.eventStore.Append(ctx, cartID, AggregateType, EventItemAdded, event)
	return err
}

func (s *Service) RemoveItem(ctx context.Context, userID, productID string) error {
	if productID == "" {
		return ErrInvalidProduct
	}

	cartID := GetCartID(userID)
	event := ItemRemovedFromCart{
		CartID:    cartID,
		UserID:    userID,
		ProductID: productID,
		RemovedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, cartID, AggregateType, EventItemRemoved, event)
	return err
}

func (s *Service) Clear(ctx context.Context, userID string) error {
	cartID := GetCartID(userID)
	event := CartCleared{
		CartID:    cartID,
		UserID:    userID,
		ClearedAt: time.Now(),
	}

	_, err := s.eventStore.Append(ctx, cartID, AggregateType, EventCartCleared, event)
	return err
}
