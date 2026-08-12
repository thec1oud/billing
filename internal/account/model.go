package account

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	events "github.com/thec1oud/billing/internal/shared/eventstore"
)

type Status string

const (
	StatusPendingVerification Status = "PENDING_VERIFICATION"
	StatusActive              Status = "ACTIVE"
	StatusSuspended           Status = "SUSPENDED"
	StatusClosed              Status = "CLOSED"
)

var (
	ErrInvalidStateTransition = errors.New("invalid account state transition")
	ErrAccountClosed          = errors.New("account is closed")
	ErrAccountNotFound        = errors.New("account not found")
	ErrAccountIDMismatch      = errors.New("account id mismatch")
	ErrInvalidAccountEvent    = errors.New("invalid account event")
	ErrInvalidSequence        = errors.New("invalid account event sequence")
)

// Account is the Account aggregate.
//
// The aggregate is reconstructed exclusively by replaying its
// event stream.
type Account struct {
	// Identity
	AccountID  uuid.UUID
	ExternalID string

	// Lifecycle
	Status Status

	// Billing configuration
	Currency string
	Timezone string
	Locale   string
	NetTerms int16

	// Compliance
	TaxIdentifiers  json.RawMessage
	ComplianceFlags json.RawMessage

	// Billing
	BillingAddress   json.RawMessage
	DunningProfileID *int64

	// Additional information
	Metadata json.RawMessage

	// Payment methods attached to the account.
	PaymentMethods []string

	// Number of the last applied event.
	Version int64
}

// Apply applies exactly one event to the Account aggregate.
func (a *Account) Apply(event events.Event) error {
	if event.Sequence <= 0 {
		return ErrInvalidSequence
	}

	// Every event after creation must belong to the same aggregate.
	if a.Version > 0 && a.AccountID != uuid.Nil {
		if event.AggregateID != a.AccountID.String() {
			return ErrAccountIDMismatch
		}

		if event.Sequence != a.Version+1 {
			return fmt.Errorf(
				"%w: expected %d, got %d",
				ErrInvalidSequence,
				a.Version+1,
				event.Sequence,
			)
		}
	}

	switch event.EventType {
	case events.AccountCreated:
		return a.applyAccountCreated(event)

	case events.AccountActivated:
		return a.applyAccountActivated(event)

	case events.AccountSuspended:
		return a.applyAccountSuspended(event)

	case events.AccountClosed:
		return a.applyAccountClosed(event)

	case events.PaymentMethodAdded:
		return a.applyPaymentMethodAdded(event)

	default:
		return fmt.Errorf(
			"%w: unsupported event type %q",
			ErrInvalidAccountEvent,
			event.EventType,
		)
	}
}

func (a *Account) applyAccountCreated(event events.Event) error {
	if a.Version != 0 {
		return fmt.Errorf(
			"%w: account already created",
			ErrInvalidAccountEvent,
		)
	}

	var payload AccountCreated

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode account.created: %w", err)
	}

	if payload.AccountID == uuid.Nil {
		return fmt.Errorf(
			"%w: account id cannot be empty",
			ErrInvalidAccountEvent,
		)
	}

	if event.AggregateID != payload.AccountID.String() {
		return ErrAccountIDMismatch
	}

	a.AccountID = payload.AccountID
	a.ExternalID = payload.ExternalID

	a.Currency = payload.Currency
	a.Timezone = payload.Timezone
	a.Locale = payload.Locale
	a.NetTerms = payload.NetTerms

	a.DunningProfileID = payload.DunningProfileID

	a.TaxIdentifiers = cloneJSON(payload.TaxIdentifiers)
	a.BillingAddress = cloneJSON(payload.BillingAddress)
	a.ComplianceFlags = cloneJSON(payload.ComplianceFlags)
	a.Metadata = cloneJSON(payload.Metadata)

	a.PaymentMethods = []string{}

	// Creation starts the lifecycle here.
	a.Status = StatusPendingVerification
	a.Version = event.Sequence

	return nil
}

func (a *Account) applyAccountActivated(event events.Event) error {
	if a.Status != StatusPendingVerification {
		return fmt.Errorf(
			"%w: cannot activate account from %s",
			ErrInvalidStateTransition,
			a.Status,
		)
	}

	var payload AccountActivated

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode account.activated: %w", err)
	}

	if payload.AccountID != a.AccountID {
		return ErrAccountIDMismatch
	}

	a.Status = StatusActive
	a.Version = event.Sequence

	return nil
}

func (a *Account) applyAccountSuspended(event events.Event) error {
	if a.Status != StatusActive {
		return fmt.Errorf(
			"%w: cannot suspend account from %s",
			ErrInvalidStateTransition,
			a.Status,
		)
	}

	var payload AccountSuspended

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode account.suspended: %w", err)
	}

	if payload.AccountID != a.AccountID {
		return ErrAccountIDMismatch
	}

	a.Status = StatusSuspended
	a.Version = event.Sequence

	return nil
}

func (a *Account) applyAccountClosed(event events.Event) error {
	if a.Status != StatusActive && a.Status != StatusSuspended {
		return fmt.Errorf(
			"%w: cannot close account from %s",
			ErrInvalidStateTransition,
			a.Status,
		)
	}

	var payload AccountClosed

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode account.closed: %w", err)
	}

	if payload.AccountID != a.AccountID {
		return ErrAccountIDMismatch
	}

	a.Status = StatusClosed
	a.Version = event.Sequence

	return nil
}

func (a *Account) applyPaymentMethodAdded(event events.Event) error {
	if a.Status == StatusClosed {
		return ErrAccountClosed
	}

	if a.Status != StatusActive {
		return fmt.Errorf(
			"%w: payment method requires ACTIVE account",
			ErrInvalidStateTransition,
		)
	}

	var payload PaymentMethodAdded

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode payment_method_added: %w", err)
	}

	if payload.AccountID != a.AccountID {
		return ErrAccountIDMismatch
	}

	a.PaymentMethods = append(
		a.PaymentMethods,
		payload.PaymentMethodID,
	)

	a.Version = event.Sequence

	return nil
}

// Rebuild reconstructs the Account aggregate from its event stream.
func Rebuild(stream []events.Event) (*Account, error) {
	account := &Account{}

	for _, event := range stream {
		if err := account.Apply(event); err != nil {
			return nil, err
		}
	}

	if account.Version == 0 {
		return nil, ErrAccountNotFound
	}

	return account, nil
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}

	result := make([]byte, len(value))
	copy(result, value)

	return result
}
