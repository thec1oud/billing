package adapters

import (
	"context"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/thec1oud/billing/internal/ppi"
)

type FakeAdapter struct {
	mu    sync.Mutex
	store map[string]ppi.ChargeResult
}

func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{store: make(map[string]ppi.ChargeResult)}
}

func (a *FakeAdapter) ChargePaymentMethod(ctx context.Context, amount int64, currency, paymentMethodID, idempotencyKey string) (ppi.ChargeResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if result, ok := a.store[idempotencyKey]; ok {
		return result, nil
	}

	var result ppi.ChargeResult
	if strings.Contains(paymentMethodID, "fail") {
		result = ppi.ChargeResult{Status: ppi.ChargeStatusFailed, FailureCode: "CARD_DECLINED"}
	} else {
		result = ppi.ChargeResult{Status: ppi.ChargeStatusSuccess, ProviderReference: uuid.NewString()}
	}

	a.store[idempotencyKey] = result
	return result, nil
}
