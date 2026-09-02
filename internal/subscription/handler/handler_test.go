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
