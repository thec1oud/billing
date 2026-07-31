package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/shared/idempotency"
	"github.com/thec1oud/billing/internal/shared/money"
)

type FakeAdapter struct {
	idem *idempotency.Store
}

func NewFakeAdapter(idem *idempotency.Store) *FakeAdapter {
	return &FakeAdapter{idem: idem}
}
func (a *FakeAdapter) ChargePaymentMethod(
	ctx context.Context,
	amount money.Money,
	paymentMethodID, idempotencyKey string,
) (ppi.ChargeResult, error) {
	payloadStr := fmt.Sprintf("%d:%s:%s", amount.AmountMinor, amount.Currency, paymentMethodID)
	requestHash := idempotency.HashRequest([]byte(payloadStr))

	return idempotency.Execute(
		ctx,
		a.idem,
		idempotencyKey,
		"ppi.charge_payment_method",
		requestHash,
		func() (ppi.ChargeResult, error) {
			if strings.Contains(paymentMethodID, "fail") {
				return ppi.ChargeResult{Status: ppi.ChargeStatusFailed, FailureCode: "CARD_DECLINED"}, nil
			}
			return ppi.ChargeResult{Status: ppi.ChargeStatusSuccess, ProviderReference: uuid.NewString()}, nil
		})
}

var _ ppi.PPI = (*FakeAdapter)(nil)
