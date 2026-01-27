package store

import (
	"encoding/json"
	"time"
)

// SnapshotThreshold defines the number of events after which a snapshot is created
const SnapshotThreshold = 10

// Snapshot represents a point-in-time state of an aggregate
type Snapshot struct {
	AggregateID   string          `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	Version       int             `json:"version"`    // Event version at snapshot time
	State         json.RawMessage `json:"state"`      // Serialized aggregate state
	CreatedAt     time.Time       `json:"created_at"`
}
