package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/invoice/model"
	invoiceservice "github.com/thec1oud/billing/internal/invoice/service"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

var log = logger.ForComponent("invoice_handler")

type InvoiceHandler struct {
	svc *invoiceservice.Service
}

func NewInvoiceHandler(svc *invoiceservice.Service) *InvoiceHandler {
	return &InvoiceHandler{
		svc: svc,
	}
}

type CreateDraftRequest struct {
	AccountID int64            `json:"account_id"`
	Currency  string           `json:"currency"`
	LineItems []model.LineItem `json:"line_items"`
}

func (h *InvoiceHandler) HandleCreateDraft(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Warn("Failed to decode invoice draft payload", logger.Err(err))
		response.Write(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	actor := eventmodel.Actor{
		ID:   "ui-test-user",
		Type: eventmodel.ActorTypeUser,
	}

	invoice, err := h.svc.CreateDraftInvoice(ctx, actor, req.AccountID, money.Currency(req.Currency), req.LineItems)
	if err != nil {
		response.WriteError(w, log, "Failed to create draft invoice", err)
		return
	}

	response.Write(w, http.StatusCreated, invoice)
}

func (h *InvoiceHandler) HandleFinalize(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	invoiceIDStr := r.PathValue("id")
	invoiceID, err := strconv.ParseInt(invoiceIDStr, 10, 64)
	if err != nil {
		log.Warn("Invalid invoice ID format", logger.Err(err))
		response.Write(w, http.StatusBadRequest, "Invalid invoice ID")
		return
	}

	actor := eventmodel.Actor{
		ID:   "ui-test-user",
		Type: eventmodel.ActorTypeUser,
	}

	// For the UI demo, we will default to Net 14 days due date
	invoice, err := h.svc.FinalizeInvoice(ctx, actor, invoiceID, 14)
	if err != nil {
		response.WriteError(w, log, "Failed to finalize invoice", err)
		return
	}

	response.Write(w, http.StatusOK, invoice)
}

func (h *InvoiceHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	invoiceIDStr := r.PathValue("id")
	invoiceID, err := strconv.ParseInt(invoiceIDStr, 10, 64)
	if err != nil {
		log.Warn("Invalid invoice ID format", logger.Err(err))
		response.Write(w, http.StatusBadRequest, "Invalid invoice ID")
		return
	}

	invoice, err := h.svc.Get(ctx, invoiceID)
	if err != nil {
		response.WriteError(w, log, "Failed to get invoice", err)
		return
	}

	response.Write(w, http.StatusOK, invoice)
}
