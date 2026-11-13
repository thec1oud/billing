package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/handler"
	"github.com/thec1oud/billing/internal/shared/money"
)

type mockService struct {
	adapters       map[string]ppi.Provider
	processed      []ppi.ProviderWebhookPayload
	published      map[string]bool
	forceErrOnProc bool
}

func newMockService() *mockService {
	return &mockService{
		adapters:  make(map[string]ppi.Provider),
		published: make(map[string]bool),
	}
}

func (s *mockService) GetAdapter(providerCode string) (ppi.Provider, bool) {
	a, ok := s.adapters[strings.ToLower(providerCode)]
	return a, ok
}

func (s *mockService) ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) (bool, error) {
	if s.forceErrOnProc {
		return false, errors.New("db query error")
	}
	for _, p := range s.processed {
		if p.ProviderCode == payload.ProviderCode && p.ProviderTxID == payload.ProviderTxID {
			if s.published[p.WebhookID] {
				return true, nil
			}
			return false, nil
		}
	}
	s.processed = append(s.processed, payload)
	return false, nil
}

func (s *mockService) MarkWebhookPublished(ctx context.Context, webhookID string) error {
	s.published[webhookID] = true
	return nil
}

func (s *mockService) ChargePaymentMethod(
	ctx context.Context, providerCode string, invoiceID int64, amount money.Money, idempotencyKey string,
) (ppi.ChargeResult, error) {
	return ppi.ChargeResult{}, nil
}

func TestWebhookHandler_SuccessFlow(t *testing.T) {
	svc := newMockService()
	fakeAdapter := fake.NewFakeAdapter()
	svc.adapters[fakeAdapter.ProviderCode()] = fakeAdapter

	h := handler.NewWebhookHandler(svc)

	body := []byte(`{
		"event_id": "evt_test_100",
		"event": "charge.success",
		"tx_ref": "tx_inv_55",
		"reference": "fake_tx_888",
		"status": "success",
		"amount_minor": 10000,
		"currency": "ETB"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(body))
	req.SetPathValue("provider", "fake")
	rec := httptest.NewRecorder()

	h.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != "processed" {
		t.Errorf("expected status 'processed', got %q", resp["status"])
	}

}
func TestWebhookHandler_DuplicatePublishedEventIgnored(t *testing.T) {
	svc := newMockService()
	fakeAdapter := fake.NewFakeAdapter()
	svc.adapters[fakeAdapter.ProviderCode()] = fakeAdapter

	h := handler.NewWebhookHandler(svc)

	body := []byte(`{
		"event_id": "evt_test_dup",
		"event": "charge.success",
		"tx_ref": "tx_inv_57",
		"reference": "dup_published_111",
		"status": "success"
	}`)

	// 1. First call: Succeeds fully
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(body))
	req1.SetPathValue("provider", "fake")
	rec1 := httptest.NewRecorder()
	h.HandleWebhook(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first webhook request to succeed, got %d", rec1.Code)
	}
	if err := svc.MarkWebhookPublished(context.Background(), "evt_test_dup"); err != nil {
		t.Fatalf("mark webhook published: %v", err)
	}

	// 2. Second call: Same payload after successful publish -> ignored as duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(body))
	req2.SetPathValue("provider", "fake")
	rec2 := httptest.NewRecorder()
	h.HandleWebhook(rec2, req2)

	var resp map[string]string
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp["status"] != "ignored" || resp["reason"] != "duplicate_event" {
		t.Errorf("expected ignored status on published duplicate, got %+v", resp)
	}
}
