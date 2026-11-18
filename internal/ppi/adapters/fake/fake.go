package fake

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

type FakeAdapter struct {
	webhookSecret []byte
}

const maxFakeWebhookBytes = 1 << 20

func NewFakeAdapter(webhookSecret string) *FakeAdapter {
	return &FakeAdapter{webhookSecret: []byte(webhookSecret)}
}

func (a *FakeAdapter) ProviderCode() string {
	return "fake"
}

func (a *FakeAdapter) ChargePaymentMethod(
	ctx context.Context,
	amount money.Money,
	providerCode, idempotencyKey string,
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

	if providerCode == "" {
		resBody, _ := json.Marshal(map[string]string{"error": "MISSING_PROVIDER_CODE"})
		return ppi.ChargeResult{
			Status:         ppi.ChargeStatusFailed,
			IdempotencyKey: idempotencyKey,
			FailureCode:    "MISSING_PROVIDER_CODE",
			RawResponse:    resBody,
		}, nil
	}

	providerRef := fmt.Sprintf("fake_ref_%s", uuid.NewString())

	// Hosted / mobile money checkout flow (returns PENDING with CheckoutURL)
	if strings.Contains(providerCode, "mobile") || strings.Contains(providerCode, "chapa") || providerCode == "fake" {
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

func (a *FakeAdapter) VerifyPayment(
	ctx context.Context,
	providerCode string,
	internalTxID string,
	providerTxID *string,
) (ppi.ChargeResult, error) {
	if strings.Contains(internalTxID, "error_verify") {
		return ppi.ChargeResult{}, errors.New("simulated verification error")
	}

	status := ppi.ChargeStatusSuccess
	if strings.Contains(internalTxID, "fail_verify") {
		status = ppi.ChargeStatusFailed
	} else if strings.Contains(internalTxID, "pending_verify") {
		status = ppi.ChargeStatusPending
	}

	resBody, _ := json.Marshal(map[string]string{
		"status": string(status),
		"tx_ref": internalTxID,
	})

	var pRef string
	if providerTxID != nil {
		pRef = *providerTxID
	}

	return ppi.ChargeResult{
		Status:            status,
		IdempotencyKey:    internalTxID,
		ProviderReference: pRef,
		RawResponse:       resBody,
	}, nil
}

func (a *FakeAdapter) ParseWebhook(r *http.Request) (ppi.ProviderWebhookPayload, error) {
	if r == nil || r.Body == nil {
		return ppi.ProviderWebhookPayload{}, errors.New("empty request body")
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxFakeWebhookBytes+1))
	if err != nil {
		return ppi.ProviderWebhookPayload{}, fmt.Errorf("failed to read webhook body: %w", err)
	}
	if len(body) > maxFakeWebhookBytes {
		return ppi.ProviderWebhookPayload{}, fmt.Errorf("fake webhook body exceeds %d bytes", maxFakeWebhookBytes)
	}

	// Restore request body for downstream readers
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	if len(a.webhookSecret) == 0 || !VerifyWebhookSignature(body, r.Header.Get("X-Fake-Signature"), a.webhookSecret) {
		return ppi.ProviderWebhookPayload{}, errors.New("invalid fake webhook signature")
	}

	// 1. Unmarshal provider-specific raw payload
	var evt fakeWebhookEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return ppi.ProviderWebhookPayload{}, fmt.Errorf("failed to decode fake provider webhook json: %w", err)
	}

	// 2. Map provider-specific status to system WebhookPaymentStatus
	var status ppi.WebhookPaymentStatus
	switch strings.ToLower(evt.Status) {
	case "success", "charge.success":
		status = ppi.WebhookPaymentSucceeded
	case "failed", "charge.failed":
		status = ppi.WebhookPaymentFailed
	default:
		status = ppi.WebhookPaymentPending
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

func SignWebhook(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func VerifyWebhookSignature(body []byte, signature string, secret []byte) bool {
	if len(secret) == 0 || !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(body)
	return hmac.Equal(provided, mac.Sum(nil))
}

func (a *FakeAdapter) RespondWebhook(
	w http.ResponseWriter, r *http.Request, code ppi.WebhookResponseCode, payload ppi.ProviderWebhookPayload,
) {
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
