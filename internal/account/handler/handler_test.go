package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thec1oud/billing/internal/account/handler"
	"github.com/thec1oud/billing/internal/account/model"
)

type mockAccountService struct {
	createFn           func(ctx context.Context, in model.CreateInput) (model.Account, error)
	getFn              func(ctx context.Context, id int64) (model.Account, error)
	activateFn         func(ctx context.Context, accountID int64) (model.Account, error)
	reactivateFn       func(ctx context.Context, accountID int64) (model.Account, error)
	suspendFn          func(ctx context.Context, accountID int64, reason string) (model.Account, error)
	closeFn            func(ctx context.Context, accountID int64, reason string) (model.Account, error)
	addPaymentMethodFn func(ctx context.Context, accountID int64, paymentMethodID string) (model.Account, error)
}

func (m *mockAccountService) Create(ctx context.Context, in model.CreateInput) (model.Account, error) {
	if m.createFn != nil {
		return m.createFn(ctx, in)
	}
	return model.Account{AccountID: 1}, nil
}

func (m *mockAccountService) Get(ctx context.Context, id int64) (model.Account, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return model.Account{AccountID: id}, nil
}

func (m *mockAccountService) Activate(ctx context.Context, accountID int64) (model.Account, error) {
	if m.activateFn != nil {
		return m.activateFn(ctx, accountID)
	}
	return model.Account{AccountID: accountID, Status: model.StatusActive}, nil
}

func (m *mockAccountService) Reactivate(ctx context.Context, accountID int64) (model.Account, error) {
	if m.reactivateFn != nil {
		return m.reactivateFn(ctx, accountID)
	}
	return model.Account{AccountID: accountID, Status: model.StatusActive}, nil
}

func (m *mockAccountService) Suspend(ctx context.Context, accountID int64, reason string) (model.Account, error) {
	if m.suspendFn != nil {
		return m.suspendFn(ctx, accountID, reason)
	}
	return model.Account{AccountID: accountID, Status: model.StatusSuspended}, nil
}

func (m *mockAccountService) Close(ctx context.Context, accountID int64, reason string) (model.Account, error) {
	if m.closeFn != nil {
		return m.closeFn(ctx, accountID, reason)
	}
	return model.Account{AccountID: accountID, Status: model.StatusClosed}, nil
}

func (m *mockAccountService) AddPaymentMethod(ctx context.Context, accountID int64, paymentMethodID string) (model.Account, error) {
	if m.addPaymentMethodFn != nil {
		return m.addPaymentMethodFn(ctx, accountID, paymentMethodID)
	}
	return model.Account{AccountID: accountID}, nil
}

