package ppi

import "context"

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
	ChargePaymentMethod(ctx context.Context, amount int64, currency, paymentMethodID, idempotencyKey string) (ChargeResult, error)
}
