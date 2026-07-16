package events

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	AccountCreated      EventType = "billing.account.created"
	SubscriptionCreated EventType = "billing.subscription.created"
	InvoiceCreated      EventType = "billing.invoice.created"
	InvoiceFinalized    EventType = "billing.invoice.finalized"
	InvoicePaid         EventType = "billing.invoice.paid"
	PaymentAttempted    EventType = "billing.payment.attempted"
	PaymentSucceeded    EventType = "billing.payment.succeeded"
	PaymentFailed       EventType = "billing.payment.failed"
)

type AggregateType string

const (
	AggregateAccount      AggregateType = "ACCOUNT"
	AggregateSubscription AggregateType = "SUBSCRIPTION"
	AggregateInvoice      AggregateType = "INVOICE"
)

// Event is the immutable envelope stored for every business-state change.
type Event struct {
	EventID       uuid.UUID       `json:"event_id"`
	EventType     EventType       `json:"event_type"`
	EventVersion  int             `json:"event_version"`
	AggregateType AggregateType   `json:"aggregate_type"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	Sequence      int64           `json:"sequence"`
	Actor         string          `json:"actor"`
	Payload       json.RawMessage `json:"payload"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CausationID   *uuid.UUID      `json:"causation_id,omitempty"`
	CorrelationID *uuid.UUID      `json:"correlation_id,omitempty"`
}

// AppendRequest contains the caller-provided event data. EventID and
// OccurredAt are created by EventStore so every stored event has a UTC UUIDv7
// envelope.
type AppendRequest struct {
	AggregateType AggregateType
	AggregateID   uuid.UUID
	Sequence      int64
	EventType     EventType
	EventVersion  int
	Actor         string
	Payload       any
	CausationID   *uuid.UUID
	CorrelationID *uuid.UUID
}
