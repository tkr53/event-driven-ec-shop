package order

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
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
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{
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
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

	err := service.Pay(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderAlreadyPaid)
}

func TestService_Pay_AlreadyShipped(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderShipped, OrderShipped{OrderID: orderID})

	err := service.Pay(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderAlreadyPaid)
}

func TestService_Pay_Cancelled(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderCancelled, OrderCancelled{OrderID: orderID})

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
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

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
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})

	err := service.Ship(ctx, orderID)

	assert.ErrorIs(t, err, ErrOrderNotPaid)
}

func TestService_Ship_AlreadyShipped(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderShipped, OrderShipped{OrderID: orderID})

	err := service.Ship(ctx, orderID)

	assert.ErrorIs(t, err, ErrInvalidStatus)
}

func TestService_Ship_Cancelled(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderCancelled, OrderCancelled{OrderID: orderID})

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
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})

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
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

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
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderShipped, OrderShipped{OrderID: orderID})

	err := service.Cancel(ctx, orderID, "too late")

	assert.ErrorIs(t, err, ErrOrderShipped)
}

func TestService_Cancel_AlreadyCancelled(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderCancelled, OrderCancelled{OrderID: orderID})

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
				_ = eventStore.AddEvent(orderID, AggregateType, eventType, struct{}{})
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

// ============================================
// Error Path Tests
// ============================================

func TestService_Place_EventStoreError(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	eventStore.AppendErr = errors.New("database error")

	items := []OrderItem{
		{ProductID: "prod-1", Quantity: 1, Price: 1000},
	}

	order, err := service.Place(ctx, "user-123", items)

	assert.Error(t, err)
	assert.Nil(t, order)
}

func TestService_Pay_EventStoreError(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})

	// Set error for next append
	eventStore.AppendErr = errors.New("database error")

	err := service.Pay(ctx, orderID)

	assert.Error(t, err)
}

func TestService_Ship_EventStoreError(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

	eventStore.AppendErr = errors.New("database error")

	err := service.Ship(ctx, orderID)

	assert.Error(t, err)
}

func TestService_Cancel_EventStoreError(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-123"
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID})

	eventStore.AppendErr = errors.New("database error")

	err := service.Cancel(ctx, orderID, "reason")

	assert.Error(t, err)
}

// ============================================
// Snapshot Tests
// ============================================

func TestService_SnapshotCreatedAtThreshold(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	testOrderID := "snapshot-order"

	// Manually add 9 events using SetEvents (to control versions)
	events := make([]store.Event, 9)
	for i := 0; i < 9; i++ {
		events[i] = store.Event{
			Version:       i + 1,
			AggregateID:   testOrderID,
			AggregateType: AggregateType,
		}
		if i == 0 {
			events[i].EventType = EventOrderPlaced
			events[i].Data = mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})
		} else {
			// Just add paid events (we'll override status in the last one)
			events[i].EventType = EventOrderPaid
			events[i].Data = mustMarshal(OrderPaid{OrderID: testOrderID})
		}
	}
	// Set the last event to be "pending" state by making all intermediate events irrelevant
	// Actually, we need the order to be in pending state for Pay to work
	// Let's simplify: just have OrderPlaced + 8 more OrderPlaced events (not realistic but tests the snapshot)

	// Better approach: set up 9 events ending in pending state
	eventStore.SetEvents(testOrderID, []store.Event{
		{Version: 1, EventType: EventOrderPlaced, Data: mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})},
		{Version: 2, EventType: EventOrderPaid, Data: mustMarshal(OrderPaid{OrderID: testOrderID})},
		{Version: 3, EventType: EventOrderCancelled, Data: mustMarshal(OrderCancelled{OrderID: testOrderID})},
		{Version: 4, EventType: EventOrderPlaced, Data: mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})},
		{Version: 5, EventType: EventOrderPaid, Data: mustMarshal(OrderPaid{OrderID: testOrderID})},
		{Version: 6, EventType: EventOrderCancelled, Data: mustMarshal(OrderCancelled{OrderID: testOrderID})},
		{Version: 7, EventType: EventOrderPlaced, Data: mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})},
		{Version: 8, EventType: EventOrderPaid, Data: mustMarshal(OrderPaid{OrderID: testOrderID})},
		{Version: 9, EventType: EventOrderCancelled, Data: mustMarshal(OrderCancelled{OrderID: testOrderID})},
	})

	// Now we need to add one more event to get to 10
	// But the order is cancelled, so we can't pay it
	// Let's use a different approach: add OrderPlaced as the 9th event
	eventStore.Reset()
	eventStore.SetEvents(testOrderID, []store.Event{
		{Version: 1, EventType: EventOrderPlaced, Data: mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})},
		{Version: 2, EventType: EventOrderPaid, Data: mustMarshal(OrderPaid{OrderID: testOrderID})},
		{Version: 3, EventType: EventOrderShipped, Data: mustMarshal(OrderShipped{OrderID: testOrderID})},
		{Version: 4, EventType: EventOrderPlaced, Data: mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})},
		{Version: 5, EventType: EventOrderPaid, Data: mustMarshal(OrderPaid{OrderID: testOrderID})},
		{Version: 6, EventType: EventOrderShipped, Data: mustMarshal(OrderShipped{OrderID: testOrderID})},
		{Version: 7, EventType: EventOrderPlaced, Data: mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})},
		{Version: 8, EventType: EventOrderPaid, Data: mustMarshal(OrderPaid{OrderID: testOrderID})},
		{Version: 9, EventType: EventOrderPlaced, Data: mustMarshal(OrderPlaced{OrderID: testOrderID, UserID: "user-1"})},
	})

	// The 10th event (Pay) should trigger a snapshot
	err := service.Pay(ctx, testOrderID)
	require.NoError(t, err)

	// Verify snapshot was created
	assert.Len(t, eventStore.SaveSnapshotCalls, 1)
	assert.Equal(t, testOrderID, eventStore.SaveSnapshotCalls[0].Snapshot.AggregateID)
	assert.Equal(t, 10, eventStore.SaveSnapshotCalls[0].Snapshot.Version)
}

