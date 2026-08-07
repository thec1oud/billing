package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

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
	ID                  uuid.UUID
	InvoiceID           int64
	PaymentMethodID     *int64
	PaymentProviderCode string
	AttemptNumber       int
	IdempotencyKey      string
	ProviderTxID        *string
	Amount              money.Money
	Status              Status
	RawResponse         json.RawMessage
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreatePaymentAttemptInput struct {
	InvoiceID       int64
	PaymentMethodID int64
	IdempotencyKey  string
	Amount          money.Money
	ProviderTxID    *string
	RawResponse     json.RawMessage
}

type UpdatePaymentAttemptStatusInput struct {
	ID           uuid.UUID
	Status       Status
	ProviderTxID *string
	RawResponse  json.RawMessage
}
