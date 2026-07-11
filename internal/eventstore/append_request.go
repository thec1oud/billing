package eventstore

import (
	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/shared/events"
)

type AppendRequest struct {

	AggregateType events.AggregateType

	AggregateID uuid.UUID

	Sequence int64

	EventType events.EventType

	EventVersion int

	Actor string

	Payload any

	CausationID *uuid.UUID

	CorrelationID *uuid.UUID
}