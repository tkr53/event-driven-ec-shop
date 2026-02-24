package notification

import (
	"context"
	"encoding/json"
	"log/slog"

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

// HandleEvent processes an event from Kinesis
func (h *Handler) HandleEvent(ctx context.Context, key, value []byte) error {
	var event store.Event
	if err := json.Unmarshal(value, &event); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal event", "error", err)
		return err
	}

	// Only process OrderPlaced events
	if event.EventType == order.EventOrderPlaced {
		return h.handleOrderPlaced(ctx, event)
	}

	return nil
}

func (h *Handler) handleOrderPlaced(ctx context.Context, event store.Event) error {
	var e order.OrderPlaced
	if err := json.Unmarshal(event.Data, &e); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal OrderPlaced event", "error", err)
		return err
	}

	slog.InfoContext(ctx, "processing OrderPlaced event", "order_id", e.OrderID, "user_id", e.UserID)

	// Get user information from read store
	userData, exists, err := h.readStore.Get("users", e.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get user", "user_id", e.UserID, "error", err)
		return nil
	}
	if !exists {
		slog.WarnContext(ctx, "user not found", "user_id", e.UserID)
		return nil
	}

	user, ok := userData.(*readmodel.UserReadModel)
	if !ok {
		slog.WarnContext(ctx, "invalid user data type", "user_id", e.UserID)
		return nil
	}

	// Convert order items to email items
	emailItems := make([]email.OrderItem, len(e.Items))
	for i, item := range e.Items {
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
		slog.ErrorContext(ctx, "failed to send email", "email", user.Email, "error", err)
		return err
	}

	slog.InfoContext(ctx, "order confirmation email sent", "email", user.Email, "order_id", e.OrderID)
	return nil
}
