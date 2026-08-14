package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/service"
)

type WebhookService interface {
	GetAdapter(providerCode string) (ppi.Provider, bool)
	ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) error
	MarkWebhookPublished(ctx context.Context, webhookID string) error
}

type WebhookHandler struct {
	svc    WebhookService
	broker messaging.Broker
}

func NewWebhookHandler(
	svc WebhookService,
	broker messaging.Broker,
) *WebhookHandler {
	return &WebhookHandler{
		svc:    svc,
		broker: broker,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	providerCode := extractProviderCode(r)
	if providerCode == "" {
		http.Error(w, "Missing provider code in URL path or parameter", http.StatusBadRequest)
		return
	}

	parser, exists := h.svc.GetAdapter(providerCode)
	if !exists {
		http.Error(w, fmt.Sprintf("Unsupported payment provider: %s", providerCode), http.StatusNotFound)
		return
	}

	// 1. Parse and verify webhook signature using provider adapter
	payload, err := parser.ParseWebhook(r)
	if err != nil {
		slog.Error("Failed to parse/verify webhook payload", "provider", providerCode, "err", err)
		http.Error(w, fmt.Sprintf("Invalid webhook payload or signature: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 2. Delegate DB Idempotency check & Audit logging to Service layer
	if h.svc != nil {
		if err := h.svc.ProcessWebhook(ctx, payload); err != nil {
			if errors.Is(err, service.ErrDuplicateWebhook) {
				slog.Info("Duplicate published webhook event ignored", "provider", payload.ProviderCode, "provider_tx_id", payload.ProviderTxID)
				parser.RespondWebhook(w, r, ppi.WebhookResponseIgnored, payload)
				return
			}

			slog.Error("Service failed to process webhook; returning HTTP 500", "provider", payload.ProviderCode, "err", err)
			parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
			return
		}
	}

	// 3. Synchronously Publish to RabbitMQ Message Broker
	if h.broker != nil {
		routingKey := DetermineRoutingKey(payload)
		msgBytes, err := json.Marshal(payload)
		if err != nil {
			slog.Error("Failed to marshal webhook payload for broker", "err", err)
			parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
			return
		}

		if err := h.broker.PublishEvent(ctx, routingKey, msgBytes); err != nil {
			slog.Error("Failed to publish webhook payload to broker; returning HTTP 500 for provider retry", "routingKey", routingKey, "err", err)
			parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
			return
		}

		// 4. Mark as published in Service layer so subsequent retries are safely ignored as duplicate
		if h.svc != nil {
			if err := h.svc.MarkWebhookPublished(ctx, payload.WebhookID); err != nil {
				slog.Warn("Failed to mark webhook as published in DB; broker publish succeeded", "webhook_id", payload.WebhookID, "err", err)
			}
		}
	}

	// 5. Respond HTTP 200 OK to payment provider
	parser.RespondWebhook(w, r, ppi.WebhookResponseProcessed, payload)
}

func extractProviderCode(r *http.Request) string {
	if code := r.PathValue("provider"); code != "" {
		return strings.ToLower(code)
	}
	return strings.ToLower(r.URL.Query().Get("provider"))
}

func DetermineRoutingKey(p ppi.ProviderWebhookPayload) string {
	switch p.Status {
	case ppi.WebhookPaymentSucceeded:
		return "ppi.webhook.payment.succeeded"
	case ppi.WebhookPaymentFailed:
		return "ppi.webhook.payment.failed"
	default:
		return "ppi.webhook.payment.pending"
	}
}
