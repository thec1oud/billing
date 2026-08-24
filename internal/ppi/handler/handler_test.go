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
	"github.com/thec1oud/billing/internal/ppi/service"
)

type mockBroker struct {
	publishedKeys []string
	publishedMsgs [][]byte
	forceErr      bool
}

func (m *mockBroker) InitTopology(ctx context.Context) error {
	return nil
}

func (m *mockBroker) PublishEvent(ctx context.Context, routingKey string, body []byte) error {
	if m.forceErr {
		return errors.New("broker connection failure")
	}
	m.publishedKeys = append(m.publishedKeys, routingKey)
	m.publishedMsgs = append(m.publishedMsgs, body)
	return nil
}

func (m *mockBroker) RegisterConsumerGroup(ctx context.Context, queueName string, routingKeys []string, h func(ctx context.Context, msg []byte) error) error {
	return nil
}

func (m *mockBroker) Close() error {
	return nil
}

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

func (s *mockService) ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) error {
	if s.forceErrOnProc {
		return errors.New("db query error")
	}
	for _, p := range s.processed {
		if p.ProviderCode == payload.ProviderCode && p.ProviderTxID == payload.ProviderTxID && s.published[p.WebhookID] {
			return service.ErrDuplicateWebhook
		}
	}
	s.processed = append(s.processed, payload)
	return nil
}

func (s *mockService) MarkWebhookPublished(ctx context.Context, webhookID string) error {
	s.published[webhookID] = true
	return nil
}

func TestWebhookHandler_SuccessFlow(t *testing.T) {
	svc := newMockService()
	fakeAdapter := fake.NewFakeAdapter()
	svc.adapters[fakeAdapter.ProviderCode()] = fakeAdapter

	broker := &mockBroker{}
	h := handler.NewWebhookHandler(svc, broker)

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

	h.ServeHTTP(rec, req)

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

	if len(broker.publishedKeys) != 1 {
		t.Fatalf("expected 1 published event to broker, got %d", len(broker.publishedKeys))
	}

	if broker.publishedKeys[0] != "ppi.webhook.payment.succeeded" {
		t.Errorf("expected routing key 'ppi.webhook.payment.succeeded', got %q", broker.publishedKeys[0])
	}
}

func TestWebhookHandler_BrokerPublishFailureRetriesOnNextRequest(t *testing.T) {
	svc := newMockService()
	fakeAdapter := fake.NewFakeAdapter()
	svc.adapters[fakeAdapter.ProviderCode()] = fakeAdapter

	broker := &mockBroker{}
	h := handler.NewWebhookHandler(svc, broker)

	body := []byte(`{
		"event_id": "evt_test_retry",
		"event": "charge.success",
		"tx_ref": "tx_inv_56",
		"reference": "ref_retry_999",
		"status": "success"
	}`)

	// 1. First call: Broker fails
	broker.forceErr = true
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(body))
	req1.SetPathValue("provider", "fake")
	rec1 := httptest.NewRecorder()

	h.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("expected HTTP 500 when broker publish fails, got %d", rec1.Code)
	}

	// 2. Provider retries 5 seconds later: Broker is recovered
	broker.forceErr = false
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(body))
	req2.SetPathValue("provider", "fake")
	rec2 := httptest.NewRecorder()

	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 on retry after broker recovery, got %d", rec2.Code)
	}

	if len(broker.publishedKeys) != 1 {
		t.Fatalf("expected broker to receive 1 successful publish on retry, got %d", len(broker.publishedKeys))
	}
}

func TestWebhookHandler_DuplicatePublishedEventIgnored(t *testing.T) {
	svc := newMockService()
	fakeAdapter := fake.NewFakeAdapter()
	svc.adapters[fakeAdapter.ProviderCode()] = fakeAdapter

	broker := &mockBroker{}
	h := handler.NewWebhookHandler(svc, broker)

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
	h.ServeHTTP(rec1, req1)

	// 2. Second call: Same payload after successful publish -> ignored as duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(body))
	req2.SetPathValue("provider", "fake")
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)

	var resp map[string]string
	_ = json.Unmarshal(rec2.Body.Bytes(), &resp)
	if resp["status"] != "ignored" || resp["reason"] != "duplicate_event" {
		t.Errorf("expected ignored status on published duplicate, got %+v", resp)
	}
}
