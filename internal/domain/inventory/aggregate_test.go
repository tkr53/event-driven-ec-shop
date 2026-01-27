package inventory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestInventoryService() (*Service, *mocks.MockEventStore) {
	eventStore := mocks.NewMockEventStore()
	service := NewService(eventStore)
	return service, eventStore
}

// ============================================
// Inventory Struct Tests
// ============================================

func TestInventory_AvailableStock(t *testing.T) {
	tests := []struct {
		name           string
		totalStock     int
		reservedStock  int
		expectedAvail  int
	}{
		{"no reservations", 100, 0, 100},
		{"some reserved", 100, 30, 70},
		{"all reserved", 50, 50, 0},
		{"zero stock", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := Inventory{
				ProductID:     "prod-1",
				TotalStock:    tt.totalStock,
				ReservedStock: tt.reservedStock,
			}

			assert.Equal(t, tt.expectedAvail, inv.AvailableStock())
		})
	}
}

// ============================================
// Add Stock Tests
// ============================================

func TestService_AddStock_ValidQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.AddStock(ctx, "prod-123", 100)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventStockAdded, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, AggregateType, eventStore.AppendCalls[0].AggregateType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(StockAdded)
	assert.Equal(t, "prod-123", data.ProductID)
	assert.Equal(t, 100, data.Quantity)
}

func TestService_AddStock_SingleUnit(t *testing.T) {
	service, _ := newTestInventoryService()
	ctx := context.Background()

	err := service.AddStock(ctx, "prod-123", 1)

	require.NoError(t, err)
}

func TestService_AddStock_ZeroQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.AddStock(ctx, "prod-123", 0)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_AddStock_NegativeQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.AddStock(ctx, "prod-123", -10)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

// ============================================
// Reserve Stock Tests
// ============================================

func TestService_Reserve_ValidQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Reserve(ctx, "prod-123", "order-456", 5)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventStockReserved, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(StockReserved)
	assert.Equal(t, "prod-123", data.ProductID)
	assert.Equal(t, "order-456", data.OrderID)
	assert.Equal(t, 5, data.Quantity)
}

func TestService_Reserve_ZeroQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Reserve(ctx, "prod-123", "order-456", 0)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Reserve_NegativeQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Reserve(ctx, "prod-123", "order-456", -5)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

// ============================================
// Release Stock Tests
// ============================================

func TestService_Release_ValidQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Release(ctx, "prod-123", "order-456", 5)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventStockReleased, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(StockReleased)
	assert.Equal(t, "prod-123", data.ProductID)
	assert.Equal(t, "order-456", data.OrderID)
	assert.Equal(t, 5, data.Quantity)
}

func TestService_Release_ZeroQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Release(ctx, "prod-123", "order-456", 0)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Release_NegativeQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Release(ctx, "prod-123", "order-456", -5)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

// ============================================
// Deduct Stock Tests
// ============================================

func TestService_Deduct_ValidQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Deduct(ctx, "prod-123", "order-456", 10)

	require.NoError(t, err)
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventStockDeducted, eventStore.AppendCalls[0].EventType)

	// Verify event data
	data := eventStore.AppendCalls[0].Data.(StockDeducted)
	assert.Equal(t, "prod-123", data.ProductID)
	assert.Equal(t, "order-456", data.OrderID)
	assert.Equal(t, 10, data.Quantity)
}

func TestService_Deduct_ZeroQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Deduct(ctx, "prod-123", "order-456", 0)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

func TestService_Deduct_NegativeQuantity(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	err := service.Deduct(ctx, "prod-123", "order-456", -10)

	assert.ErrorIs(t, err, ErrInvalidQuantity)
	assert.Empty(t, eventStore.AppendCalls)
}

// ============================================
// Integration-like Tests
// ============================================

func TestInventoryOperations_Sequence(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	productID := "prod-123"
	orderID := "order-456"

	// 1. Add initial stock
	err := service.AddStock(ctx, productID, 100)
	require.NoError(t, err)

	// 2. Reserve some stock
	err = service.Reserve(ctx, productID, orderID, 20)
	require.NoError(t, err)

	// 3. Release some reserved stock
	err = service.Release(ctx, productID, orderID, 5)
	require.NoError(t, err)

	// 4. Deduct from stock
	err = service.Deduct(ctx, productID, orderID, 15)
	require.NoError(t, err)

	// Verify all events were recorded
	assert.Len(t, eventStore.AppendCalls, 4)
	assert.Equal(t, EventStockAdded, eventStore.AppendCalls[0].EventType)
	assert.Equal(t, EventStockReserved, eventStore.AppendCalls[1].EventType)
	assert.Equal(t, EventStockReleased, eventStore.AppendCalls[2].EventType)
	assert.Equal(t, EventStockDeducted, eventStore.AppendCalls[3].EventType)
}

