package ppi

import (
	"context"
	"net/http"

	"github.com/thec1oud/billing/internal/shared/money"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag interface{ RowsAffected() int64 }, err error)
	QueryRow(ctx context.Context, sql string, args ...any) interface{ Scan(dest ...any) error }
}

type WebhookRepository interface {
	SaveWebhook(ctx context.Context, db DBTX, payload ProviderWebhookPayload) error
	GetWebhookByID(ctx context.Context, db DBTX, webhookID string) (ProviderWebhookPayload, error)
	IsDuplicate(ctx context.Context, db DBTX, providerCode, providerTxID string) (bool, error)
	MarkPublished(ctx context.Context, db DBTX, webhookID string) error
}

type WebhookResponseCode string

const (
	WebhookResponseProcessed WebhookResponseCode = "PROCESSED"
	WebhookResponseIgnored   WebhookResponseCode = "IGNORED"
	WebhookResponseError     WebhookResponseCode = "ERROR"
)

type ChargeProvider interface {
	ChargePaymentMethod(
		ctx context.Context,
		amount money.Money,
		paymentMethodID, idempotencyKey string,
	) (ChargeResult, error)
}

type WebhookParser interface {
	ProviderCode() string
	ParseWebhook(r *http.Request) (ProviderWebhookPayload, error)
	RespondWebhook(w http.ResponseWriter, r *http.Request, code WebhookResponseCode, payload ProviderWebhookPayload)
}

type Provider interface {
	ChargeProvider
	WebhookParser
}

// Backward compatibility alias for PPI interface
type PPI interface {
	ChargeProvider
}
