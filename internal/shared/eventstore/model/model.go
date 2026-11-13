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

// EventType names a domain event. Values are dot-separated and always carry
// the "billing." prefix, e.g. "billing.invoice.paid". State machine definitions
// reference these same strings in EmitEventType; the loader rejects any value
// that is not listed in knownEventTypes.
type EventType string

const (
	AccountCreated       EventType = "billing.account.created"
	AccountActivated     EventType = "billing.account.activated"
	AccountSuspended     EventType = "billing.account.suspended"
	AccountClosed        EventType = "billing.account.closed"
	PaymentMethodAdded   EventType = "billing.account.payment_method_added"
	SubscriptionCreated  EventType = "billing.subscription.created"
	SubscriptionPaused   EventType = "billing.subscription.paused"
	SubscriptionResumed  EventType = "billing.subscription.resumed"
	SubscriptionCanceled EventType = "billing.subscription.canceled"
	InvoiceCreated       EventType = "billing.invoice.created"
	InvoiceFinalized     EventType = "billing.invoice.finalized"
	InvoicePaid          EventType = "billing.invoice.paid"
	InvoiceVoided        EventType = "billing.invoice.voided"
	PaymentAttempted     EventType = "billing.payment.attempted"
	PaymentSucceeded     EventType = "billing.payment.succeeded"
	PaymentFailed        EventType = "billing.payment.failed"
)

var knownEventTypes = map[EventType]struct{}{
	AccountCreated:       {},
	AccountActivated:     {},
	AccountSuspended:     {},
	AccountClosed:        {},
	PaymentMethodAdded:   {},
	SubscriptionCreated:  {},
	SubscriptionPaused:   {},
	SubscriptionResumed:  {},
	SubscriptionCanceled: {},
	InvoiceCreated:       {},
	InvoiceFinalized:     {},
	InvoicePaid:          {},
	InvoiceVoided:        {},
	PaymentAttempted:     {},
	PaymentSucceeded:     {},
	PaymentFailed:        {},
}

// Valid reports whether t is one of the declared domain event types.
func (t EventType) Valid() bool {
	_, ok := knownEventTypes[t]
	return ok
}

// AggregateType identifies an event stream together with the aggregate ID.
//
// It is also the state machine SubjectType for the same entity: services must
// pass string(AggregateX) to CreateInstance/GetInstanceBySubject so that the
// engine, which emits AggregateType(instance.SubjectType), and the services
// append to one stream per aggregate.
type AggregateType string

const (
	AggregateAccount      AggregateType = "account"
	AggregateSubscription AggregateType = "subscription"
	AggregateInvoice      AggregateType = "invoice"
)

// Valid reports whether t is one of the declared aggregate types.
func (t AggregateType) Valid() bool {
	switch t {
	case AggregateAccount, AggregateSubscription, AggregateInvoice:
		return true
	}
	return false
}

// String returns the value as stored in event_log.aggregate_type and
// sm_instances.subject_type.
func (t AggregateType) String() string { return string(t) }

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
