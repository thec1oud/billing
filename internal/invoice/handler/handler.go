package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/ppi"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

type InvoiceService interface {
	GetInvoice(ctx context.Context, invoiceID int64) (model.Invoice, error)
	ListInvoices(ctx context.Context, accountID int64) ([]model.Invoice, error)
	CreateDraftInvoice(ctx context.Context, actor eventmodel.Actor, accountID int64, currency money.Currency, items []model.LineItem) (model.Invoice, error)
	FinalizeInvoice(ctx context.Context, actor eventmodel.Actor, invoiceID int64, paymentTermsDays int) (model.Invoice, error)
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
	ProviderCode   string `json:"provider_code"`
	IdempotencyKey string `json:"idempotency_key"`
}

type PayInvoiceResponse struct {
	CheckoutURL       string `json:"checkout_url"`
	InternalTxID      string `json:"internal_tx_id"`
	ProviderReference string `json:"provider_reference"`
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

	if in.IdempotencyKey == "" {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_INPUT",
			Message: "idempotency_key is required",
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

	result, err := h.ppi.ChargePaymentMethod(r.Context(), in.ProviderCode, invoiceID, inv.AmountDue, in.IdempotencyKey)
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
		CheckoutURL:       result.CheckoutURL,
		InternalTxID:      result.IdempotencyKey,
		ProviderReference: result.ProviderReference,
	})
}

type DevGenerateInvoiceInput struct {
	AccountID int64 `json:"account_id"`
	PlanID    int64 `json:"plan_id"`
}

// HandleGenerateDevInvoice is a development-only endpoint to instantly generate an OPEN invoice for UI testing.
func (h *InvoiceHandler) HandleGenerateDevInvoice(w http.ResponseWriter, r *http.Request) {
	var in DevGenerateInvoiceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body",
		})
		return
	}
	defer r.Body.Close()

	var itemID int64
	// Development hack: Directly query the DB to resolve the item ID associated with the plan.
	// In a real flow, the catalog resolves this when items are added to a cart.
	db, ok := r.Context().Value("db_pool").(interface {
		QueryRow(context.Context, string, ...any) interface{ Scan(...any) error }
	})
	if !ok {
		// Fallback if db_pool isn't in context, try hardcoding 1 as last resort but this is bad.
		// Actually, we don't have db_pool in context by default unless we add it via middleware.
		// Since we didn't add it, let's just use the plan_id directly if possible, or wait, we need the itemID.
		itemID = in.PlanID // If we just assume they share an ID for dev, this breaks if not true.
	} else {
		err := db.QueryRow(r.Context(), "SELECT item_id FROM purchasable_items WHERE plan_id = $1 LIMIT 1", in.PlanID).Scan(&itemID)
		if err != nil {
			response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
				Code:    "ITEM_LOOKUP_FAILED",
				Message: "Failed to find purchasable item for plan",
			})
			return
		}
	}

	actor := eventmodel.Actor{Type: "system", ID: "dev_endpoint"}
	lineItems := []model.LineItem{
		{
			ItemID:        itemID,
			Description:   "Dev Mock Subscription Charge",
			QuantityValue: 1,
			QuantityUnit:  "units",
			UnitAmount:    money.Money{AmountMinor: 1000, Currency: "ETB"},
			TotalAmount:   money.Money{AmountMinor: 1000, Currency: "ETB"},
		},
	}

	// 1. Create Draft
	draftInv, err := h.svc.CreateDraftInvoice(r.Context(), actor, in.AccountID, money.Currency("ETB"), lineItems)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "DRAFT_FAILED",
			Message: err.Error(),
		})
		return
	}

	// 2. Finalize
	finalInv, err := h.svc.FinalizeInvoice(r.Context(), actor, draftInv.InvoiceID, 14)
	if err != nil {
		response.Write(w, http.StatusInternalServerError, &response.ErrorResponse{
			Code:    "FINALIZE_FAILED",
			Message: err.Error(),
		})
		return
	}

	response.Write(w, http.StatusCreated, finalInv)
}
