package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "PENDING"
	PaymentStatusSuccess  PaymentStatus = "SUCCESS"
	PaymentStatusFailed   PaymentStatus = "FAILED"
	PaymentStatusRefunded PaymentStatus = "REFUNDED"
)

type Payment struct {
	ID             uuid.UUID       `json:"id"`
	InvoiceID      int64           `json:"invoice_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	AttemptNumber  int             `json:"attempt_number"`
	Provider       string          `json:"provider"`
	ProviderTxID   *string         `json:"provider_tx_id,omitempty"`
	AmountMinor    int64           `json:"amount_minor"`
	Currency       string          `json:"currency"`
	Status         PaymentStatus   `json:"status"`
	RawResponse    json.RawMessage `json:"raw_response,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
