package notification

import (
	"context"
	"encoding/json"
	"log"

	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/email"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
)

// Handler processes events for sending notifications
type Handler struct {
	emailService *email.Service
	readStore    store.ReadStoreInterface
}

// NewHandler creates a new notification handler
func NewHandler(emailSvc *email.Service, readStore store.ReadStoreInterface) *Handler {
	return &Handler{
		emailService: emailSvc,
		readStore:    readStore,
	}
}

// HandleEvent processes an event from Kafka
func (h *Handler) HandleEvent(ctx context.Context, key, value []byte) error {
	var event store.Event
	if err := json.Unmarshal(value, &event); err != nil {
		log.Printf("[Notifier] Failed to unmarshal event: %v", err)
		return err
	}

	// Only process OrderPlaced events
	if event.EventType == order.EventOrderPlaced {
		return h.handleOrderPlaced(event)
	}

	return nil
}

func (h *Handler) handleOrderPlaced(event store.Event) error {
	var e order.OrderPlaced
	if err := json.Unmarshal(event.Data, &e); err != nil {
		log.Printf("[Notifier] Failed to unmarshal OrderPlaced event: %v", err)
		return err
	}

	log.Printf("[Notifier] Processing OrderPlaced event for order %s, user %s", e.OrderID, e.UserID)

	// Get user information from read store
	userData, exists, err := h.readStore.Get("users", e.UserID)
	if err != nil {
		log.Printf("[Notifier] Error getting user %s: %v", e.UserID, err)
		return nil
	}
	if !exists {
		log.Printf("[Notifier] User not found: %s", e.UserID)
		return nil
	}

	user, ok := userData.(*readmodel.UserReadModel)
	if !ok {
		log.Printf("[Notifier] Invalid user data type for user: %s", e.UserID)
		return nil
	}

	// Convert order items to email items
	emailItems := make([]email.OrderItem, len(e.Items))
	for i, item := range e.Items {
		// Try to get product name from read store
		productName := item.ProductID
		if productData, exists, _ := h.readStore.Get("products", item.ProductID); exists {
			if product, ok := productData.(*readmodel.ProductReadModel); ok {
				productName = product.Name
			}
		}

		emailItems[i] = email.OrderItem{
			ProductID: item.ProductID,
			Name:      productName,
			Quantity:  item.Quantity,
			Price:     item.Price,
		}
	}

	// Send order confirmation email
	if err := h.emailService.SendOrderConfirmation(user.Email, e.OrderID, e.Total, emailItems); err != nil {
		log.Printf("[Notifier] Failed to send email to %s: %v", user.Email, err)
		return err
	}

	log.Printf("[Notifier] Order confirmation email sent to %s for order %s", user.Email, e.OrderID)
	return nil
}
