package invoice

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/thec1oud/billing/internal/substrate/money"
)

type Status string

const (
	StatusDraft Status = "DRAFT"
	StatusOpen  Status = "OPEN"
)

type LineItem struct {
	Description string      `json:"description"`
	Amount      money.Money `json:"amount"`
	PeriodStart time.Time   `json:"period_start"`
	PeriodEnd   time.Time   `json:"period_end"`
}

type Invoice struct {
	InvoiceID      uuid.UUID
	AccountID      uuid.UUID
	SubscriptionID uuid.UUID
	Status         Status
	Currency       string
	PeriodStart    time.Time
	PeriodEnd      time.Time
	LineItems      []LineItem
	Total          money.Money
}

type CreatedPayload struct {
	AccountID      uuid.UUID   `json:"account_id"`
	SubscriptionID uuid.UUID   `json:"subscription_id"`
	Currency       string      `json:"currency"`
	PeriodStart    time.Time   `json:"period_start"`
	PeriodEnd      time.Time   `json:"period_end"`
	LineItems      []LineItem  `json:"line_items"`
	Total          money.Money `json:"total"`
}

type PaymentAttemptedPayload struct {
	PaymentMethodID string      `json:"payment_method_id"`
	Amount          money.Money `json:"amount"`
}

type FinalizedPayload struct{}

type AccountLookup interface {
	DefaultPaymentMethodID(ctx context.Context, accountID uuid.UUID) (string, error)
}

const StatusPaid Status = "PAID"

type PaymentSucceededPayload struct {
	ProviderReference string `json:"provider_reference"`
}

type PaymentFailedPayload struct {
	FailureCode string `json:"failure_code"`
}

type InvoicePaidPayload struct{}
