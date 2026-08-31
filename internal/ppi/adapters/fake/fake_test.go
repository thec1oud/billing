package fake_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/shared/money"
)

func newTestAdapter() *fake.FakeAdapter {
	return fake.NewFakeAdapter()
}

func TestChargePaymentMethod_Success(t *testing.T) {
	adapter := newTestAdapter()
	ctx := context.Background()
	amount, _ := money.New(1000, "USD")

	result, err := adapter.ChargePaymentMethod(ctx, amount, "card", "tx_inv_100")
	if err != nil {
		t.Fatalf("unexpected error on charge call: %v", err)
	}

	if result.Status != ppi.ChargeStatusSuccess {
		t.Errorf("expected SUCCESS status, got %q", result.Status)
	}
	if result.IdempotencyKey != "tx_inv_100" {
		t.Errorf("expected IdempotencyKey 'tx_inv_100', got %q", result.IdempotencyKey)
	}
	if result.ProviderReference == "" {
		t.Error("expected non-empty provider_reference")
	}
	if len(result.RawResponse) == 0 {
		t.Error("expected non-empty RawResponse")
	}
}

func TestChargePaymentMethod_PendingWithCheckoutURL(t *testing.T) {
	adapter := newTestAdapter()
	ctx := context.Background()
	amount, _ := money.New(5000, "ETB")

	result, err := adapter.ChargePaymentMethod(ctx, amount, "fake", "tx_inv_101")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != ppi.ChargeStatusPending {
		t.Errorf("expected PENDING status for mobile payment, got %q", result.Status)
	}
	if result.IdempotencyKey != "tx_inv_101" {
		t.Errorf("expected IdempotencyKey 'tx_inv_101', got %q", result.IdempotencyKey)
	}
	if result.CheckoutURL == "" {
		t.Error("expected non-empty CheckoutURL for pending payment")
	}
}

func TestParseWebhook_ProviderSpecificJSON(t *testing.T) {
	adapter := newTestAdapter()

	body := []byte(`{
		"event_id": "evt_9999",
		"event": "charge.success",
		"tx_ref": "tx_invoice_100",
		"reference": "fake_ref_999",
		"status": "success",
		"amount_minor": 5000,
		"currency": "ETB"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(body))

	payload, err := adapter.ParseWebhook(req)
	if err != nil {
		t.Fatalf("unexpected error parsing webhook: %v", err)
	}

	if payload.ProviderCode != "fake" {
		t.Errorf("expected provider code 'fake', got %q", payload.ProviderCode)
	}
	if payload.InternalTxID != "tx_invoice_100" {
		t.Errorf("expected InternalTxID 'tx_invoice_100', got %q", payload.InternalTxID)
	}
	if payload.ProviderTxID != "fake_ref_999" {
		t.Errorf("expected ProviderTxID 'fake_ref_999', got %q", payload.ProviderTxID)
	}
	if payload.Status != ppi.WebhookPaymentSucceeded {
		t.Errorf("expected SUCCESS status, got %q", payload.Status)
	}
}
