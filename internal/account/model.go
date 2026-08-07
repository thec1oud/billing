package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	// ErrInvalidStateTransition is returned when an event attempts
	// to move an account from its current state to an invalid state.
	ErrInvalidStateTransition = errors.New("invalid account state transition")

	// ErrAccountClosed is returned when an operation attempts to
	// modify an account that has already been closed.
	ErrAccountClosed = errors.New("account is closed")

	// ErrAccountNotFound is returned when an account aggregate has
	// not been created yet.
	ErrAccountNotFound = errors.New("account not found")

	// ErrAccountIDMismatch is returned when an event belongs to a
	// different account than the aggregate being rebuilt.
	ErrAccountIDMismatch = errors.New("account id mismatch")

	// ErrInvalidAccountEvent is returned when an account event
	// cannot be applied to the current aggregate state.
	ErrInvalidAccountEvent = errors.New("invalid account event")
)

// Account is the Account aggregate.
//
// An Account represents a billing relationship and is the root
// aggregate to which billing activity is associated.
//
// The Account lifecycle follows the RFC/schema state machine:
//
//	PENDING_VERIFICATION -> ACTIVE
//	ACTIVE                -> SUSPENDED
//	ACTIVE                -> CLOSED
//	SUSPENDED             -> CLOSED
//
// CLOSED is terminal.
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

	// Dunning
	DunningProfileID *int64

	// Compliance
	TaxIdentifiers  map[string]string
	ComplianceFlags map[string]bool

	// Billing information
	BillingAddress map[string]string

	// Provider/payment references
	PaymentMethods []string

	// Arbitrary application metadata
	Metadata map[string]string

	// Event-sourcing version.
	//
	// This corresponds to the latest sequence number that has
	// already been applied to this aggregate.
	Version int64

	// Creation timestamp
	CreatedAt time.Time
}

// Apply updates the Account aggregate by applying one event.
//
// IMPORTANT:
// This function is the state-machine enforcement point for the
// event-sourced aggregate. We do not simply assign a status.
// Each state-changing event must be valid for the current state.
func (a *Account) Apply(event events.Event) error {
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

// applyAccountCreated applies the initial AccountCreated event.
//
// Valid initial state:
//
//	PENDING_VERIFICATION
//
// AccountCreated must be the first event in the account stream.
func (a *Account) applyAccountCreated(event events.Event) error {
	if a.Version != 0 || a.AccountID != uuid.Nil {
		return fmt.Errorf(
			"%w: account can only be created once",
			ErrInvalidStateTransition,
		)
	}

	var payload AccountCreated

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode AccountCreated event: %w", err)
	}

	if payload.AccountID == uuid.Nil {
		return fmt.Errorf(
			"%w: AccountCreated contains empty account id",
			ErrInvalidAccountEvent,
		)
	}

	a.AccountID = payload.AccountID
	a.ExternalID = payload.ExternalID

	// IMPORTANT:
	// Account creation does NOT activate the account.
	//
	// The RFC/schema state machine requires verification before
	// the account can become ACTIVE.
	a.Status = StatusPendingVerification

	a.Currency = payload.Currency
	a.Timezone = payload.Timezone
	a.Locale = payload.Locale
	a.NetTerms = payload.NetTerms

	a.DunningProfileID = payload.DunningProfileID

	a.TaxIdentifiers = cloneStringMap(payload.TaxIdentifiers)
	a.BillingAddress = cloneStringMap(payload.BillingAddress)
	a.ComplianceFlags = cloneBoolMap(payload.ComplianceFlags)
	a.Metadata = cloneStringMap(payload.Metadata)

	a.PaymentMethods = []string{}

	a.Version = event.Sequence
	a.CreatedAt = event.OccurredAt

	return nil
}

// applyAccountActivated applies the transition:
//
//	PENDING_VERIFICATION -> ACTIVE
//
// Activation is intentionally a separate event from account creation.
func (a *Account) applyAccountActivated(event events.Event) error {
	if a.AccountID == uuid.Nil {
		return ErrAccountNotFound
	}

	if a.Status != StatusPendingVerification {
		return fmt.Errorf(
			"%w: cannot activate account from %s",
			ErrInvalidStateTransition,
			a.Status,
		)
	}

	var payload AccountActivated

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode AccountActivated event: %w", err)
	}

	if err := a.validateEventAccountID(payload.AccountID); err != nil {
		return err
	}

	a.Status = StatusActive
	a.Version = event.Sequence

	return nil
}

