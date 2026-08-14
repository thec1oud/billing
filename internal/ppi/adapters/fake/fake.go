package fake

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/shared/money"
)

type fakeWebhookEvent struct {
	EventID       string `json:"event_id"`
	Event         string `json:"event"`          // e.g. "charge.success", "charge.failed"
	TxRef         string `json:"tx_ref"`         // internal transaction reference (internal_tx_id)
	Reference     string `json:"reference"`      // provider transaction reference (provider_tx_id)
	Status        string `json:"status"`         // e.g. "success", "failed", "pending"
	AmountMinor   int64  `json:"amount_minor"`   // minor currency units
	Currency      string `json:"currency"`       // e.g. "ETB", "USD"
	FailureReason string `json:"failure_reason"` // optional failure reason
	Timestamp     int64  `json:"timestamp"`      // unix timestamp
}

type FakeAdapter struct{}

func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{}
}

func (a *FakeAdapter) ProviderCode() string {
	return "fake"
}

func (a *FakeAdapter) ChargePaymentMethod(
	ctx context.Context,
	amount money.Money,
	paymentMethodID, idempotencyKey string,
) (ppi.ChargeResult, error) {
	if amount.AmountMinor <= 0 {
		resBody, _ := json.Marshal(map[string]string{"error": "INVALID_AMOUNT"})
		return ppi.ChargeResult{
			Status:         ppi.ChargeStatusFailed,
			IdempotencyKey: idempotencyKey,
			FailureCode:    "INVALID_AMOUNT",
			RawResponse:    resBody,
		}, nil
	}

	if paymentMethodID == "" {
		resBody, _ := json.Marshal(map[string]string{"error": "MISSING_PAYMENT_METHOD"})
		return ppi.ChargeResult{
			Status:         ppi.ChargeStatusFailed,
			IdempotencyKey: idempotencyKey,
			FailureCode:    "MISSING_PAYMENT_METHOD",
			RawResponse:    resBody,
		}, nil
	}

	providerRef := fmt.Sprintf("fake_ref_%s", uuid.NewString())

	// Hosted / mobile money checkout flow (returns PENDING with CheckoutURL)
	if strings.Contains(paymentMethodID, "mobile") || strings.Contains(paymentMethodID, "chapa") {
		checkoutURL := fmt.Sprintf("https://checkout.fake-provider.com/pay/%s", providerRef)
		resBody, _ := json.Marshal(map[string]string{
			"status":       "PENDING",
			"tx_ref":       idempotencyKey,
			"reference":    providerRef,
			"checkout_url": checkoutURL,
		})
		return ppi.ChargeResult{
			Status:            ppi.ChargeStatusPending,
			IdempotencyKey:    idempotencyKey,
			ProviderReference: providerRef,
			CheckoutURL:       checkoutURL,
			RawResponse:       resBody,
		}, nil
	}

	resBody, _ := json.Marshal(map[string]string{
		"status":    "SUCCESS",
		"tx_ref":    idempotencyKey,
		"reference": providerRef,
	})

	return ppi.ChargeResult{
		Status:            ppi.ChargeStatusSuccess,
		IdempotencyKey:    idempotencyKey,
		ProviderReference: providerRef,
		RawResponse:       resBody,
	}, nil
}

func (a *FakeAdapter) ParseWebhook(r *http.Request) (ppi.ProviderWebhookPayload, error) {
	if r == nil || r.Body == nil {
		return ppi.ProviderWebhookPayload{}, errors.New("empty request body")
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return ppi.ProviderWebhookPayload{}, fmt.Errorf("failed to read webhook body: %w", err)
	}

	// Restore request body for downstream readers
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// 1. Unmarshal provider-specific raw payload
	var evt fakeWebhookEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return ppi.ProviderWebhookPayload{}, fmt.Errorf("failed to decode fake provider webhook json: %w", err)
	}

	// 2. Map provider-specific status to system ChargeStatus
	var status ppi.ChargeStatus
	switch strings.ToLower(evt.Status) {
	case "success", "charge.success":
		status = ppi.ChargeStatusSuccess
	case "failed", "charge.failed":
		status = ppi.ChargeStatusFailed
	default:
		status = ppi.ChargeStatusPending
	}

	// 3. Map currency & amount
	parsedMoney, _ := money.New(evt.AmountMinor, evt.Currency)

	// 4. Construct normalized system ProviderWebhookPayload
	webhookID := evt.EventID
	if webhookID == "" {
		webhookID = uuid.NewString()
	}

	occurredAt := time.Now().UTC()
	if evt.Timestamp > 0 {
		occurredAt = time.Unix(evt.Timestamp, 0).UTC()
	}

	payload := ppi.ProviderWebhookPayload{
		WebhookID:    webhookID,
		ProviderCode: a.ProviderCode(),
		EventType:    evt.Event,
		InternalTxID: evt.TxRef,
		ProviderTxID: evt.Reference,
		Status:       status,
		Amount:       parsedMoney,
		FailureCode:  evt.FailureReason,
		RawPayload:   json.RawMessage(body),
		OccurredAt:   occurredAt,
	}

	return payload, nil
}

func (a *FakeAdapter) RespondWebhook(w http.ResponseWriter, r *http.Request, code ppi.WebhookResponseCode, payload ppi.ProviderWebhookPayload) {
	w.Header().Set("Content-Type", "application/json")
	switch code {
	case ppi.WebhookResponseIgnored:
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ignored",
			"reason": "duplicate_event",
		})
	case ppi.WebhookResponseError:
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
		})
	default:
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":     "processed",
			"webhook_id": payload.WebhookID,
		})
	}
}

var _ ppi.Provider = (*FakeAdapter)(nil)
