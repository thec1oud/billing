package ppi

import (
	"context"
	"net/http"

	"github.com/thec1oud/billing/internal/shared/money"
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
}

type Provider interface {
	ChargeProvider
	WebhookParser
}

// Backward compatibility alias for PPI interface
type PPI interface {
	ChargeProvider
}
