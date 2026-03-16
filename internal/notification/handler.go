package notification

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/example/ec-event-driven/internal/domain/order"
	"github.com/example/ec-event-driven/internal/email"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/readmodel"
	orderpb "github.com/example/ec-event-driven/proto/domain/orderpb"
	"google.golang.org/protobuf/proto"
)

type Handler struct {
	emailService *email.Service
	readStore    store.ReadStoreInterface
}

func NewHandler(emailSvc *email.Service, readStore store.ReadStoreInterface) *Handler {
	return &Handler{
		emailService: emailSvc,
		readStore:    readStore,
	}
}

func (h *Handler) HandleEvent(ctx context.Context, key, value []byte) error {
	var event store.Event
	if err := json.Unmarshal(value, &event); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal event", "error", err)
		return err
	}

	if event.EventType == order.EventOrderPlaced {
		return h.handleOrderPlaced(ctx, event)
	}

	return nil
}

func (h *Handler) handleOrderPlaced(ctx context.Context, event store.Event) error {
	var binaryData []byte
	if err := json.Unmarshal(event.Data, &binaryData); err != nil {
		slog.ErrorContext(ctx, "failed to decode event data", "error", err)
		return err
	}

	var e orderpb.OrderPlacedEvent
	if err := proto.Unmarshal(binaryData, &e); err != nil {
		slog.ErrorContext(ctx, "failed to unmarshal OrderPlaced protobuf", "error", err)
		return err
	}

	slog.InfoContext(ctx, "processing OrderPlaced event", "order_id", e.OrderId, "user_id", e.UserId)

	userData, exists, err := h.readStore.Get("users", e.UserId)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get user", "user_id", e.UserId, "error", err)
		return nil
	}
	if !exists {
		slog.WarnContext(ctx, "user not found", "user_id", e.UserId)
		return nil
	}

	user, ok := userData.(*readmodel.UserReadModel)
	if !ok {
		slog.WarnContext(ctx, "invalid user data type", "user_id", e.UserId)
		return nil
	}

	emailItems := make([]email.OrderItem, len(e.Items))
	for i, item := range e.Items {
		productName := item.ProductId
		if productData, exists, _ := h.readStore.Get("products", item.ProductId); exists {
			if product, ok := productData.(*readmodel.ProductReadModel); ok {
				productName = product.Name
			}
		}

		emailItems[i] = email.OrderItem{
			ProductID: item.ProductId,
			Name:      productName,
			Quantity:  int(item.Quantity),
			Price:     int(item.Price),
		}
	}

	if err := h.emailService.SendOrderConfirmation(user.Email, e.OrderId, int(e.Total), emailItems); err != nil {
		slog.ErrorContext(ctx, "failed to send email", "email", user.Email, "error", err)
		return err
	}

	slog.InfoContext(ctx, "order confirmation email sent", "email", user.Email, "order_id", e.OrderId)
	return nil
}
