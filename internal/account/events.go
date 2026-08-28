package account

// AccountCreated represents the payload stored when an
// account aggregate is created.
//
// Creation starts the account in PENDING_VERIFICATION.
// Activation happens through a separate AccountActivated event.
type AccountCreated struct {
	AccountID int64 `json:"account_id"`

	ExternalID string `json:"external_id,omitempty"`

	Currency string `json:"currency"`
	Timezone string `json:"timezone"`

	Locale   string `json:"locale,omitempty"`
	NetTerms int16  `json:"net_terms,omitempty"`

	DunningProfileID *int64 `json:"dunning_profile_id,omitempty"`

	TaxIdentifiers  []byte `json:"tax_identifiers,omitempty"`
	BillingAddress  []byte `json:"billing_address,omitempty"`
	ComplianceFlags []byte `json:"compliance_flags,omitempty"`
	Metadata        []byte `json:"metadata,omitempty"`
}

// AccountActivated represents:
//
// PENDING_VERIFICATION -> ACTIVE
type AccountActivated struct {
	AccountID int64 `json:"account_id"`
}

// AccountSuspended represents:
//
// ACTIVE -> SUSPENDED
type AccountSuspended struct {
	AccountID int64  `json:"account_id"`
	Reason    string `json:"reason"`
}

// AccountClosed represents:
//
// ACTIVE -> CLOSED
// SUSPENDED -> CLOSED
type AccountClosed struct {
	AccountID int64  `json:"account_id"`
	Reason    string `json:"reason"`
}

// PaymentMethodAdded represents a payment method attached
// to an existing Account aggregate.
type PaymentMethodAdded struct {
	AccountID       int64  `json:"account_id"`
	PaymentMethodID string `json:"payment_method_id"`
}
