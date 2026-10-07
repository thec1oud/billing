package ppi

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/thec1oud/billing/internal/shared/money"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type WebhookRepository interface {
	SaveWebhook(ctx context.Context, db DBTX, payload ProviderWebhookPayload) error
	GetWebhookByID(ctx context.Context, db DBTX, webhookID string) (ProviderWebhookPayload, error)
	CheckWebhookStatus(ctx context.Context, db DBTX, providerCode, providerTxID string) (isDuplicate bool, isPublished bool, err error)
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
		providerCode, idempotencyKey string,
	) (ChargeResult, error)
	VerifyPayment(
		ctx context.Context,
		providerCode string,
		internalTxID string,
		providerTxID *string,
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

// PPI is the unified system facade interface used by domain modules (invoice, etc.)
type PPI interface {
	ChargePaymentMethod(
		ctx context.Context,
		providerCode string,
		invoiceID int64,
		amount money.Money,
		idempotencyKey string,
	) (ChargeResult, error)
}
