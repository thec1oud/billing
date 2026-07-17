package account

import "github.com/google/uuid"

// AccountCreated is the payload stored inside the billing event store.
type AccountCreated struct {
	AccountID uuid.UUID `json:"account_id"`
	Currency  string    `json:"currency"`
	Timezone  string    `json:"timezone"`
}

type PaymentMethodAdded struct {
	AccountID       uuid.UUID `json:"account_id"`
	PaymentMethodID string    `json:"payment_method_id"`
}
