package aggregate

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	pb "github.com/example/ec-event-driven/proto/domain/inventorypb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func spawnInventoryActor(t *testing.T, mockStore *mocks.MockEventStore) (*actor.ActorSystem, *actor.RootContext, *actor.PID) {
	t.Helper()
	system := actor.NewActorSystem()
	provider := actorpersistence.NewDynamoProvider(mockStore, 10)
	sysRef := &mockSystemRef{}

	props := actor.PropsFromProducer(
		func() actor.Actor { return NewInventoryActor(sysRef) },
		actor.WithReceiverMiddleware(persistence.Using(provider)),
	)

	pid, err := system.Root.SpawnNamed(props, "Inventory:prod-1")
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	return system, system.Root, pid
}

func TestInventoryActor_AddStock(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnInventoryActor(t, mockStore)
	defer system.Shutdown()

	cmd := &pb.AddStockCommand{ProductId: "prod-1", Quantity: 100}
	future := root.RequestFuture(pid, cmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	_, ok := result.(*CommandSuccess)
	require.True(t, ok, "expected *CommandSuccess, got %T", result)

	require.Len(t, mockStore.AppendCalls, 1)
	assert.Equal(t, "StockAdded", mockStore.AppendCalls[0].EventType)
}

func TestInventoryActor_ReserveStock(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnInventoryActor(t, mockStore)
	defer system.Shutdown()

	// Add stock first
	addCmd := &pb.AddStockCommand{ProductId: "prod-1", Quantity: 100}
	future := root.RequestFuture(pid, addCmd, 5*time.Second)
	_, err := future.Result()
	require.NoError(t, err)

	// Reserve
	reserveCmd := &pb.ReserveStockCommand{ProductId: "prod-1", OrderId: "order-1", Quantity: 30}
	future = root.RequestFuture(pid, reserveCmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	_, ok := result.(*CommandSuccess)
	require.True(t, ok)
	assert.Equal(t, "StockReserved", mockStore.AppendCalls[1].EventType)
}

func TestInventoryActor_ReserveStock_InsufficientStock(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnInventoryActor(t, mockStore)
	defer system.Shutdown()

	// Add stock
	addCmd := &pb.AddStockCommand{ProductId: "prod-1", Quantity: 10}
	future := root.RequestFuture(pid, addCmd, 5*time.Second)
	_, err := future.Result()
	require.NoError(t, err)

	// Reserve more than available
	reserveCmd := &pb.ReserveStockCommand{ProductId: "prod-1", OrderId: "order-1", Quantity: 20}
	future = root.RequestFuture(pid, reserveCmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	errResp, ok := result.(*ErrorResponse)
	require.True(t, ok)
	assert.Equal(t, ErrInsufficientStock, errResp.Err)
}

func TestInventoryActor_InvalidQuantity(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnInventoryActor(t, mockStore)
	defer system.Shutdown()

	cmd := &pb.AddStockCommand{ProductId: "prod-1", Quantity: 0}
	future := root.RequestFuture(pid, cmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	errResp, ok := result.(*ErrorResponse)
	require.True(t, ok)
	assert.Equal(t, ErrInvalidQuantity, errResp.Err)
}
