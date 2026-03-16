package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/asynkron/protoactor-go/persistence"
	"github.com/example/ec-event-driven/internal/infrastructure/store"
	"google.golang.org/protobuf/proto"
)

// EventTypeRegistry maps event_type strings to protobuf message constructors.
var EventTypeRegistry = map[string]func() proto.Message{}

// EventTypeNameRegistry maps protobuf full name to event_type strings.
var EventTypeNameRegistry = map[string]string{}

// RegisterEventType registers a bidirectional mapping between event_type string and protobuf message.
func RegisterEventType(eventType string, factory func() proto.Message) {
	EventTypeRegistry[eventType] = factory
	msg := factory()
	fullName := string(msg.ProtoReflect().Descriptor().FullName())
	EventTypeNameRegistry[fullName] = eventType
}

// AggregateTypeForActor maps actor prefixes to aggregate type strings.
var AggregateTypeForActor = map[string]string{}

func RegisterAggregateType(prefix, aggregateType string) {
	AggregateTypeForActor[prefix] = aggregateType
}

// ParseActorName splits "AggregateType:ID" into its components.
func ParseActorName(actorName string) (aggregateType, aggregateID string) {
	parts := strings.SplitN(actorName, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", actorName
}

type DynamoProvider struct {
	state *DynamoProviderState
}

func NewDynamoProvider(eventStore store.EventStoreInterface, snapshotInterval int) *DynamoProvider {
	return &DynamoProvider{
		state: &DynamoProviderState{
			eventStore:       eventStore,
			snapshotInterval: snapshotInterval,
		},
	}
}

func (p *DynamoProvider) GetState() persistence.ProviderState {
	return p.state
}

// DynamoProviderState implements persistence.ProviderState using protobuf binary serialization.
// Binary data is stored as base64-encoded JSON strings via Go's json.Marshal([]byte).
type DynamoProviderState struct {
	eventStore       store.EventStoreInterface
	snapshotInterval int
}

func (s *DynamoProviderState) Restart() {}

func (s *DynamoProviderState) GetSnapshotInterval() int {
	return s.snapshotInterval
}

// GetEvents loads events from DynamoDB and deserializes protobuf binary back into messages.
func (s *DynamoProviderState) GetEvents(actorName string, eventIndexStart int, eventIndexEnd int, callback func(e interface{})) {
	_, aggregateID := ParseActorName(actorName)
	ctx := context.Background()

	var events []store.Event
	if eventIndexStart > 0 {
		events = s.eventStore.GetEventsFromVersion(ctx, aggregateID, eventIndexStart)
	} else {
		events = s.eventStore.GetEvents(aggregateID)
	}

	for _, event := range events {
		if eventIndexEnd > 0 && event.Version > eventIndexEnd {
			break
		}

		factory, ok := EventTypeRegistry[event.EventType]
		if !ok {
			slog.Warn("unknown event type in registry", "event_type", event.EventType, "aggregate_id", aggregateID)
			continue
		}

		// event.Data is json.RawMessage containing base64-encoded protobuf binary
		var binaryData []byte
		if err := json.Unmarshal(event.Data, &binaryData); err != nil {
			slog.Error("failed to decode event data from base64", "event_type", event.EventType, "error", err)
			continue
		}

		msg := factory()
		if err := proto.Unmarshal(binaryData, msg); err != nil {
			slog.Error("failed to unmarshal protobuf event", "event_type", event.EventType, "error", err)
			continue
		}

		callback(msg)
	}
}

// PersistEvent serializes a protobuf message to binary and appends to the event store.
// The binary bytes are passed as []byte to Append, which json.Marshal encodes to base64.
func (s *DynamoProviderState) PersistEvent(actorName string, eventIndex int, event proto.Message) {
	prefix, aggregateID := ParseActorName(actorName)
	ctx := context.Background()

	aggregateType, ok := AggregateTypeForActor[prefix]
	if !ok {
		slog.Error("unknown aggregate type for actor", "prefix", prefix, "actor_name", actorName)
		return
	}

	fullName := string(event.ProtoReflect().Descriptor().FullName())
	eventType, ok := EventTypeNameRegistry[fullName]
	if !ok {
		slog.Error("unknown event type name", "proto_name", fullName)
		return
	}

	binaryData, err := proto.Marshal(event)
	if err != nil {
		slog.Error("failed to marshal protobuf event", "event_type", eventType, "error", err)
		return
	}

	_, err = s.eventStore.Append(ctx, aggregateID, aggregateType, eventType, binaryData)
	if err != nil {
		slog.Error("failed to append event", "event_type", eventType, "aggregate_id", aggregateID, "error", err)
	}
}

func (s *DynamoProviderState) DeleteEvents(_ string, _ int) {}

// GetSnapshot loads a snapshot and deserializes from protobuf binary.
func (s *DynamoProviderState) GetSnapshot(actorName string) (snapshot interface{}, eventIndex int, ok bool) {
	_, aggregateID := ParseActorName(actorName)
	ctx := context.Background()

	snap, err := s.eventStore.GetSnapshot(ctx, aggregateID)
	if err != nil || snap == nil {
		return nil, 0, false
	}

	prefix, _ := ParseActorName(actorName)
	snapshotFactory, exists := snapshotFactoryRegistry[prefix]
	if !exists {
		slog.Warn("no snapshot factory registered", "prefix", prefix)
		return nil, 0, false
	}

	// snap.State is json.RawMessage containing base64-encoded protobuf binary
	var binaryData []byte
	if err := json.Unmarshal(snap.State, &binaryData); err != nil {
		slog.Error("failed to decode snapshot from base64", "error", err)
		return nil, 0, false
	}

	msg := snapshotFactory()
	if err := proto.Unmarshal(binaryData, msg); err != nil {
		slog.Error("failed to unmarshal protobuf snapshot", "error", err)
		return nil, 0, false
	}

	return msg, snap.Version, true
}

// PersistSnapshot serializes a snapshot to protobuf binary and saves to DynamoDB.
func (s *DynamoProviderState) PersistSnapshot(actorName string, snapshotIndex int, snapshot proto.Message) {
	prefix, aggregateID := ParseActorName(actorName)
	ctx := context.Background()

	aggregateType, ok := AggregateTypeForActor[prefix]
	if !ok {
		slog.Error("unknown aggregate type for snapshot", "prefix", prefix)
		return
	}

	binaryData, err := proto.Marshal(snapshot)
	if err != nil {
		slog.Error("failed to marshal protobuf snapshot", "error", err)
		return
	}

	// Wrap binary in JSON base64 string for json.RawMessage compatibility
	encoded, err := json.Marshal(binaryData)
	if err != nil {
		slog.Error("failed to encode snapshot to base64", "error", err)
		return
	}

	err = s.eventStore.SaveSnapshot(ctx, &store.Snapshot{
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		Version:       snapshotIndex,
		State:         encoded,
	})
	if err != nil {
		slog.Error("failed to save snapshot", "aggregate_id", aggregateID, "error", err)
	}
}

func (s *DynamoProviderState) DeleteSnapshots(_ string, _ int) {}

var snapshotFactoryRegistry = map[string]func() proto.Message{}

func RegisterSnapshotFactory(prefix string, factory func() proto.Message) {
	snapshotFactoryRegistry[prefix] = factory
}

func FormatActorName(aggregateType, aggregateID string) string {
	return fmt.Sprintf("%s:%s", aggregateType, aggregateID)
}