// ============================================
// Snapshot Tests
// ============================================

func TestInventoryService_SnapshotCreatedAtThreshold(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	productID := "prod-snapshot"

	// Add stock 9 times
	for i := 1; i <= 9; i++ {
		err := service.AddStock(ctx, productID, 10)
		require.NoError(t, err)
	}

	// Reset snapshot calls counter
	eventStore.SaveSnapshotCalls = nil

	// The 10th event should trigger a snapshot
	err := service.AddStock(ctx, productID, 10)
	require.NoError(t, err)

	// Verify snapshot was created
	assert.Len(t, eventStore.SaveSnapshotCalls, 1)
	assert.Equal(t, productID, eventStore.SaveSnapshotCalls[0].Snapshot.AggregateID)
	assert.Equal(t, 10, eventStore.SaveSnapshotCalls[0].Snapshot.Version)
}

func TestInventoryService_LoadInventoryFromSnapshot(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	productID := "prod-with-snapshot"

	// Create a snapshot with some stock
	snapshotState := Inventory{
		ProductID:     productID,
		TotalStock:    100,
		ReservedStock: 20,
		Version:       10,
	}
	stateJSON, _ := json.Marshal(snapshotState)
	eventStore.SetSnapshot(&store.Snapshot{
		AggregateID:   productID,
		AggregateType: AggregateType,
		Version:       10,
		State:         stateJSON,
	})

	// Add more stock - this should load from snapshot first
	err := service.AddStock(ctx, productID, 50)
	require.NoError(t, err)

	// Verify the stock added event was appended
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventStockAdded, eventStore.AppendCalls[0].EventType)
}

func TestInventoryService_LoadInventoryFromSnapshotWithSubsequentEvents(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	productID := "prod-snapshot-with-events"

	// Create a snapshot at version 5
	snapshotState := Inventory{
		ProductID:     productID,
		TotalStock:    100,
		ReservedStock: 0,
		Version:       5,
	}
	stateJSON, _ := json.Marshal(snapshotState)
	eventStore.SetSnapshot(&store.Snapshot{
		AggregateID:   productID,
		AggregateType: AggregateType,
		Version:       5,
		State:         stateJSON,
	})

	// Add events after the snapshot
	eventStore.SetEvents(productID, []store.Event{
		{
			Version:   6,
			EventType: EventStockReserved,
			Data:      mustMarshalInventory(StockReserved{ProductID: productID, OrderID: "order-1", Quantity: 30}),
		},
	})

	// Reserve more stock - this should work after loading from snapshot + events
	err := service.Reserve(ctx, productID, "order-2", 20)
	require.NoError(t, err)

	// Verify the reserve event was appended
	assert.Len(t, eventStore.AppendCalls, 1)
	assert.Equal(t, EventStockReserved, eventStore.AppendCalls[0].EventType)
}

func TestInventoryService_SnapshotStateCorrectness(t *testing.T) {
	service, eventStore := newTestInventoryService()
	ctx := context.Background()

	productID := "prod-state-test"

	// Add events to build up state
	for i := 1; i <= 9; i++ {
		err := service.AddStock(ctx, productID, 10)
		require.NoError(t, err)
	}

	// The 10th event should trigger a snapshot with correct state
	err := service.AddStock(ctx, productID, 10)
	require.NoError(t, err)

	// Verify snapshot was created with correct state
	require.Len(t, eventStore.SaveSnapshotCalls, 1)
	snapshot := eventStore.SaveSnapshotCalls[0].Snapshot

	var savedState Inventory
	err = json.Unmarshal(snapshot.State, &savedState)
	require.NoError(t, err)

	// Total stock should be 100 (10 * 10)
	assert.Equal(t, 100, savedState.TotalStock)
	assert.Equal(t, 0, savedState.ReservedStock)
	assert.Equal(t, productID, savedState.ProductID)
}

func mustMarshalInventory(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}
