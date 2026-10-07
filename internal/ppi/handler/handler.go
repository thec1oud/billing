package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/shared/money"
)


type PPIService interface {
	GetAdapter(providerCode string) (ppi.Provider, bool)
	ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) (bool, error)
	MarkWebhookPublished(ctx context.Context, webhookID string) error
	ChargePaymentMethod(ctx context.Context, providerCode string, invoiceID int64, amount money.Money, idempotencyKey string) (ppi.ChargeResult, error)
}

type WebhookService = PPIService

type PPIHandler struct {
	svc    PPIService
	broker messaging.Broker
	log    *slog.Logger
}

type WebhookHandler = PPIHandler

func NewPPIHandler(
	svc PPIService,
	broker messaging.Broker,
) *PPIHandler {
	return &PPIHandler{
		svc:    svc,
		broker: broker,
		log:    logger.ForComponent("ppi_handler"),
	}
}

func NewWebhookHandler(
	svc PPIService,
	broker messaging.Broker,
) *PPIHandler {
	return NewPPIHandler(svc, broker)
}

type AttemptPaymentRequest struct {
	ProviderCode string `json:"provider"`
	InvoiceID    int64  `json:"invoice_id"`
	AmountMinor  int64  `json:"amount_minor"`
	Currency     string `json:"currency"`
}

func (h *PPIHandler) HandleAttemptPayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req AttemptPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("Failed to decode checkout payload", logger.Err(err))
		response.Write(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	amt := money.Money{
		AmountMinor: req.AmountMinor,
		Currency:    money.Currency(req.Currency),
	}

	idempotencyKey := fmt.Sprintf("tx_%d_%d", req.InvoiceID, time.Now().UnixNano())

	result, err := h.svc.ChargePaymentMethod(ctx, req.ProviderCode, req.InvoiceID, amt, idempotencyKey)
	if err != nil {
		h.log.Error("Failed to charge payment method", logger.Err(err))
		response.Write(w, http.StatusInternalServerError, "Failed to initiate checkout")
		return
	}

	response.Write(w, http.StatusOK, result)
}

func (h *PPIHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	providerCode := extractProviderCode(r)
	if providerCode == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "MISSING_PROVIDER_CODE",
			Message: "Missing provider code in URL path or parameter",
		})
		return
	}

	parser, exists := h.svc.GetAdapter(providerCode)
	if !exists {
		response.Write(w, http.StatusNotFound, &response.ErrorResponse{
			Code:    "UNSUPPORTED_PROVIDER",
			Message: fmt.Sprintf("Unsupported payment provider: %s", providerCode),
		})
		return
	}

	// 1. Parse and verify webhook signature using provider adapter
	payload, err := parser.ParseWebhook(r)
	if err != nil {
		h.log.Error("Failed to parse/verify webhook payload", slog.String("provider", providerCode), logger.Err(err))
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_SIGNATURE",
			Message: fmt.Sprintf("Invalid webhook payload or signature: %v", err),
		})
		return
	}

	ctx := r.Context()

	var isPublished bool

	// 2. Delegate DB Idempotency check & Audit logging to Service layer
	if h.svc != nil {
		isPublished, err = h.svc.ProcessWebhook(ctx, payload)
		if err != nil {
			h.log.Error("Service failed to process webhook; returning HTTP 500", slog.String("provider", payload.ProviderCode), logger.Err(err))
			parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
			return
		}
	}

	if isPublished {
		h.log.Info(
			"Duplicate published webhook event ignored",
			slog.String("provider", payload.ProviderCode),
			slog.String("provider_tx_id", payload.ProviderTxID),
		)
	} else {
		// 3. Synchronously Publish to RabbitMQ Message Broker
		if h.broker != nil {
			routingKey := DetermineRoutingKey(payload)
			msgBytes, err := json.Marshal(payload)
			if err != nil {
				h.log.Error("Failed to marshal webhook payload for broker", logger.Err(err))
				parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
				return
			}

			if err := h.broker.PublishEvent(ctx, routingKey, msgBytes); err != nil {
				h.log.Error(
					"Failed to publish webhook payload to broker; returning HTTP 500 for provider retry",
					slog.String("routingKey", routingKey),
					logger.Err(err),
				)
				parser.RespondWebhook(w, r, ppi.WebhookResponseError, payload)
				return
			}

			// 4. Mark as published in Service layer so subsequent retries are safely ignored as duplicate
			if h.svc != nil {
				if err := h.svc.MarkWebhookPublished(ctx, payload.WebhookID); err != nil {
					h.log.Warn(
						"Failed to mark webhook as published in DB; broker publish succeeded",
						slog.String("webhook_id", payload.WebhookID),
						logger.Err(err),
					)
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