func TestHandleCreateAccount(t *testing.T) {
	tests := []struct {
		name           string
		body           any
		mockCreate     func(ctx context.Context, in model.CreateInput) (model.Account, error)
		expectedStatus int
	}{
		{
			name: "success",
			body: model.CreateInput{
				ExternalID: "ext-1",
				Currency:   "ETB",
			},
			mockCreate: func(ctx context.Context, in model.CreateInput) (model.Account, error) {
				return model.Account{AccountID: 1, ExternalID: "ext-1", Currency: "ETB"}, nil
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid body",
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "service validation error",
			body: model.CreateInput{},
			mockCreate: func(ctx context.Context, in model.CreateInput) (model.Account, error) {
				return model.Account{}, errors.New("currency is required")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "service internal error",
			body: model.CreateInput{},
			mockCreate: func(ctx context.Context, in model.CreateInput) (model.Account, error) {
				return model.Account{}, errors.New("db connection down")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockAccountService{createFn: tt.mockCreate}
			h := handler.NewAccountHandler(svc)

			var bodyBytes []byte
			if str, ok := tt.body.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", bytes.NewReader(bodyBytes))
			rec := httptest.NewRecorder()

			h.HandleCreate(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleGetAccount(t *testing.T) {
	tests := []struct {
		name           string
		pathID         string
		mockGet        func(ctx context.Context, id int64) (model.Account, error)
		expectedStatus int
	}{
		{
			name:   "success",
			pathID: "123",
			mockGet: func(ctx context.Context, id int64) (model.Account, error) {
				return model.Account{AccountID: id}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid ID",
			pathID:         "xyz",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "not found",
			pathID: "999",
			mockGet: func(ctx context.Context, id int64) (model.Account, error) {
				return model.Account{}, model.ErrNotFound
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockAccountService{getFn: tt.mockGet}
			h := handler.NewAccountHandler(svc)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/"+tt.pathID, nil)
			req.SetPathValue("accountID", tt.pathID)
			rec := httptest.NewRecorder()

			h.HandleGet(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestHandleActivateAccount(t *testing.T) {
	tests := []struct {
		name           string
		pathID         string
		mockActivate   func(ctx context.Context, accountID int64) (model.Account, error)
		expectedStatus int
	}{
		{
			name:   "success",
			pathID: "1",
			mockActivate: func(ctx context.Context, accountID int64) (model.Account, error) {
				return model.Account{AccountID: 1, Status: model.StatusActive}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid ID",
			pathID:         "abc",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "not found",
			pathID: "99",
			mockActivate: func(ctx context.Context, accountID int64) (model.Account, error) {
				return model.Account{}, model.ErrNotFound
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "invalid state",
			pathID: "1",
			mockActivate: func(ctx context.Context, accountID int64) (model.Account, error) {
				return model.Account{}, model.ErrInvalidStateTransition
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockAccountService{activateFn: tt.mockActivate}
			h := handler.NewAccountHandler(svc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+tt.pathID+"/activate", nil)
			req.SetPathValue("accountID", tt.pathID)
			rec := httptest.NewRecorder()

			h.HandleActivate(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleSuspendAndReactivate(t *testing.T) {
	svc := &mockAccountService{
		suspendFn: func(ctx context.Context, accountID int64, reason string) (model.Account, error) {
			return model.Account{AccountID: accountID, Status: model.StatusSuspended}, nil
		},
		reactivateFn: func(ctx context.Context, accountID int64) (model.Account, error) {
			return model.Account{AccountID: accountID, Status: model.StatusActive}, nil
		},
	}
	h := handler.NewAccountHandler(svc)

	// Test Suspend
	body, _ := json.Marshal(map[string]string{"reason": "overdue"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/1/suspend", bytes.NewReader(body))
	req.SetPathValue("accountID", "1")
	rec := httptest.NewRecorder()
	h.HandleSuspend(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("suspend expected 200, got %d", rec.Code)
	}

	// Test Reactivate
	req = httptest.NewRequest(http.MethodPost, "/api/v1/accounts/1/reactivate", nil)
	req.SetPathValue("accountID", "1")
	rec = httptest.NewRecorder()
	h.HandleReactivate(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reactivate expected 200, got %d", rec.Code)
	}
}

func TestHandleCloseAndPaymentMethod(t *testing.T) {
	svc := &mockAccountService{
		closeFn: func(ctx context.Context, accountID int64, reason string) (model.Account, error) {
			return model.Account{AccountID: accountID, Status: model.StatusClosed}, nil
		},
		addPaymentMethodFn: func(ctx context.Context, accountID int64, paymentMethodID string) (model.Account, error) {
			return model.Account{AccountID: accountID}, nil
		},
	}
	h := handler.NewAccountHandler(svc)

	// Test Close
	body, _ := json.Marshal(map[string]string{"reason": "user requested"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/1/close", bytes.NewReader(body))
	req.SetPathValue("accountID", "1")
	rec := httptest.NewRecorder()
	h.HandleClose(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("close expected 200, got %d", rec.Code)
	}

	// Test Add Payment Method
	body, _ = json.Marshal(map[string]string{"payment_method_id": "pm_12345"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/accounts/1/payment-methods", bytes.NewReader(body))
	req.SetPathValue("accountID", "1")
	rec = httptest.NewRecorder()
	h.HandleAddPaymentMethod(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("add payment method expected 200, got %d", rec.Code)
	}
}
