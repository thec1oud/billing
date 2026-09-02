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
	createFn   func(ctx context.Context, in model.CreateInput) (model.Account, error)
	activateFn func(ctx context.Context, accountID int64) (model.Account, error)
}

func (m *mockAccountService) Create(ctx context.Context, in model.CreateInput) (model.Account, error) {
	if m.createFn != nil {
		return m.createFn(ctx, in)
	}
	return model.Account{}, nil
}

func (m *mockAccountService) Activate(ctx context.Context, accountID int64) (model.Account, error) {
	if m.activateFn != nil {
		return m.activateFn(ctx, accountID)
	}
	return model.Account{}, nil
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
			name: "service error",
			body: model.CreateInput{},
			mockCreate: func(ctx context.Context, in model.CreateInput) (model.Account, error) {
				return model.Account{}, errors.New("db error")
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
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.HandleCreateAccount(rec, req)

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
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "service error",
			pathID: "1",
			mockActivate: func(ctx context.Context, accountID int64) (model.Account, error) {
				return model.Account{}, errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockAccountService{activateFn: tt.mockActivate}
			h := handler.NewAccountHandler(svc)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/"+tt.pathID+"/activate", nil)
			req.SetPathValue("id", tt.pathID)
			rec := httptest.NewRecorder()

			h.HandleActivateAccount(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tt.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}
