package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/plan/handler"
	plan "github.com/thec1oud/billing/internal/plan/model"
)

type mockPlanService struct {
	createErr error
}

func (m *mockPlanService) CreatePlan(ctx context.Context, tx pgx.Tx, p plan.Plan) (plan.Plan, error) {
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

func (m *mockPlanService) CreatePlanDuration(ctx context.Context, tx pgx.Tx, duration plan.PlanDuration) (plan.PlanDuration, error) {
	duration.ID = 1
	return duration, nil
}

func (m *mockPlanService) ListActivePlans(ctx context.Context) ([]plan.Plan, error) {
	return []plan.Plan{{ID: 1, PlanCode: "PRO"}}, nil
}

func TestHandleCreatePlan(t *testing.T) {
	svc := &mockPlanService{}
	h := handler.NewPlanHandler(nil, svc, nil)

	in := plan.Plan{
		PlanCode:              "PRO",
		LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
		Durations: []plan.PlanDuration{
			{Duration: 30 * 24 * time.Hour},
		},
	}
	body, _ := json.Marshal(in)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/plans", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.HandleCreatePlan(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestHandleListPlans(t *testing.T) {
	svc := &mockPlanService{}
	h := handler.NewPlanHandler(nil, svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plans", nil)
	w := httptest.NewRecorder()

	h.HandleListPlans(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}
