package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/ppi"
)

type WebhookHandler struct {
	parsers map[string]ppi.WebhookParser
	repo    ppi.WebhookRepository
	db      ppi.DBTX
	broker  messaging.Broker
}

func NewWebhookHandler(
	repo ppi.WebhookRepository,
	db ppi.DBTX,
	broker messaging.Broker,
) *WebhookHandler {
	return &WebhookHandler{
		parsers: make(map[string]ppi.WebhookParser),
		repo:    repo,
		db:      db,
		broker:  broker,
	}
}

func (h *WebhookHandler) RegisterAdapter(parser ppi.WebhookParser) {
	if parser != nil {
		h.parsers[strings.ToLower(parser.ProviderCode())] = parser
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

	parser, exists := h.parsers[strings.ToLower(providerCode)]
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

	// 2. State-Aware Idempotency Check: Ignored ONLY if event was already published successfully
	if h.repo != nil {
		isDup, err := h.repo.IsDuplicate(ctx, h.db, payload.ProviderCode, payload.ProviderTxID)
		if err != nil {
			slog.Error("Idempotency check query failed; rejecting process to prevent duplicate execution", "provider", payload.ProviderCode, "err", err)
			parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
			return
		}

		if isDup {
			slog.Info("Duplicate published webhook event ignored", "provider", payload.ProviderCode, "provider_tx_id", payload.ProviderTxID)
			parser.RespondWebhook(w, r, ppi.WebhookResponseIgnored, payload)
			return
		}

		// 3. Persist Webhook Audit Record in DB (published_at = NULL initially)
		if err := h.repo.SaveWebhook(ctx, h.db, payload); err != nil {
			slog.Error("Failed to persist webhook audit record to DB", "provider", payload.ProviderCode, "err", err)
			parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
			return
		}
	}

	// 4. Synchronously Publish to RabbitMQ Message Broker
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

		// 5. Mark as published in DB so subsequent retries are safely ignored as duplicate
		if h.repo != nil {
			if err := h.repo.MarkPublished(ctx, h.db, payload.WebhookID); err != nil {
				slog.Warn("Failed to mark webhook as published in DB; broker publish succeeded", "webhook_id", payload.WebhookID, "err", err)
			}
		}
	}

	// 6. Respond HTTP 200 OK to payment provider
	parser.RespondWebhook(w, r, ppi.WebhookResponseProcessed, payload)
}

func extractProviderCode(r *http.Request) string {
	if code := r.PathValue("provider"); code != "" {
		return strings.ToLower(code)
	}
	return strings.ToLower(r.URL.Query().Get("provider"))
}

func DetermineRoutingKey(p ppi.ProviderWebhookPayload) string {
	statusStr := strings.ToLower(string(p.Status))
	if statusStr == "" {
		statusStr = "pending"
	}
	return fmt.Sprintf("ppi.webhook.payment.%s", statusStr)
}
