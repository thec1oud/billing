package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/ppi"
)

var log = logger.ForComponent("ppi_handler")

type WebhookService interface {
	GetAdapter(providerCode string) (ppi.Provider, bool)
	ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) (bool, error)
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
		log.Error("Failed to parse/verify webhook payload", slog.String("provider", providerCode), logger.Err(err))
		http.Error(w, fmt.Sprintf("Invalid webhook payload or signature: %v", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	var isPublished bool

	// 2. Delegate DB Idempotency check & Audit logging to Service layer
	if h.svc != nil {
		isPublished, err = h.svc.ProcessWebhook(ctx, payload)
		if err != nil {
			log.Error("Service failed to process webhook; returning HTTP 500", slog.String("provider", payload.ProviderCode), logger.Err(err))
			parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
			return
		}
	}

	if isPublished {
		log.Info("Duplicate published webhook event ignored", slog.String("provider", payload.ProviderCode), slog.String("provider_tx_id", payload.ProviderTxID))
	} else {
		// 3. Synchronously Publish to RabbitMQ Message Broker
		if h.broker != nil {
			routingKey := DetermineRoutingKey(payload)
			msgBytes, err := json.Marshal(payload)
			if err != nil {
				log.Error("Failed to marshal webhook payload for broker", logger.Err(err))
				parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
				return
			}

			if err := h.broker.PublishEvent(ctx, routingKey, msgBytes); err != nil {
				log.Error("Failed to publish webhook payload to broker; returning HTTP 500 for provider retry", slog.String("routingKey", routingKey), logger.Err(err))
				parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
				return
			}

			// 4. Mark as published in Service layer so subsequent retries are safely ignored as duplicate
			if h.svc != nil {
				if err := h.svc.MarkWebhookPublished(ctx, payload.WebhookID); err != nil {
					log.Warn("Failed to mark webhook as published in DB; broker publish succeeded", slog.String("webhook_id", payload.WebhookID), logger.Err(err))
				}
			}
		}
	}

	// 5. Respond HTTP 200 OK to payment provider
	responseCode := ppi.WebhookResponseProcessed
	if isPublished {
		responseCode = ppi.WebhookResponseIgnored
	}
	parser.RespondWebhook(w, r, responseCode, payload)
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
