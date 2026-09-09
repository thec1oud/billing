package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thec1oud/billing/internal/plan/handler"
	plan "github.com/thec1oud/billing/internal/plan/model"
)

type mockPlanService struct {
	createErr error
}

func (m *mockPlanService) CreatePlan(ctx context.Context, p plan.Plan) (plan.Plan, error) {
	if m.createErr != nil {
		return plan.Plan{}, m.createErr
	}
	p.ID = 1
	p.Version = 1
	if p.EffectiveFrom.IsZero() {
		p.EffectiveFrom = time.Now().UTC()
	}
	return p, nil
}

func (m *mockPlanService) ListActivePlans(ctx context.Context) ([]plan.Plan, error) {
	return nil, nil
}

func TestHandleCreatePlan(t *testing.T) {
	svc := &mockPlanService{}
	h := handler.NewPlanHandler(svc)

	in := plan.Plan{
		PlanCode:              "PRO",
		LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
	}
	body, _ := json.Marshal(in)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleCreatePlan(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}
