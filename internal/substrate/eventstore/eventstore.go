package events

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrSequenceConflict = errors.New("event sequence conflict: stale or duplicate state write path")
	ErrInvalidSequence  = errors.New("event sequence must be a positive integer greater than zero")
)

// Domain Event Enumerations
type EventType string
const (
	AccountCreated      EventType = "billing.account.created"
	PaymentMethodAdded  EventType = "billing.account.payment_method_added"
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

// Event is the unalterable system envelope ledger item contract
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

// AppendRequest contains user-provided execution mutations parameters
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

// EventStore contract interface for out-of-package boundary callers (Track B & C)
type EventStore interface {
	Append(ctx context.Context, req AppendRequest) (uuid.UUID, error)
	ReadStream(ctx context.Context, aggregateType AggregateType, aggregateID uuid.UUID) ([]Event, error)
}

// Reducer functions represent mathematical deterministic folds transforming history back into runtime structures
type Reducer[State any] func(State, Event) (State, error)

// Rebuild streams out  ledger history and compiles  system state on the fly
func Rebuild[State any](initial State, stream []Event, reduce Reducer[State]) (State, error) {
	state := initial
	for _, event := range stream {
		var err error
		state, err = reduce(state, event)
		if err != nil {
			return state, err
		}
	}
	return state, nil
}
