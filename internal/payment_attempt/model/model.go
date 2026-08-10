package model

import (
	"encoding/json"
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusSuccess  Status = "SUCCESS"
	StatusFailed   Status = "FAILED"
	StatusRefunded Status = "REFUNDED"
)

type PaymentAttempt struct {
	AttemptID    int64
	InvoiceID    int64
	ProviderCode string
	InternalTxID string
	ProviderTxID *string
	Amount       money.Money
	Status       Status
	RawResponse  json.RawMessage
	RawRequest   json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreatePaymentAttemptInput struct {
	InvoiceID    int64
	ProviderCode string
	InternalTxID string
	Amount       money.Money
	RawRequest   json.RawMessage
}

type UpdatePaymentAttemptResultInput struct {
	AttemptID    int64
	Status       Status
	ProviderTxID *string
	RawResponse  json.RawMessage
}