// applyAccountSuspended applies the transition:
//
//	ACTIVE -> SUSPENDED
func (a *Account) applyAccountSuspended(event events.Event) error {
	if a.AccountID == uuid.Nil {
		return ErrAccountNotFound
	}

	if a.Status != StatusActive {
		return fmt.Errorf(
			"%w: cannot suspend account from %s",
			ErrInvalidStateTransition,
			a.Status,
		)
	}

	var payload AccountSuspended

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode AccountSuspended event: %w", err)
	}

	if err := a.validateEventAccountID(payload.AccountID); err != nil {
		return err
	}

	a.Status = StatusSuspended
	a.Version = event.Sequence

	return nil
}

// applyAccountClosed applies the transition:
//
//	ACTIVE -> CLOSED
//	SUSPENDED -> CLOSED
//
// CLOSED is terminal and cannot transition back to another state.
func (a *Account) applyAccountClosed(event events.Event) error {
	if a.AccountID == uuid.Nil {
		return ErrAccountNotFound
	}

	if a.Status != StatusActive && a.Status != StatusSuspended {
		return fmt.Errorf(
			"%w: cannot close account from %s",
			ErrInvalidStateTransition,
			a.Status,
		)
	}

	var payload AccountClosed

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode AccountClosed event: %w", err)
	}

	if err := a.validateEventAccountID(payload.AccountID); err != nil {
		return err
	}

	a.Status = StatusClosed
	a.Version = event.Sequence

	return nil
}

// applyPaymentMethodAdded applies a payment-method association.
//
// Payment methods may only be added to an ACTIVE account.
//
// We deliberately do not allow payment-method mutations against
// PENDING_VERIFICATION, SUSPENDED, or CLOSED accounts.
func (a *Account) applyPaymentMethodAdded(event events.Event) error {
	if a.AccountID == uuid.Nil {
		return ErrAccountNotFound
	}

	if a.Status == StatusClosed {
		return ErrAccountClosed
	}

	if a.Status != StatusActive {
		return fmt.Errorf(
			"%w: payment method cannot be added while account is %s",
			ErrInvalidStateTransition,
			a.Status,
		)
	}

	var payload PaymentMethodAdded

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode PaymentMethodAdded event: %w", err)
	}

	if err := a.validateEventAccountID(payload.AccountID); err != nil {
		return err
	}

	if payload.PaymentMethodID == "" {
		return fmt.Errorf(
			"%w: payment method id cannot be empty",
			ErrInvalidAccountEvent,
		)
	}

	// Prevent duplicate payment-method associations.
	for _, existingID := range a.PaymentMethods {
		if existingID == payload.PaymentMethodID {
			return fmt.Errorf(
				"%w: payment method %q already exists",
				ErrInvalidAccountEvent,
				payload.PaymentMethodID,
			)
		}
	}

	a.PaymentMethods = append(
		a.PaymentMethods,
		payload.PaymentMethodID,
	)

	a.Version = event.Sequence

	return nil
}

// validateEventAccountID makes sure an event belongs to the
// aggregate currently being rebuilt.
func (a *Account) validateEventAccountID(eventAccountID uuid.UUID) error {
	if eventAccountID == uuid.Nil {
		return fmt.Errorf(
			"%w: event contains empty account id",
			ErrInvalidAccountEvent,
		)
	}

	if eventAccountID != a.AccountID {
		return fmt.Errorf(
			"%w: aggregate=%s event=%s",
			ErrAccountIDMismatch,
			a.AccountID,
			eventAccountID,
		)
	}

	return nil
}

// CanActivate reports whether the account can transition to ACTIVE.
func (a *Account) CanActivate() bool {
	return a.Status == StatusPendingVerification
}

// CanSuspend reports whether the account can transition to SUSPENDED.
func (a *Account) CanSuspend() bool {
	return a.Status == StatusActive
}

// CanClose reports whether the account can transition to CLOSED.
func (a *Account) CanClose() bool {
	return a.Status == StatusActive || a.Status == StatusSuspended
}

// IsClosed reports whether the account has reached its terminal state.
func (a *Account) IsClosed() bool {
	return a.Status == StatusClosed
}

// Rebuild reconstructs an Account aggregate by replaying its
// event stream from the beginning.
//
// Event sourcing rule:
// The event stream is authoritative. The current Account state
// is derived by applying the events in sequence.
func Rebuild(stream []events.Event) (*Account, error) {
	account := &Account{}

	for _, event := range stream {
		if err := account.Apply(event); err != nil {
			return nil, fmt.Errorf(
				"rebuild account failed at sequence %d (%s): %w",
				event.Sequence,
				event.EventType,
				err,
			)
		}
	}

	return account, nil
}

// cloneStringMap creates a copy of a string map.
//
// This prevents the aggregate from accidentally sharing mutable
// map storage with an event payload.
func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}

	result := make(map[string]string, len(source))

	for key, value := range source {
		result[key] = value
	}

	return result
}

// cloneBoolMap creates a copy of a bool map.
func cloneBoolMap(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}

	result := make(map[string]bool, len(source))

	for key, value := range source {
		result[key] = value
	}

	return result
}
