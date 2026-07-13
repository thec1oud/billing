package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/idempotency"
	"github.com/thec1oud/billing/internal/ppi"
)

type FakeAdapter struct {
	idem *idempotency.Store
}

func NewFakeAdapter(idem *idempotency.Store) *FakeAdapter {
	return &FakeAdapter{idem: idem}
}

func (a *FakeAdapter) ChargePaymentMethod(ctx context.Context, amount int64, currency, paymentMethodID, idempotencyKey string) (ppi.ChargeResult, error) {
	requestHash := idempotency.HashRequest([]byte(fmt.Sprintf("%d:%s:%s", amount, currency, paymentMethodID)))

	decision, err := a.idem.CheckOrReserve(ctx, idempotencyKey, "ppi.charge_payment_method", requestHash)
	if err != nil {
		return ppi.ChargeResult{}, fmt.Errorf("idempotency check: %w", err)
	}
	if !decision.ShouldProceed() {
		var cached ppi.ChargeResult
		if err := json.Unmarshal(decision.CachedResponse, &cached); err != nil {
			return ppi.ChargeResult{}, fmt.Errorf("unmarshal cached charge result: %w", err)
		}
		return cached, nil
	}

	var result ppi.ChargeResult
	if strings.Contains(paymentMethodID, "fail") {
		result = ppi.ChargeResult{Status: ppi.ChargeStatusFailed, FailureCode: "CARD_DECLINED"}
	} else {
		result = ppi.ChargeResult{Status: ppi.ChargeStatusSuccess, ProviderReference: uuid.NewString()}
	}

	if err := a.idem.StoreResponse(ctx, *decision.ProceedToken, result); err != nil {
		return ppi.ChargeResult{}, fmt.Errorf("store idempotent response: %w", err)
	}
	return result, nil
}

var _ ppi.PPI = (*FakeAdapter)(nil)
