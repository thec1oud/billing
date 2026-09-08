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
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/money"
)

type mockInvoiceService struct {
	getInvoiceFn         func(ctx context.Context, id int64) (model.Invoice, error)
	listInvoicesFn       func(ctx context.Context, accountID int64) ([]model.Invoice, error)
	createDraftInvoiceFn func(ctx context.Context, actor eventmodel.Actor, accountID int64, currency money.Currency, items []model.LineItem) (model.Invoice, error)
	finalizeInvoiceFn    func(ctx context.Context, actor eventmodel.Actor, invoiceID int64, paymentTermsDays int) (model.Invoice, error)
}

func (m *mockInvoiceService) GetInvoice(ctx context.Context, id int64) (model.Invoice, error) {
	return m.getInvoiceFn(ctx, id)
}

func (m *mockInvoiceService) ListInvoices(ctx context.Context, accountID int64) ([]model.Invoice, error) {
	return m.listInvoicesFn(ctx, accountID)
}

func (m *mockInvoiceService) CreateDraftInvoice(ctx context.Context, actor eventmodel.Actor, accountID int64, currency money.Currency, items []model.LineItem) (model.Invoice, error) {
	if m.createDraftInvoiceFn != nil {
		return m.createDraftInvoiceFn(ctx, actor, accountID, currency, items)
	}
	return model.Invoice{}, nil
}

func (m *mockInvoiceService) FinalizeInvoice(ctx context.Context, actor eventmodel.Actor, invoiceID int64, paymentTermsDays int) (model.Invoice, error) {
	if m.finalizeInvoiceFn != nil {
		return m.finalizeInvoiceFn(ctx, actor, invoiceID, paymentTermsDays)
	}
	return model.Invoice{}, nil
}

type mockPPIService struct {
	charge func(ctx context.Context, providerCode string, invoiceID int64, amount money.Money, idempotencyKey string) (ppi.ChargeResult, error)
}

func (m *mockPPIService) ChargePaymentMethod(ctx context.Context, providerCode string, invoiceID int64, amount money.Money, idempotencyKey string) (ppi.ChargeResult, error) {
	return m.charge(ctx, providerCode, invoiceID, amount, idempotencyKey)
}

func TestHandleGetInvoice(t *testing.T) {
	svc := &mockInvoiceService{
		getInvoice: func(ctx context.Context, invoiceID int64) (model.Invoice, error) {
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
		listInvoices: func(ctx context.Context, accountID int64) ([]model.Invoice, error) {
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
		getInvoice: func(ctx context.Context, invoiceID int64) (model.Invoice, error) {
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

	body, _ := json.Marshal(PayInvoiceInput{ProviderCode: "fake"})
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
