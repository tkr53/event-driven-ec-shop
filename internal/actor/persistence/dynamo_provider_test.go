package persistence

import (
	"testing"

	"github.com/example/ec-event-driven/internal/infrastructure/store/mocks"
	pb "github.com/example/ec-event-driven/proto/domain/productpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func init() {
	RegisterEventType("ProductCreated", func() proto.Message { return &pb.ProductCreatedEvent{} })
	RegisterEventType("ProductUpdated", func() proto.Message { return &pb.ProductUpdatedEvent{} })
	RegisterEventType("ProductDeleted", func() proto.Message { return &pb.ProductDeletedEvent{} })
	RegisterAggregateType("Product", "Product")
	RegisterSnapshotFactory("Product", func() proto.Message { return &pb.ProductSnapshot{} })
}

func TestParseActorName(t *testing.T) {
	aggType, aggID := ParseActorName("Product:abc-123")
	assert.Equal(t, "Product", aggType)
	assert.Equal(t, "abc-123", aggID)
}

func TestPersistAndGetEvents(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	provider := NewDynamoProvider(mockStore, 10)
	state := provider.GetState()

	event := &pb.ProductCreatedEvent{
		ProductId:   "prod-1",
		Name:        "Test Product",
		Description: "A test product",
		Price:       1000,
		Stock:       50,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	state.PersistEvent("Product:prod-1", 1, event)

	require.Len(t, mockStore.AppendCalls, 1)
	call := mockStore.AppendCalls[0]
	assert.Equal(t, "prod-1", call.AggregateID)
	assert.Equal(t, "Product", call.AggregateType)
	assert.Equal(t, "ProductCreated", call.EventType)

	// Verify data is stored as []byte (protobuf binary)
	_, ok := call.Data.([]byte)
	assert.True(t, ok, "expected data to be []byte (protobuf binary), got %T", call.Data)

	var receivedEvents []interface{}
	state.GetEvents("Product:prod-1", 0, 0, func(e interface{}) {
		receivedEvents = append(receivedEvents, e)
	})

	require.Len(t, receivedEvents, 1)
	got, ok := receivedEvents[0].(*pb.ProductCreatedEvent)
	require.True(t, ok)
	assert.Equal(t, "prod-1", got.ProductId)
	assert.Equal(t, "Test Product", got.Name)
	assert.Equal(t, int32(1000), got.Price)
}

func TestPersistAndGetSnapshot(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	provider := NewDynamoProvider(mockStore, 10)
	state := provider.GetState()

	snapshot := &pb.ProductSnapshot{
		Id:          "prod-1",
		Name:        "Test Product",
		Description: "A test product",
		Price:       1000,
		Stock:       50,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	state.PersistSnapshot("Product:prod-1", 5, snapshot)

	require.Len(t, mockStore.SaveSnapshotCalls, 1)

	got, idx, ok := state.GetSnapshot("Product:prod-1")
	require.True(t, ok)
	assert.Equal(t, 5, idx)

	gotSnapshot, ok := got.(*pb.ProductSnapshot)
	require.True(t, ok)
	assert.Equal(t, "prod-1", gotSnapshot.Id)
	assert.Equal(t, "Test Product", gotSnapshot.Name)
	assert.Equal(t, int32(1000), gotSnapshot.Price)
}

func TestGetSnapshot_NotFound(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	provider := NewDynamoProvider(mockStore, 10)
	state := provider.GetState()

	_, _, ok := state.GetSnapshot("Product:nonexistent")
	assert.False(t, ok)
}

func TestGetSnapshotInterval(t *testing.T) {
	mockStore := mocks.NewMockEventStore()
	provider := NewDynamoProvider(mockStore, 10)
	assert.Equal(t, 10, provider.GetState().GetSnapshotInterval())
}

func TestFormatActorName(t *testing.T) {
	assert.Equal(t, "Product:abc-123", FormatActorName("Product", "abc-123"))
}

func TestEventTypeRegistration(t *testing.T) {
	factory, ok := EventTypeRegistry["ProductCreated"]
	require.True(t, ok)

	msg := factory()
	_, ok = msg.(*pb.ProductCreatedEvent)
	assert.True(t, ok)
}

func TestProtobufBinaryRoundTrip(t *testing.T) {
	event := &pb.ProductCreatedEvent{
		ProductId:   "prod-1",
		Name:        "テスト商品",
		Description: "説明文",
		Price:       2000,
		Stock:       100,
		CreatedAt:   "2026-01-01T00:00:00Z",
	}

	binaryData, err := proto.Marshal(event)
	require.NoError(t, err)

	restored := &pb.ProductCreatedEvent{}
	err = proto.Unmarshal(binaryData, restored)
	require.NoError(t, err)

	assert.Equal(t, event.ProductId, restored.ProductId)
	assert.Equal(t, event.Name, restored.Name)
	assert.Equal(t, event.Price, restored.Price)
	assert.Equal(t, event.Stock, restored.Stock)
}
