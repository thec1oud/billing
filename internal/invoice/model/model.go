package model

import (
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type Status string

const (
	StatusDraft         Status = "DRAFT"
	StatusOpen          Status = "OPEN"
	StatusPaid          Status = "PAID"
	StatusUncollectible Status = "UNCOLLECTIBLE"
	StatusVoid          Status = "VOID"
)

type LineItem struct {
	LineItemID     int64          `json:"line_item_id,omitempty"`
	ItemID         int64          `json:"item_id"`
	Description    string         `json:"description"`
	QuantityValue  float64        `json:"quantity_value"`
	QuantityUnit   string         `json:"quantity_unit"`
	UnitAmount     money.Money    `json:"unit_amount"`
	TotalAmount    money.Money    `json:"total_amount"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	SubscriptionID int64          `json:"subscription_id,omitempty"`
}

type Invoice struct {
	InvoiceID     int64          `json:"invoice_id"`
	AccountID     int64          `json:"account_id"`
	InvoiceNumber string         `json:"invoice_number,omitempty"`
	Status        Status         `json:"status"`
	Currency      money.Currency `json:"currency"`
	Subtotal      money.Money    `json:"subtotal"`
	Tax           money.Money    `json:"tax"`
	Discount      money.Money    `json:"discount"`
	Total         money.Money    `json:"total"`
	AmountPaid    money.Money    `json:"amount_paid"`
	AmountDue     money.Money    `json:"amount_due"`
	DueAt         *time.Time     `json:"due_at,omitempty"`
	FinalizedAt   *time.Time     `json:"finalized_at,omitempty"`
	PaidAt        *time.Time     `json:"paid_at,omitempty"`
	LineItems     []LineItem     `json:"line_items"`
}

// Request payload for POST /api/v1/invoices
type CreateDraftInput struct {
	AccountID      int64      `json:"account_id"`
	SubscriptionID int64      `json:"subscription_id,omitempty"`
	Currency       string     `json:"currency"`
	LineItems      []LineItem `json:"line_items"`
}

type CreatedPayload struct {
	AccountID      int64       `json:"account_id"`
	SubscriptionID int64       `json:"subscription_id,omitempty"`
	Currency       string      `json:"currency"`
	LineItems      []LineItem  `json:"line_items"`
	Total          money.Money `json:"total"`
}

type PaymentAttemptedPayload struct {
	PaymentMethodID string      `json:"payment_method_id"`
	Amount          money.Money `json:"amount"`
}

type FinalizedPayload struct{}

type PaymentSucceededPayload struct {
	ProviderReference string `json:"provider_reference"`
}

type PaymentFailedPayload struct {
	FailureCode string `json:"failure_code"`
}

type InvoicePaidPayload struct{}
