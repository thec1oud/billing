package invoice

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusDraft Status = "DRAFT"
	StatusOpen  Status = "OPEN"
)

type LineItem struct {
	Description string    `json:"description"`
	Amount      int64     `json:"amount"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
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
	Total          int64
}

type CreatedPayload struct {
	AccountID      uuid.UUID  `json:"account_id"`
	SubscriptionID uuid.UUID  `json:"subscription_id"`
	Currency       string     `json:"currency"`
	PeriodStart    time.Time  `json:"period_start"`
	PeriodEnd      time.Time  `json:"period_end"`
	LineItems      []LineItem `json:"line_items"`
	Total          int64      `json:"total"`
}

type FinalizedPayload struct{}

type AccountLookup interface {
	DefaultPaymentMethodID(ctx context.Context, accountID uuid.UUID) (string, error)
}

const StatusPaid Status = "PAID"

type PaymentAttemptedPayload struct {
	PaymentMethodID string `json:"payment_method_id"`
	Amount          int64  `json:"amount"`
	Currency        string `json:"currency"`
}

type PaymentSucceededPayload struct {
	ProviderReference string `json:"provider_reference"`
}

type PaymentFailedPayload struct {
	FailureCode string `json:"failure_code"`
}

type InvoicePaidPayload struct{}
