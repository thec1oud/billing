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

type WebhookPaymentStatus string

const (
	WebhookPaymentSucceeded WebhookPaymentStatus = "SUCCEEDED"
	WebhookPaymentFailed    WebhookPaymentStatus = "FAILED"
	WebhookPaymentPending   WebhookPaymentStatus = "PENDING"
)

type ChargeResult struct {
	Status            ChargeStatus    `json:"status"`
	IdempotencyKey    string          `json:"idempotency_key"`    // internal_tx_id from payment attempt
	ProviderReference string          `json:"provider_reference"` // provider_tx_id from payment provider
	CheckoutURL       string          `json:"checkout_url,omitempty"`
	FailureCode       string          `json:"failure_code,omitempty"`
	RawResponse       json.RawMessage `json:"raw_response,omitempty"`
}

type ProviderWebhookPayload struct {
	WebhookID    string               `json:"webhook_id"`
	ProviderCode string               `json:"provider_code"`
	EventType    string               `json:"event_type"`
	InternalTxID string               `json:"internal_tx_id"`
	ProviderTxID string               `json:"provider_tx_id"`
	Status       WebhookPaymentStatus `json:"status"`
	Amount       money.Money          `json:"amount"`
	FailureCode  string               `json:"failure_code,omitempty"`
	RawPayload   json.RawMessage      `json:"raw_payload"`
	OccurredAt   time.Time            `json:"occurred_at"`
}
