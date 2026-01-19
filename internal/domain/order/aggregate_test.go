package order

import (
	"context"
	"testing"

	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOrderService() (*Service, *mocks.MockEventStore) {
	eventStore := mocks.NewMockEventStore()
	service := NewService(eventStore)
	return service, eventStore
}

// ============================================
// Place Order Tests
// ============================================

func TestService_Place_Success(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	items := []OrderItem{
		{ProductID: "prod-1", Quantity: 2, Price: 1000},
		{ProductID: "prod-2", Quantity: 1, Price: 2000},
	}

	order, err := service.Place(ctx, "user-123", items)

	require.NoError(t, err)
	assert.NotEmpty(t, order.ID)
	assert.Equal(t, "user-123", order.UserID)
	assert.Equal(t, items, order.Items)
	assert.Equal(t, 4000, order.Total) // 2*1000 + 1*2000
	assert.Equal(t, StatusPending, order.Status)

	// Verify event was stored
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventOrderPlaced, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, AggregateType, eventStore.AppendCalls[0].AggregateType)
}

func TestService_Place_SingleItem(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	items := []OrderItem{
		{ProductID: "prod-1", Quantity: 1, Price: 500},
	}

	order, err := service.Place(ctx, "user-123", items)

	require.NoError(t, err)
	assert.Equal(t, 500, order.Total)
}

func TestService_Place_EmptyItems(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	order, err := service.Place(ctx, "user-123", []OrderItem{})

	assert.ErrorIs(t, err, ErrEmptyOrder)
	assert.Nil(t, order)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Place_NilItems(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	order, err := service.Place(ctx, "user-123", nil)

	assert.ErrorIs(t, err, ErrEmptyOrder)
	assert.Nil(t, order)
}

// ============================================
// Pay Order Tests - State Transitions
// ============================================

func TestService_Pay_FromPending_Success(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	// Create an order first
	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{
		OrderID: orderID,
		UserID:  "user-123",
	})

	err := service.Pay(ctx, orderID)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventOrderPaid, eventStore.AppendCalls[0].EventType)
}

func TestService_Pay_OrderNotFound(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	err := service.Pay(ctx, "non-existent-order")

	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestService_Pay_AlreadyPaid(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

	err := service.Pay(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderAlreadyPaid)
}

func TestService_Pay_AlreadyShipped(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderShipped, OrderShipped{OrderID: orderID})

	err := service.Pay(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderAlreadyPaid)
}

func TestService_Pay_Cancelled(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderCancelled, OrderCancelled{OrderID: orderID})

	err := service.Pay(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderCancelled)
}

// ============================================
// Ship Order Tests - State Transitions
// ============================================

func TestService_Ship_FromPaid_Success(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

	err := service.Ship(ctx, orderID)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventOrderShipped, eventStore.AppendCalls[0].EventType)
}

func TestService_Ship_OrderNotFound(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	err := service.Ship(ctx, "non-existent-order")

	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestService_Ship_FromPending(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})

	err := service.Ship(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderNotPaid)
}

func TestService_Ship_AlreadyShipped(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderShipped, OrderShipped{OrderID: orderID})

	err := service.Ship(ctx, orderID)

	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestService_Ship_Cancelled(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderCancelled, OrderCancelled{OrderID: orderID})

	err := service.Ship(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderCancelled)
}

// ============================================
// Cancel Order Tests - State Transitions
// ============================================

func TestService_Cancel_FromPending_Success(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})

	err := service.Cancel(ctx, orderID, "customer request")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventOrderCancelled, eventStore.AppendCalls[0].EventType)

	// Verify reason is stored
	call := eventStore.AppendCalls[0]
	data := call.Data.(OrderCancelled)
	assert.Equal(t, "customer request", data.Reason)
}

func TestService_Cancel_FromPaid_Success(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

	err := service.Cancel(ctx, orderID, "refund requested")

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventOrderCancelled, eventStore.AppendCalls[0].EventType)
}

