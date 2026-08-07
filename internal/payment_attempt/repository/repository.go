package repository

import (
	"context"

	"github.com/thec1oud/billing/internal/payment_attempt/model"
)

type PaymentRepository interface {
	// CreateAttempt inserts a new PENDING payment.
	// Fails if another PENDING attempt exists for the same invoice (enforced via partial index or lock).
	CreateAttempt(ctx context.Context, payment *model.Payment) error

	// GetLatestAttemptNumber returns the highest attempt count for an invoice to calculate next attempt_number.
	GetLatestAttemptNumber(ctx context.Context, invoiceID int64) (int, error)

	// GetByProviderTxID finds a payment record when handling external gateway webhooks.
	GetByProviderTxID(ctx context.Context, provider, providerTxID string) (*model.Payment, error)

	// UpdateStatus transitions PENDING to SUCCESS/FAILED and attaches raw provider payload.
	UpdateStatus(ctx context.Context, id string, status model.PaymentStatus, providerTxID *string, rawResponse []byte) error
}
