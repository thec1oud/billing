package ppi

import (
	"context"

	"github.com/thec1oud/billing/internal/substrate/money"
)

type ChargeStatus string

const (
	ChargeStatusSuccess        ChargeStatus = "SUCCESS"
	ChargeStatusPending        ChargeStatus = "PENDING"
	ChargeStatusFailed         ChargeStatus = "FAILED"
	ChargeStatusUnsupported    ChargeStatus = "UNSUPPORTED"
	ChargeStatusRequiresAction ChargeStatus = "REQUIRES_ACTION"
)

type ChargeResult struct {
	Status            ChargeStatus
	ProviderReference string
	FailureCode       string
}

type PPI interface {
	ChargePaymentMethod(ctx context.Context, amount money.Money, paymentMethodID, idempotencyKey string) (ChargeResult, error)
}
