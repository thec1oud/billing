package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	AggregateID   string          `json:"aggregate_id"`
	Sequence      int64           `json:"sequence"`
	Actor         json.RawMessage `json:"actor"`
	Payload       json.RawMessage `json:"payload"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CausationID   *uuid.UUID      `json:"causation_id,omitempty"`
	CorrelationID *uuid.UUID      `json:"correlation_id,omitempty"`
}

// UnmarshalPayload deserializes the raw JSON payload into the target struct.
func (e Event) UnmarshalPayload(target any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(e.Payload, target); err != nil {
		return fmt.Errorf("unmarshal event payload (%s): %w", e.EventType, err)
	}
	return nil
}

// AppendRequest contains user-provided execution mutations parameters
type AppendRequest struct {
	AggregateType AggregateType
	AggregateID   string
	Sequence      int64
	EventType     EventType
	EventVersion  int
	Actor         Actor
	CausationID   *uuid.UUID
	CorrelationID *uuid.UUID
	Payload       any
}

func (req AppendRequest) Validate() error {

	if req.AggregateID == "" {
		return errors.New("aggregate_id cannot be empty")
	}
	if req.AggregateType == "" {
		return errors.New("aggregate_type cannot be empty")
	}
	if req.EventType == "" {
		return errors.New("event_type cannot be empty")
	}
	if req.EventVersion <= 0 {
		return errors.New("event_version must be greater than zero")
	}
	return nil
}

// EventStore contract interface for out-of-package boundary callers
type EventStore interface {
	Append(ctx context.Context, req AppendRequest) (uuid.UUID, error)
	ReadStream(ctx context.Context, aggregateType AggregateType, aggregateID string) ([]Event, error)
	ReadStreamFrom(ctx context.Context, aggregateType AggregateType,
		aggregateID string, fromSequence int64) ([]Event, error)
}

// Reducer functions represent mathematical deterministic folds transforming history back into runtime structures
type Reducer[State any] func(State, Event) (State, error)

// Rebuild events out ledger history and compiles system state on the fly
func Rebuild[State any](initial State, events []Event, reduce Reducer[State]) (State, error) {
	state := initial
	for _, event := range events {
		var err error
		state, err = reduce(state, event)
		if err != nil {
			return state, fmt.Errorf("rebuild failed at sequence %d (%s): %w", event.Sequence, event.EventType, err)
		}
	}
	return state, nil
}

type ActorType string

const (
	ActorTypeUser   ActorType = "USER"
	ActorTypeSystem ActorType = "SYSTEM"
)

type Actor struct {
	ID   string    `json:"id"`
	Type ActorType `json:"type"`
	Name string    `json:"name,omitempty"`
}
