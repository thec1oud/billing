package adapters

import (
	"context"
	"errors"
	"testing"

	"github.com/thec1oud/billing/internal/idempotency"
)

func newTestAdapter() *FakeAdapter {
	return NewFakeAdapter(idempotency.NewStore())
}

func TestChargePaymentMethod_IdempotentOnRetry(t *testing.T) {
	adapter := newTestAdapter()
	ctx := context.Background()

	first, err := adapter.ChargePaymentMethod(ctx, 1000, "USD", "pm_123", "idem-key-1")
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	second, err := adapter.ChargePaymentMethod(ctx, 1000, "USD", "pm_123", "idem-key-1")
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if first.ProviderReference != second.ProviderReference {
		t.Errorf("expected same provider_reference on retry, got %q then %q", first.ProviderReference, second.ProviderReference)
	}
	if first.Status != second.Status {
		t.Errorf("expected same status on retry, got %q then %q", first.Status, second.Status)
	}
}

func TestChargePaymentMethod_DifferentKeysChargeIndependently(t *testing.T) {
	adapter := newTestAdapter()
	ctx := context.Background()

	first, _ := adapter.ChargePaymentMethod(ctx, 1000, "USD", "pm_123", "idem-key-1")
	second, _ := adapter.ChargePaymentMethod(ctx, 1000, "USD", "pm_123", "idem-key-2")

	if first.ProviderReference == second.ProviderReference {
		t.Errorf("expected distinct provider_reference for distinct idempotency keys, got same %q for both", first.ProviderReference)
	}
}

func TestChargePaymentMethod_FailPaymentMethodReturnsFailed(t *testing.T) {
	adapter := newTestAdapter()
	ctx := context.Background()

	result, err := adapter.ChargePaymentMethod(ctx, 1000, "USD", "pm_fail_card", "idem-key-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "FAILED" {
		t.Errorf("expected FAILED status for pm containing 'fail', got %q", result.Status)
	}
	if result.FailureCode == "" {
		t.Error("expected non-empty failure_code on FAILED result")
	}
}

func TestChargePaymentMethod_SameKeyDifferentParamsConflicts(t *testing.T) {
	adapter := newTestAdapter()
	ctx := context.Background()

	if _, err := adapter.ChargePaymentMethod(ctx, 1000, "USD", "pm_123", "idem-key-1"); err != nil {
		t.Fatalf("first call: %v", err)
	}

	_, err := adapter.ChargePaymentMethod(ctx, 2000, "USD", "pm_123", "idem-key-1")
	if !errors.Is(err, idempotency.ErrConflict) {
		t.Fatalf("expected ErrConflict when reusing key with different amount, got %v", err)
	}
}
