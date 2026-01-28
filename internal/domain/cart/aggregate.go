package cart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

const AggregateType = "Cart"

var (
	ErrInvalidQuantity = errors.New("quantity must be positive")
	ErrInvalidProduct  = errors.New("product_id is required")
)

type CartItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Price     int    `json:"price"`
}

type Cart struct {
	ID      string              `json:"id"`
	UserID  string              `json:"user_id"`
	Items   map[string]CartItem `json:"items"` // productID -> item
	Version int                 `json:"version"`
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

// applyEvent applies a single event to the cart state
func (c *Cart) applyEvent(event store.Event) error {
	switch event.EventType {
	case EventItemAdded:
		var data ItemAddedToCart
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		if c.Items == nil {
			c.Items = make(map[string]CartItem)
		}
		c.ID = data.CartID
		c.UserID = data.UserID
		// Add or update item quantity
		if existing, ok := c.Items[data.ProductID]; ok {
			existing.Quantity += data.Quantity
			existing.Price = data.Price
			c.Items[data.ProductID] = existing
		} else {
			c.Items[data.ProductID] = CartItem{
				ProductID: data.ProductID,
				Quantity:  data.Quantity,
				Price:     data.Price,
			}
		}
	case EventItemRemoved:
		var data ItemRemovedFromCart
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		delete(c.Items, data.ProductID)
	case EventCartCleared:
		var data CartCleared
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		c.Items = make(map[string]CartItem)
	}
	c.Version = event.Version
	return nil
}

// loadCart loads a cart by replaying events, using snapshot if available
func (s *Service) loadCart(ctx context.Context, cartID string) (*Cart, error) {
	cart := &Cart{Items: make(map[string]CartItem)}

	// Try to load from snapshot first
	snapshot, err := s.eventStore.GetSnapshot(ctx, cartID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	var events []store.Event
	if snapshot != nil {
		// Restore state from snapshot
		if err := json.Unmarshal(snapshot.State, cart); err != nil {
			return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
		}
		// Get only events after the snapshot
		events = s.eventStore.GetEventsFromVersion(ctx, cartID, snapshot.Version)
	} else {
		// No snapshot, get all events
		events = s.eventStore.GetEvents(cartID)
	}

	// Apply remaining events
	for _, event := range events {
		if err := cart.applyEvent(event); err != nil {
			return nil, fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return cart, nil
}

// maybeCreateSnapshot creates a snapshot if the threshold is exceeded
func (s *Service) maybeCreateSnapshot(ctx context.Context, cart *Cart) error {
	if cart.Version > 0 && cart.Version%store.SnapshotThreshold == 0 {
		state, err := json.Marshal(cart)
		if err != nil {
			return fmt.Errorf("failed to marshal cart state: %w", err)
		}

		snapshot := &store.Snapshot{
			AggregateID:   cart.ID,
			AggregateType: AggregateType,
			Version:       cart.Version,
			State:         state,
			CreatedAt:     time.Now(),
		}

		if err := s.eventStore.SaveSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}
	}
	return nil
}

func (s *Service) AddItem(ctx context.Context, userID, productID string, quantity, price int) error {
	if productID == "" {
		return ErrInvalidProduct
	}
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	cartID := GetCartID(userID)

	// Load current cart state for snapshot check
	cart, err := s.loadCart(ctx, cartID)
	if err != nil {
		// Cart doesn't exist yet, create new one
		cart = &Cart{
			ID:     cartID,
			UserID: userID,
			Items:  make(map[string]CartItem),
		}
	}

	event := ItemAddedToCart{
		CartID:    cartID,
		UserID:    userID,
		ProductID: productID,
		Quantity:  quantity,
		Price:     price,
		AddedAt:   time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, cartID, AggregateType, EventItemAdded, event)
	if err != nil {
		return err
	}

	// Update cart for snapshot check
	if storedEvent != nil {
		cart.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, cart); err != nil {
		log.Printf("[Cart] Failed to create snapshot for cart %s: %v", cart.ID, err)
	}

	return nil
}

func (s *Service) RemoveItem(ctx context.Context, userID, productID string) error {
	if productID == "" {
		return ErrInvalidProduct
	}

	cartID := GetCartID(userID)

	// Load current cart state for snapshot check
	cart, err := s.loadCart(ctx, cartID)
	if err != nil {
		cart = &Cart{
			ID:     cartID,
			UserID: userID,
			Items:  make(map[string]CartItem),
		}
	}

	event := ItemRemovedFromCart{
		CartID:    cartID,
		UserID:    userID,
		ProductID: productID,
		RemovedAt: time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, cartID, AggregateType, EventItemRemoved, event)
	if err != nil {
		return err
	}

	// Update cart for snapshot check
	if storedEvent != nil {
		cart.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, cart); err != nil {
		log.Printf("[Cart] Failed to create snapshot for cart %s: %v", cart.ID, err)
	}

	return nil
}

func (s *Service) Clear(ctx context.Context, userID string) error {
	cartID := GetCartID(userID)

	// Load current cart state for snapshot check
	cart, err := s.loadCart(ctx, cartID)
	if err != nil {
		cart = &Cart{
			ID:     cartID,
			UserID: userID,
			Items:  make(map[string]CartItem),
		}
	}

	event := CartCleared{
		CartID:    cartID,
		UserID:    userID,
		ClearedAt: time.Now(),
	}

	storedEvent, err := s.eventStore.Append(ctx, cartID, AggregateType, EventCartCleared, event)
	if err != nil {
		return err
	}

	// Update cart for snapshot check
	if storedEvent != nil {
		cart.Version = storedEvent.Version
	}

	// Check if we need to create a snapshot
	if err := s.maybeCreateSnapshot(ctx, cart); err != nil {
		log.Printf("[Cart] Failed to create snapshot for cart %s: %v", cart.ID, err)
	}

	return nil
}
