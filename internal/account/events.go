package account

import "github.com/google/uuid"

// AccountCreated is emitted when a billing account is created.
//
// According to the account state machine, a newly created
// account starts in PENDING_VERIFICATION.
type AccountCreated struct {
	AccountID  uuid.UUID `json:"account_id"`
	ExternalID string    `json:"external_id"`

	Currency string `json:"currency"`
	Timezone string `json:"timezone"`
	Locale   string `json:"locale"`

	NetTerms int16 `json:"net_terms"`

	DunningProfileID *int64 `json:"dunning_profile_id"`

	TaxIdentifiers  map[string]string `json:"tax_identifiers"`
	BillingAddress  map[string]string `json:"billing_address"`
	ComplianceFlags map[string]bool   `json:"compliance_flags"`
	Metadata        map[string]string `json:"metadata"`
}

// AccountActivated is emitted when an account moves
// from PENDING_VERIFICATION to ACTIVE.
type AccountActivated struct {
	AccountID uuid.UUID `json:"account_id"`
}

// AccountSuspended is emitted when an account moves
// from ACTIVE to SUSPENDED.
type AccountSuspended struct {
	AccountID uuid.UUID `json:"account_id"`
	Reason    string    `json:"reason"`
}

// AccountClosed is emitted when an account moves
// to CLOSED.
//
// CLOSED is a terminal state. Once an account is closed,
// it must not return to ACTIVE or SUSPENDED.
type AccountClosed struct {
	AccountID uuid.UUID `json:"account_id"`
	Reason    string    `json:"reason"`
}

// PaymentMethodAdded records that a payment method
// was associated with an account.
type PaymentMethodAdded struct {
	AccountID       uuid.UUID `json:"account_id"`
	PaymentMethodID string    `json:"payment_method_id"`
}
