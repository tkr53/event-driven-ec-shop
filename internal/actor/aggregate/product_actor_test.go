package aggregate

import (
	"testing"
	"time"

	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/persistence"
	actorpersistence "github.com/example/ec-event-driven/internal/actor/persistence"
	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	pb "github.com/example/ec-event-driven/proto/domain/productpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSystemRef struct{}

func (m *mockSystemRef) RemoveActor(_ string) {}

func spawnProductActor(t *testing.T, mockStore *mocks.MockEventStore) (*actor.ActorSystem, *actor.RootContext, *actor.PID) {
	t.Helper()
	system := actor.NewActorSystem()
	provider := actorpersistence.NewDynamoProvider(mockStore, 10)
	sysRef := &mockSystemRef{}

	props := actor.PropsFromProducer(
		func() actor.Actor { return NewProductActor(sysRef) },
		actor.WithReceiverMiddleware(persistence.Using(provider)),
	)

	pid, err := system.Root.SpawnNamed(props, "Product:test-product-1")
	require.NoError(t, err)

	// Wait for actor to be ready
	time.Sleep(100 * time.Millisecond)

	return system, system.Root, pid
}

func TestProductActor_CreateProduct(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnProductActor(t, mockStore)
	defer system.Shutdown()

	cmd := &pb.CreateProductCommand{
		ProductId:   "test-product-1",
		Name:        "テスト商品",
		Description: "テスト用の商品",
		Price:       2000,
		Stock:       100,
	}

	future := root.RequestFuture(pid, cmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	resp, ok := result.(*pb.ProductResponse)
	require.True(t, ok, "expected *pb.ProductResponse, got %T", result)
	assert.Equal(t, "test-product-1", resp.Id)
	assert.Equal(t, "テスト商品", resp.Name)
	assert.Equal(t, "テスト用の商品", resp.Description)
	assert.Equal(t, int32(2000), resp.Price)
	assert.Equal(t, int32(100), resp.Stock)

	require.Len(t, mockStore.AppendCalls, 1)
	assert.Equal(t, "ProductCreated", mockStore.AppendCalls[0].EventType)
}

func TestProductActor_CreateProduct_InvalidName(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnProductActor(t, mockStore)
	defer system.Shutdown()

	cmd := &pb.CreateProductCommand{
		ProductId: "prod-1",
		Name:      "",
		Price:     1000,
		Stock:     10,
	}

	future := root.RequestFuture(pid, cmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	errResp, ok := result.(*ErrorResponse)
	require.True(t, ok, "expected *ErrorResponse, got %T", result)
	assert.Equal(t, ErrProductInvalidName, errResp.Err)
	assert.Empty(t, mockStore.AppendCalls)
}

func TestProductActor_CreateProduct_InvalidPrice(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnProductActor(t, mockStore)
	defer system.Shutdown()

	cmd := &pb.CreateProductCommand{
		ProductId: "prod-1",
		Name:      "Product",
		Price:     0,
		Stock:     10,
	}

	future := root.RequestFuture(pid, cmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	errResp, ok := result.(*ErrorResponse)
	require.True(t, ok)
	assert.Equal(t, ErrProductInvalidPrice, errResp.Err)
}

func TestProductActor_UpdateProduct(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnProductActor(t, mockStore)
	defer system.Shutdown()

	// First create a product
	createCmd := &pb.CreateProductCommand{
		ProductId:   "test-product-1",
		Name:        "Original",
		Description: "Desc",
		Price:       1000,
		Stock:       50,
	}
	future := root.RequestFuture(pid, createCmd, 5*time.Second)
	_, err := future.Result()
	require.NoError(t, err)

	// Then update
	updateCmd := &pb.UpdateProductCommand{
		ProductId:   "test-product-1",
		Name:        "Updated",
		Description: "New Desc",
		Price:       1500,
	}
	future = root.RequestFuture(pid, updateCmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	_, ok := result.(*CommandSuccess)
	require.True(t, ok, "expected *CommandSuccess, got %T", result)

	require.Len(t, mockStore.AppendCalls, 2)
	assert.Equal(t, "ProductUpdated", mockStore.AppendCalls[1].EventType)
}

func TestProductActor_DeleteProduct(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnProductActor(t, mockStore)
	defer system.Shutdown()

	// Create
	createCmd := &pb.CreateProductCommand{
		ProductId: "test-product-1",
		Name:      "Product",
		Price:     1000,
		Stock:     10,
	}
	future := root.RequestFuture(pid, createCmd, 5*time.Second)
	_, err := future.Result()
	require.NoError(t, err)

	// Delete
	deleteCmd := &pb.DeleteProductCommand{ProductId: "test-product-1"}
	future = root.RequestFuture(pid, deleteCmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	_, ok := result.(*CommandSuccess)
	require.True(t, ok)

	assert.Equal(t, "ProductDeleted", mockStore.AppendCalls[1].EventType)
}

func TestProductActor_UpdateNonexistent(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnProductActor(t, mockStore)
	defer system.Shutdown()

	cmd := &pb.UpdateProductCommand{
		ProductId: "nonexistent",
		Name:      "Updated",
		Price:     1000,
	}

	future := root.RequestFuture(pid, cmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	errResp, ok := result.(*ErrorResponse)
	require.True(t, ok)
	assert.Equal(t, ErrProductNotFound, errResp.Err)
}

func TestProductActor_DeleteNonexistent(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	system, root, pid := spawnProductActor(t, mockStore)
	defer system.Shutdown()

	cmd := &pb.DeleteProductCommand{ProductId: "nonexistent"}

	future := root.RequestFuture(pid, cmd, 5*time.Second)
	result, err := future.Result()
	require.NoError(t, err)

	errResp, ok := result.(*ErrorResponse)
	require.True(t, ok)
	assert.Equal(t, ErrProductNotFound, errResp.Err)
}
