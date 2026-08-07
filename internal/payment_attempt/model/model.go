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
	ID             uuid.UUID       `json:"id"`
	InvoiceID      int64           `json:"invoice_id"`
	AttemptNumber  int             `json:"attempt_number"`
	IdempotencyKey string          `json:"idempotency_key"`
	Provider       string          `json:"provider"`
	ProviderTxID   *string         `json:"provider_tx_id,omitempty"`
	Amount         money.Money     `json:"amount"`
	Status         Status          `json:"status"`
	RawResponse    json.RawMessage `json:"raw_response,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
