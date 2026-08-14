package ppi

import (
	"encoding/json"
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type ChargeStatus string

const (
	ChargeStatusSuccess ChargeStatus = "SUCCESS"
	ChargeStatusPending ChargeStatus = "PENDING"
	ChargeStatusFailed  ChargeStatus = "FAILED"
)

type ChargeResult struct {
	Status            ChargeStatus
	ProviderReference string
	FailureCode       string
}

type ProviderWebhookPayload struct {
	WebhookID    string          `json:"webhook_id"`
	ProviderCode string          `json:"provider_code"`
	EventType    string          `json:"event_type"`
	InternalTxID string          `json:"internal_tx_id"`
	ProviderTxID string          `json:"provider_tx_id"`
	Status       ChargeStatus    `json:"status"`
	Amount       money.Money     `json:"amount"`
	FailureCode  string          `json:"failure_code,omitempty"`
	RawPayload   json.RawMessage `json:"raw_payload"`
	OccurredAt   time.Time       `json:"occurred_at"`
}