func TestService_LoadOrderFromSnapshot(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-with-snapshot"

	// Create a snapshot at version 10
	snapshotState := Order{
		ID:     orderID,
		UserID: "user-123",
		Items:  []OrderItem{{ProductID: "prod-1", Quantity: 2, Price: 1000}},
		Total:  2000,
		Status: StatusPaid,
	}
	stateJSON, _ := json.Marshal(snapshotState)
	eventStore.SetSnapshot(&store.Snapshot{
		AggregateID:   orderID,
		AggregateType: AggregateType,
		Version:       10,
		State:         stateJSON,
	})

	// Ship the order - this should load from snapshot first
	err := service.Ship(ctx, orderID)
	require.NoError(t, err)

	// Verify the ship event was appended
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventOrderShipped, eventStore.AppendCalls[0].EventType)
}

func TestService_LoadOrderFromSnapshotWithSubsequentEvents(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-with-snapshot-and-events"

	// Create a snapshot at version 5 with pending status
	snapshotState := Order{
		ID:     orderID,
		UserID: "user-123",
		Items:  []OrderItem{{ProductID: "prod-1", Quantity: 1, Price: 1000}},
		Total:  1000,
		Status: StatusPending,
	}
	stateJSON, _ := json.Marshal(snapshotState)
	eventStore.SetSnapshot(&store.Snapshot{
		AggregateID:   orderID,
		AggregateType: AggregateType,
		Version:       5,
		State:         stateJSON,
	})

	// Add events after the snapshot (versions 6, 7)
	eventStore.SetEvents(orderID, []store.Event{
		{Version: 6, EventType: EventOrderPaid, Data: mustMarshal(OrderPaid{OrderID: orderID})},
	})

	// Try to ship - should succeed because the order is paid (from event version 6)
	err := service.Ship(ctx, orderID)
	require.NoError(t, err)
}

func TestService_LoadOrderWithoutSnapshot(t *testing.T) {
	service, eventStore := newTestOrderService()
	ctx := context.Background()

	orderID := "order-no-snapshot"

	// Add events without snapshot
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPlaced, OrderPlaced{OrderID: orderID, UserID: "user-123"})
	_ = eventStore.AddEvent(orderID, AggregateType, EventOrderPaid, OrderPaid{OrderID: orderID})

	// Ship the order - should work by replaying all events
	err := service.Ship(ctx, orderID)
	require.NoError(t, err)
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
