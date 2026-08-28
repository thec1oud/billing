package account

// AccountCreated is the payload stored inside the billing event store.
type AccountCreated struct {
	AccountID int64  `json:"account_id"`
	Currency  string `json:"currency"`
	Timezone  string `json:"timezone"`
}

type PaymentMethodAdded struct {
	AccountID       int64  `json:"account_id"`
	PaymentMethodID string `json:"payment_method_id"`
}
