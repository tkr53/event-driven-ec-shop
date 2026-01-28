package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/example/ec-event-driven/internal/infrastructure/store"
)

// Aggregate defines the interface for event-sourced aggregates
type Aggregate interface {
	GetID() string
	GetVersion() int
	SetVersion(int)
	ApplyEvent(store.Event) error
}

// LoadAggregate loads an aggregate by replaying events, using snapshot if available
// Returns the aggregate, a boolean indicating if data was found, and any error
func LoadAggregate[T Aggregate](
	ctx context.Context,
	eventStore store.EventStoreInterface,
	id string,
	newAggregate func() T,
) (T, bool, error) {
	agg := newAggregate()

	snapshot, err := eventStore.GetSnapshot(ctx, id)
	if err != nil {
		var zero T
		return zero, false, fmt.Errorf("failed to get snapshot: %w", err)
	}

	var events []store.Event
	if snapshot != nil {
		if err := json.Unmarshal(snapshot.State, agg); err != nil {
			var zero T
			return zero, false, fmt.Errorf("failed to unmarshal snapshot: %w", err)
		}
		events = eventStore.GetEventsFromVersion(ctx, id, snapshot.Version)
	} else {
		events = eventStore.GetEvents(id)
	}

	// Check if any data was found
	hasData := snapshot != nil || len(events) > 0

	for _, event := range events {
		if err := agg.ApplyEvent(event); err != nil {
			var zero T
			return zero, false, fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return agg, hasData, nil
}

// MaybeCreateSnapshot creates a snapshot if the threshold is exceeded
func MaybeCreateSnapshot(
	ctx context.Context,
	eventStore store.EventStoreInterface,
	agg Aggregate,
	aggregateType string,
) error {
	version := agg.GetVersion()
	if version > 0 && version%store.SnapshotThreshold == 0 {
		state, err := json.Marshal(agg)
		if err != nil {
			return fmt.Errorf("failed to marshal aggregate state: %w", err)
		}

		snapshot := &store.Snapshot{
			AggregateID:   agg.GetID(),
			AggregateType: aggregateType,
			Version:       version,
			State:         state,
			CreatedAt:     time.Now(),
		}

		if err := eventStore.SaveSnapshot(ctx, snapshot); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}
	}
	return nil
}
