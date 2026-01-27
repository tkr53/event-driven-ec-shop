package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshot_Struct(t *testing.T) {
	state := map[string]interface{}{
		"id":     "order-123",
		"status": "paid",
		"total":  1000,
	}
	stateJSON, err := json.Marshal(state)
	require.NoError(t, err)

	snapshot := Snapshot{
		AggregateID:   "order-123",
		AggregateType: "Order",
		Version:       10,
		State:         stateJSON,
		CreatedAt:     time.Now(),
	}

	assert.Equal(t, "order-123", snapshot.AggregateID)
	assert.Equal(t, "Order", snapshot.AggregateType)
	assert.Equal(t, 10, snapshot.Version)
	assert.NotEmpty(t, snapshot.State)
	assert.NotZero(t, snapshot.CreatedAt)
}

func TestSnapshot_JSONMarshalUnmarshal(t *testing.T) {
	state := map[string]interface{}{
		"id":     "order-123",
		"status": "paid",
	}
	stateJSON, err := json.Marshal(state)
	require.NoError(t, err)

	original := Snapshot{
		AggregateID:   "order-123",
		AggregateType: "Order",
		Version:       10,
		State:         stateJSON,
		CreatedAt:     time.Now().Truncate(time.Second),
	}

	// Marshal
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal
	var restored Snapshot
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.AggregateID, restored.AggregateID)
	assert.Equal(t, original.AggregateType, restored.AggregateType)
	assert.Equal(t, original.Version, restored.Version)
	assert.JSONEq(t, string(original.State), string(restored.State))
}

func TestSnapshotThreshold(t *testing.T) {
	assert.Equal(t, 10, SnapshotThreshold)
}

func TestSnapshot_StateContainsValidJSON(t *testing.T) {
	ctx := context.Background()

	type OrderState struct {
		ID     string `json:"id"`
		UserID string `json:"user_id"`
		Status string `json:"status"`
		Total  int    `json:"total"`
	}

	originalState := OrderState{
		ID:     "order-123",
		UserID: "user-456",
		Status: "paid",
		Total:  5000,
	}

	stateJSON, err := json.Marshal(originalState)
	require.NoError(t, err)

	snapshot := &Snapshot{
		AggregateID:   "order-123",
		AggregateType: "Order",
		Version:       10,
		State:         stateJSON,
		CreatedAt:     time.Now(),
	}

	// Verify we can unmarshal the state back
	var restoredState OrderState
	err = json.Unmarshal(snapshot.State, &restoredState)
	require.NoError(t, err)

	assert.Equal(t, originalState.ID, restoredState.ID)
	assert.Equal(t, originalState.UserID, restoredState.UserID)
	assert.Equal(t, originalState.Status, restoredState.Status)
	assert.Equal(t, originalState.Total, restoredState.Total)

	_ = ctx // Used to show this is ready for integration tests
}
