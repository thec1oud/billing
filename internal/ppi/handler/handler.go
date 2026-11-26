package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/ppi"
)

type PPIService interface {
	GetAdapter(providerCode string) (ppi.Provider, bool)
	ProcessWebhook(ctx context.Context, payload ppi.ProviderWebhookPayload) (bool, error)
	MarkWebhookPublished(ctx context.Context, webhookID string) error
}

type WebhookService = PPIService

type PPIHandler struct {
	svc PPIService
	log *slog.Logger
}

type WebhookHandler = PPIHandler

func NewPPIHandler(
	svc PPIService,
) *PPIHandler {
	return &PPIHandler{
		svc: svc,
		log: logger.ForComponent("ppi_handler"),
	}
}

func NewWebhookHandler(
	svc PPIService,
) *PPIHandler {
	return NewPPIHandler(svc)
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

	// 2. Delegate DB Idempotency check, Audit logging, & Broker publishing to Service layer
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
	}

	// 3. Respond HTTP 200 OK to payment provider
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
