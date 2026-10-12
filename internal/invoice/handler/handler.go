package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/thec1oud/billing/internal/infra/api/response"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/ppi"
	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

type InvoiceService interface {
	Get(ctx context.Context, invoiceID int64) (model.Invoice, error)
	CreateDraftInvoice(ctx context.Context, actor eventmodel.Actor, accountID int64, currency money.Currency, items []model.LineItem) (model.Invoice, error)
	FinalizeInvoice(ctx context.Context, actor eventmodel.Actor, invoiceID int64, paymentTermsDays int) (model.Invoice, error)
}

type InvoiceLister interface {
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

type PurchasableItemService interface {
	GetByPlanID(ctx context.Context, planID int64) (itemmodel.PurchasableItem, error)
}

type InvoiceHandler struct {
	svc     InvoiceService
	ppi     PPIService
	itemSvc PurchasableItemService
	log     *slog.Logger
}

func NewInvoiceHandler(svc InvoiceService, ppi PPIService, itemSvc ...PurchasableItemService) *InvoiceHandler {
	var items PurchasableItemService
	if len(itemSvc) > 0 {
		items = itemSvc[0]
	}
	return &InvoiceHandler{
		svc:     svc,
		ppi:     ppi,
		itemSvc: items,
		log:     logger.ForComponent("invoice_handler"),
	}
}

func parseInvoiceID(r *http.Request) (int64, bool) {
	val := r.PathValue("invoiceID")
	if val == "" {
		val = r.PathValue("id")
	}
	id, err := strconv.ParseInt(val, 10, 64)
	return id, err == nil && id > 0
}

func (h *InvoiceHandler) HandleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id, ok := parseInvoiceID(r)
	if !ok {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: "Invoice ID must be a valid positive integer",
		})
		return
	}

	inv, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			response.Write(w, http.StatusNotFound, &response.ErrorResponse{
				Code:    "NOT_FOUND",
				Message: err.Error(),
			})
			return
		}
		response.WriteError(w, h.log, "get invoice failed", err)
		return
	}

	response.Write(w, http.StatusOK, inv)
}

func (h *InvoiceHandler) HandleListInvoices(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = r.PathValue("accountID")
	}
	accountID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || accountID <= 0 {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: "Account ID must be a valid positive integer",
		})
		return
	}

	lister, ok := h.svc.(InvoiceLister)
	if !ok {
		response.Write(w, http.StatusNotImplemented, &response.ErrorResponse{
			Code:    "NOT_IMPLEMENTED",
			Message: "Listing invoices is not supported",
		})
		return
	}

	invoices, err := lister.ListInvoices(r.Context(), accountID)
	if err != nil {
		response.WriteError(w, h.log, "list invoices failed", err)
		return
	}

	response.Write(w, http.StatusOK, invoices)
}

type PayInvoiceInput struct {
	ProviderCode   string `json:"provider_code"`
	IdempotencyKey string `json:"idempotency_key"`
}

type PayInvoiceResponse struct {
	Status            string `json:"status"`
	InternalTxID      string `json:"internal_tx_id"`
	ProviderReference string `json:"provider_reference"`
	CheckoutURL       string `json:"checkout_url,omitempty"`
	FailureCode       string `json:"failure_code,omitempty"`
	RawResponse       any    `json:"raw_response,omitempty"`
}

func (h *InvoiceHandler) HandlePayInvoice(w http.ResponseWriter, r *http.Request) {
	invoiceID, ok := parseInvoiceID(r)
	if !ok {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_ID",
			Message: "Invoice ID must be a valid positive integer",
		})
		return
	}

	var in PayInvoiceInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body: " + err.Error(),
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

	inv, err := h.svc.Get(r.Context(), invoiceID)
	if err != nil {
		if errors.Is(err, repository.ErrInvoiceNotFound) {
			response.Write(w, http.StatusNotFound, &response.ErrorResponse{
				Code:    "NOT_FOUND",
				Message: err.Error(),
			})
			return
		}
		response.WriteError(w, h.log, "get invoice failed", err)
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
		response.WriteError(w, h.log, "charge payment method failed", err)
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
		Status:            string(result.Status),
		InternalTxID:      result.IdempotencyKey,
		ProviderReference: result.ProviderReference,
		CheckoutURL:       result.CheckoutURL,
		FailureCode:       result.FailureCode,
		RawResponse:       result.RawResponse,
	})
}

type DevGenerateInvoiceInput struct {
	AccountID int64 `json:"account_id"`
	PlanID    int64 `json:"plan_id"`
}

// HandleGenerateDevInvoice is a development-only endpoint to generate an OPEN invoice for UI testing.
func (h *InvoiceHandler) HandleGenerateDevInvoice(w http.ResponseWriter, r *http.Request) {
	var in DevGenerateInvoiceInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_REQUEST_BODY",
			Message: "Failed to decode request body: " + err.Error(),
		})
		return
	}
	defer r.Body.Close()

	if in.AccountID <= 0 || in.PlanID <= 0 {
		response.Write(w, http.StatusBadRequest, &response.ErrorResponse{
			Code:    "INVALID_INPUT",
			Message: "account_id and plan_id must be greater than zero",
		})
		return
	}

	var itemID int64
	var itemName string = "Dev Mock Subscription Charge"

	if h.itemSvc != nil {
		item, err := h.itemSvc.GetByPlanID(r.Context(), in.PlanID)
		if err != nil {
			response.WriteError(w, h.log, "find plan purchasable item failed", fmt.Errorf("find plan purchasable item: %w", err))
			return
		}
		itemID = item.ID
		itemName = item.Name
	} else if db, ok := r.Context().Value("db_pool").(interface {
		QueryRow(context.Context, string, ...any) interface{ Scan(...any) error }
	}); ok {
		_ = db.QueryRow(r.Context(), "SELECT item_id FROM purchasable_items WHERE plan_id = $1 LIMIT 1", in.PlanID).Scan(&itemID)
	}

	if itemID <= 0 {
		itemID = in.PlanID
	}

	amount := money.MustNew(1000, money.Currency("ETB"))
	lineItems := []model.LineItem{
		{
			ItemID:        itemID,
			Description:   itemName,
			QuantityValue: 1,
			QuantityUnit:  "units",
			UnitAmount:    amount,
			TotalAmount:   amount,
		},
	}

	actor := eventmodel.Actor{Type: "system", ID: "web_dev_endpoint"}
	draftInv, err := h.svc.CreateDraftInvoice(r.Context(), actor, in.AccountID, "ETB", lineItems)
	if err != nil {
		response.WriteError(w, h.log, "create dev invoice draft failed", fmt.Errorf("create dev invoice: %w", err))
		return
	}

	finalInv, err := h.svc.FinalizeInvoice(r.Context(), actor, draftInv.InvoiceID, 14)
	if err != nil {
		response.WriteError(w, h.log, "finalize dev invoice failed", fmt.Errorf("finalize dev invoice: %w", err))
		return
	}

	response.Write(w, http.StatusCreated, finalInv)
}
