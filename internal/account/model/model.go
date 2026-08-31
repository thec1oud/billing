package model

import (
	"encoding/json"
	"errors"
	"time"
)

type Status string

const (
	StatusPendingVerification Status = "PENDING_VERIFICATION"
	StatusActive              Status = "ACTIVE"
	StatusSuspended           Status = "SUSPENDED"
	StatusClosed              Status = "CLOSED"
)

var (
	ErrNotFound               = errors.New("account not found")
	ErrInvalidStateTransition = errors.New("invalid account state transition")
	ErrClosed                 = errors.New("account is closed")
	ErrDuplicateExternalID    = errors.New("account external id already exists")
)

// Account is a database-backed account. AccountID is a PostgreSQL BIGINT identity.
type Account struct {
	AccountID        int64
	ExternalID       string
	Status           Status
	Currency         string
	Timezone         string
	Locale           string
	NetTerms         int16
	DunningProfileID *int64
	TaxIdentifiers   json.RawMessage
	BillingAddress   json.RawMessage
	ComplianceFlags  json.RawMessage
	Metadata         json.RawMessage
	PaymentMethods   []string
	CreatedAt        time.Time
}

type CreateInput struct {
	ExternalID       string
	Currency         string
	Timezone         string
	Locale           string
	NetTerms         int16
	DunningProfileID *int64
	TaxIdentifiers   json.RawMessage
	BillingAddress   json.RawMessage
	ComplianceFlags  json.RawMessage
	Metadata         json.RawMessage
}
