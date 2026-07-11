package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)


type Event struct {

	// Globally unique identifier (UUIDv7)
	EventID uuid.UUID `json:"event_id"`

	// Event schema identifier
	EventType EventType `json:"event_type"`

	// Event schema version
	EventVersion int `json:"event_version"`

	// Aggregate information
	AggregateType AggregateType `json:"aggregate_type"`
	AggregateID   uuid.UUID     `json:"aggregate_id"`

	// Monotonically increasing sequence
	Sequence int64 `json:"sequence"`

	// Actor responsible for this event
	Actor string `json:"actor"`

	// Event payload
	Payload json.RawMessage `json:"payload"`

	// UTC timestamp
	OccurredAt time.Time `json:"occurred_at"`

	// Event relationships
	CausationID  *uuid.UUID `json:"causation_id,omitempty"`
	CorrelationID *uuid.UUID `json:"correlation_id,omitempty"`
}