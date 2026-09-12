package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/subscription/handler"
	"github.com/thec1oud/billing/internal/subscription/model"
)

type mockSubscriptionService struct {
	createErr error
	getFn     func(ctx context.Context, subscriptionID int64) (model.Subscription, error)
}

func (m *mockSubscriptionService) Create(ctx context.Context, input model.CreateInput) (model.Subscription, error) {
	if m.createErr != nil {
		return model.Subscription{}, m.createErr
	}
	return model.Subscription{
		SubscriptionID: 1,
		AccountID:      input.AccountID,
		PlanID:         input.PlanID,
		PlanVersion:    input.PlanVersion,
		Status:         model.StatusActive,
	}, nil
}

func (m *mockSubscriptionService) Get(ctx context.Context, subscriptionID int64) (model.Subscription, error) {
	if m.getFn != nil {
		return m.getFn(ctx, subscriptionID)
	}
	return model.Subscription{
		SubscriptionID: subscriptionID,
		Status:         model.StatusActive,
	}, nil
}

func (m *mockSubscriptionService) ListAccountSubscriptions(ctx context.Context, accountID int64) ([]model.Subscription, error) {
	return []model.Subscription{{SubscriptionID: 1, AccountID: accountID}}, nil
}

func TestHandleCreateSubscription(t *testing.T) {
	svc := &mockSubscriptionService{}
	h := handler.NewSubscriptionHandler(svc)

	in := model.CreateInput{
		AccountID:   1,
		PlanID:      10,
		PlanVersion: 2,
	}
	body, _ := json.Marshal(in)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleCreateSubscription(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleGetSubscription(t *testing.T) {
	svc := &mockSubscriptionService{
		getFn: func(ctx context.Context, subscriptionID int64) (model.Subscription, error) {
			if subscriptionID == 999 {
				return model.Subscription{}, model.ErrNotFound
			}
			return model.Subscription{SubscriptionID: subscriptionID, Status: model.StatusActive}, nil
		},
	}
	h := handler.NewSubscriptionHandler(svc)

	// Success with subscriptionID
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/1", nil)
	req.SetPathValue("subscriptionID", "1")
	w := httptest.NewRecorder()
	h.HandleGetSubscription(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Not found
	req = httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/999", nil)
	req.SetPathValue("subscriptionID", "999")
	w = httptest.NewRecorder()
	h.HandleGetSubscription(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)

	// Invalid ID
	req = httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions/abc", nil)
	req.SetPathValue("subscriptionID", "abc")
	w = httptest.NewRecorder()
	h.HandleGetSubscription(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleListAccountSubscriptions(t *testing.T) {
	svc := &mockSubscriptionService{}
	h := handler.NewSubscriptionHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/1/subscriptions", nil)
	req.SetPathValue("id", "1")
	w := httptest.NewRecorder()
	h.HandleListAccountSubscriptions(w, req)
	require.Equal(t, http.StatusOK, w.Code)
}