func TestService_Cancel_OrderNotFound(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	err := service.Cancel(ctx, "non-existent-order", "reason")

	assert.ErrorIs(t, err, ErrOrderNotFound)
}

func TestService_Cancel_FromShipped(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderShipped, OrderShipped{OrderID: orderID})

	err := service.Cancel(ctx, orderID, "too late")

	assert.ErrorIs(t, err, ErrOrderShipped)
}

func TestService_Cancel_AlreadyCancelled(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	eventStore.AddEvent(orderID, AggregateType, EventOrderCancelled, OrderCancelled{OrderID: orderID})

	err := service.Cancel(ctx, orderID, "duplicate cancel")

	assert.ErrorIs(t, err, ErrOrderCancelled)
}

// ============================================
// State Transition Matrix Tests
// ============================================

func TestRebuildStatus(t *testing.T) {
	service, eventStore := newTestOrderService()
	orderID := "order-123"

	tests := []struct {
		name           string
		events         []string
		expectedStatus Status
	}{
		{
			name:           "no events (default)",
			events:         []string{},
			expectedStatus: StatusPending,
		},
		{
			name:           "placed only",
			events:         []string{EventOrderPlaced},
			expectedStatus: StatusPending,
		},
		{
			name:           "placed then paid",
			events:         []string{EventOrderPlaced, EventOrderPaid},
			expectedStatus: StatusPaid,
		},
		{
			name:           "placed, paid, shipped",
			events:         []string{EventOrderPlaced, EventOrderPaid, EventOrderShipped},
			expectedStatus: StatusShipped,
		},
		{
			name:           "placed then cancelled",
			events:         []string{EventOrderPlaced, EventOrderCancelled},
			expectedStatus: StatusCancelled,
		},
		{
			name:           "placed, paid then cancelled",
			events:         []string{EventOrderPlaced, EventOrderPaid, EventOrderCancelled},
			expectedStatus: StatusCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventStore.Reset()
			for _, eventType := range tt.events {
				eventStore.AddEvent(orderID, AggregateType, eventType, struct{}{})
			}

			events := eventStore.GetEvents(orderID)
			status := service.rebuildStatus(events)

			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// ============================================
// Full Order Lifecycle Test
// ============================================

func TestOrderLifecycle_HappyPath(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	// 1. Place order
	items := []OrderItem{
		{ProductID: "prod-1", Quantity: 1, Price: 1000},
	}
	order, err := service.Place(ctx, "user-123", items)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, order.Status)

	// 2. Pay order
	err = service.Pay(ctx, order.ID)
	require.NoError(t, err)

	// 3. Ship order
	err = service.Ship(ctx, order.ID)
	require.NoError(t, err)
}

func TestOrderLifecycle_CancelAfterPlace(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	// 1. Place order
	items := []OrderItem{
		{ProductID: "prod-1", Quantity: 1, Price: 1000},
	}
	order, err := service.Place(ctx, "user-123", items)
	require.NoError(t, err)

	// 2. Cancel order
	err = service.Cancel(ctx, order.ID, "changed mind")
	require.NoError(t, err)

	// 3. Cannot pay cancelled order
	err = service.Pay(ctx, order.ID)
	assert.ErrorIs(t, err, ErrOrderCancelled)
}

func TestOrderLifecycle_CancelAfterPay(t *testing.T) {
	service, _ := newTestOrderService()
	ctx := context.Background()

	// 1. Place order
	items := []OrderItem{
		{ProductID: "prod-1", Quantity: 1, Price: 1000},
	}
	order, err := service.Place(ctx, "user-123", items)
	require.NoError(t, err)

	// 2. Pay order
	err = service.Pay(ctx, order.ID)
	require.NoError(t, err)

	// 3. Cancel order (refund case)
	err = service.Cancel(ctx, order.ID, "refund")
	require.NoError(t, err)

	// 4. Cannot ship cancelled order
	err = service.Ship(ctx, order.ID)
	assert.ErrorIs(t, err, ErrOrderCancelled)
}
