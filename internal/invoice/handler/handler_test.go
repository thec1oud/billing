package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/ppi"
	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

type mockInvoiceService struct {
	getFn                func(ctx context.Context, id int64) (model.Invoice, error)
	listInvoicesFn       func(ctx context.Context, accountID int64) ([]model.Invoice, error)
	createDraftInvoiceFn func(ctx context.Context, actor eventmodel.Actor, accountID int64, currency money.Currency, items []model.LineItem) (model.Invoice, error)
	finalizeInvoiceFn    func(ctx context.Context, actor eventmodel.Actor, invoiceID int64, paymentTermsDays int) (model.Invoice, error)
}

func (m *mockInvoiceService) Get(ctx context.Context, id int64) (model.Invoice, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return model.Invoice{InvoiceID: id, Status: model.StatusOpen}, nil
}

func (m *mockInvoiceService) ListInvoices(ctx context.Context, accountID int64) ([]model.Invoice, error) {
	if m.listInvoicesFn != nil {
		return m.listInvoicesFn(ctx, accountID)
	}
	return []model.Invoice{{AccountID: accountID}}, nil
}

func (m *mockInvoiceService) CreateDraftInvoice(ctx context.Context, actor eventmodel.Actor, accountID int64, currency money.Currency, items []model.LineItem) (model.Invoice, error) {
	if m.createDraftInvoiceFn != nil {
		return m.createDraftInvoiceFn(ctx, actor, accountID, currency, items)
	}
	return model.Invoice{InvoiceID: 10, Status: model.StatusDraft}, nil
}

func (m *mockInvoiceService) FinalizeInvoice(ctx context.Context, actor eventmodel.Actor, invoiceID int64, paymentTermsDays int) (model.Invoice, error) {
	if m.finalizeInvoiceFn != nil {
		return m.finalizeInvoiceFn(ctx, actor, invoiceID, paymentTermsDays)
	}
	return model.Invoice{InvoiceID: invoiceID, Status: model.StatusOpen}, nil
}

type mockPPIService struct {
	charge func(ctx context.Context, providerCode string, invoiceID int64, amount money.Money, idempotencyKey string) (ppi.ChargeResult, error)
}

func (m *mockPPIService) ChargePaymentMethod(ctx context.Context, providerCode string, invoiceID int64, amount money.Money, idempotencyKey string) (ppi.ChargeResult, error) {
	return m.charge(ctx, providerCode, invoiceID, amount, idempotencyKey)
}

type mockItemService struct {
	item itemmodel.PurchasableItem
}

func (m *mockItemService) GetByPlanID(ctx context.Context, planID int64) (itemmodel.PurchasableItem, error) {
	return m.item, nil
}

func TestHandleGetInvoice(t *testing.T) {
	svc := &mockInvoiceService{
		getFn: func(ctx context.Context, invoiceID int64) (model.Invoice, error) {
			if invoiceID == 999 {
				return model.Invoice{}, repository.ErrInvoiceNotFound
			}
			return model.Invoice{InvoiceID: invoiceID, Status: model.StatusOpen}, nil
		},
	}
	h := NewInvoiceHandler(svc, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/invoices/{id}", h.HandleGetInvoice)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/v1/invoices/999", nil)
	recNotFound := httptest.NewRecorder()
	mux.ServeHTTP(recNotFound, reqNotFound)

	assert.Equal(t, http.StatusNotFound, recNotFound.Code)
}

func TestHandleListInvoices(t *testing.T) {
	svc := &mockInvoiceService{
		listInvoicesFn: func(ctx context.Context, accountID int64) ([]model.Invoice, error) {
			return []model.Invoice{{AccountID: accountID}}, nil
		},
	}
	h := NewInvoiceHandler(svc, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/accounts/{id}/invoices", h.HandleListInvoices)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/123/invoices", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandlePayInvoice(t *testing.T) {
	svc := &mockInvoiceService{
		getFn: func(ctx context.Context, invoiceID int64) (model.Invoice, error) {
			if invoiceID == 1 {
				return model.Invoice{InvoiceID: 1, Status: model.StatusOpen, AmountDue: money.Money{AmountMinor: 1000, Currency: "ETB"}}, nil
			}
			return model.Invoice{InvoiceID: invoiceID, Status: model.StatusDraft}, nil
		},
	}
	ppiSvc := &mockPPIService{
		charge: func(ctx context.Context, providerCode string, invoiceID int64, amount money.Money, idempotencyKey string) (ppi.ChargeResult, error) {
			return ppi.ChargeResult{Status: ppi.ChargeStatusPending, CheckoutURL: "http://checkout.local"}, nil
		},
	}
	h := NewInvoiceHandler(svc, ppiSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/invoices/{id}/pay", h.HandlePayInvoice)

	body, _ := json.Marshal(PayInvoiceInput{ProviderCode: "fake", IdempotencyKey: "test-payment-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invoices/1/pay", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var envelope struct {
		Data PayInvoiceResponse `json:"data"`
	}
	json.NewDecoder(rec.Body).Decode(&envelope)
	assert.Equal(t, "http://checkout.local", envelope.Data.CheckoutURL)

	// Test invalid status
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/invoices/2/pay", bytes.NewReader(body))
	recInvalid := httptest.NewRecorder()
	mux.ServeHTTP(recInvalid, reqInvalid)

	assert.Equal(t, http.StatusBadRequest, recInvalid.Code)
}

func TestHandleGenerateDevInvoice(t *testing.T) {
	svc := &mockInvoiceService{}
	itemSvc := &mockItemService{
		item: itemmodel.PurchasableItem{ID: 5, Name: "Test Item"},
	}
	h := NewInvoiceHandler(svc, nil, itemSvc)

	body, _ := json.Marshal(DevGenerateInvoiceInput{AccountID: 1, PlanID: 2})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/invoices/generate", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.HandleGenerateDevInvoice(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}
