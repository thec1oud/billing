package model

import (
	"encoding/json"
	"time"

	"github.com/thec1oud/billing/internal/shared/apperr"
)

type Status string

const (
	StatusPendingVerification Status = "PENDING_VERIFICATION"
	StatusActive              Status = "ACTIVE"
	StatusSuspended           Status = "SUSPENDED"
	StatusClosed              Status = "CLOSED"
)

var (
	ErrNotFound               = apperr.NotFound("account_not_found", "account not found")
	ErrInvalidStateTransition = apperr.Conflict("invalid_state_transition", "invalid account state transition")
	ErrClosed                 = apperr.Conflict("account_closed", "account is closed")
	ErrDuplicateExternalID    = apperr.Conflict("duplicate_external_id", "account external id already exists")
)

// Account is a database-backed account. AccountID is a PostgreSQL BIGINT identity.
type Account struct {
	AccountID        int64           `json:"id"`
	ExternalID       string          `json:"external_id,omitempty"`
	Status           Status          `json:"status"`
	Currency         string          `json:"currency"`
	Timezone         string          `json:"timezone,omitempty"`
	Locale           string          `json:"locale,omitempty"`
	NetTerms         int16           `json:"net_terms"`
	DunningProfileID *int64          `json:"dunning_profile_id,omitempty"`
	TaxIdentifiers   json.RawMessage `json:"tax_identifiers,omitempty"`
	BillingAddress   json.RawMessage `json:"billing_address,omitempty"`
	ComplianceFlags  json.RawMessage `json:"compliance_flags,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
	PaymentMethods   []string        `json:"payment_methods,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

type CreateInput struct {
	ExternalID       string          `json:"external_id,omitempty"`
	Currency         string          `json:"currency"`
	Timezone         string          `json:"timezone,omitempty"`
	Locale           string          `json:"locale,omitempty"`
	NetTerms         int16           `json:"net_terms,omitempty"`
	DunningProfileID *int64          `json:"dunning_profile_id,omitempty"`
	TaxIdentifiers   json.RawMessage `json:"tax_identifiers,omitempty"`
	BillingAddress   json.RawMessage `json:"billing_address,omitempty"`
	ComplianceFlags  json.RawMessage `json:"compliance_flags,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
}
