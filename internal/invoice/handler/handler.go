package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/shared/money"
)

type InvoiceService interface {
	GetInvoice(ctx context.Context, invoiceID int64) (model.Invoice, error)
	ListInvoices(ctx context.Context, accountID int64) ([]model.Invoice, error)
}

type PPIService interface {
	ChargePaymentMethod(
		ctx context.Context,
		providerCode string,
		invoiceID int64,
		amount money.Money,
		idempotencyKey string,
	) (ppi.ChargeResult, error)
}

type InvoiceHandler struct {
	svc InvoiceService
	ppi PPIService
}

func NewInvoiceHandler(svc InvoiceService, ppi PPIService) *InvoiceHandler {
	return &InvoiceHandler{svc: svc, ppi: ppi}
}

func (h *InvoiceHandler) HandleGetInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: "Invoice ID must be a valid integer",
		})
		return
	}

	inv, err := h.svc.GetInvoice(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			response.Write(w, http.StatusNotFound, &response.ErrorResponse{
				Code:    "NOT_FOUND",
				Message: err.Error(),
			})
			return
		}
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "GET_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusOK, inv)
}

func (h *InvoiceHandler) HandleListInvoices(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: "Account ID must be a valid integer",
		})
		return
	}

	invoices, err := h.svc.ListInvoices(r.Context(), accountID)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "LIST_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusOK, invoices)
}

type PayInvoiceInput struct {
	ProviderCode string `json:"provider_code"`
}

type PayInvoiceResponse struct {
	CheckoutURL string `json:"checkout_url"`
}

func (h *InvoiceHandler) HandlePayInvoice(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	invoiceID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: "Invoice ID must be a valid integer",
		})
		return
	}

	var in PayInvoiceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body",
		})
		return
	}
	defer r.Body.Close()

	if in.ProviderCode == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_INPUT",
			Message: "provider_code is required",
		})
		return
	}

	inv, err := h.svc.GetInvoice(r.Context(), invoiceID)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			response.Write(w, http.StatusNotFound, &response.ErrorResponse{
				Code:    "NOT_FOUND",
				Message: err.Error(),
			})
			return
		}
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "GET_FAILED",
			Message: err.Error(),
		})
		return
	}

	if inv.Status != model.StatusOpen {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_STATUS",
			Message: "Invoice must be in OPEN status to be paid",
		})
		return
	}

	idempotencyKey := "req_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	result, err := h.ppi.ChargePaymentMethod(r.Context(), in.ProviderCode, invoiceID, inv.AmountDue, idempotencyKey)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "CHARGE_FAILED",
			Message: err.Error(),
		})
		return
	}

	if result.Status == ppi.ChargeStatusFailed {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "PAYMENT_FAILED",
			Message: "Payment was rejected or failed",
		})
		return
	}

	response.Write(w, http.StatusOK, PayInvoiceResponse{
		CheckoutURL: result.CheckoutURL,
	})
}
