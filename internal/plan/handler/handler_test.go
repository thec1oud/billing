package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	planhandler "github.com/thec1oud/billing/internal/plan/handler"
	planmodel "github.com/thec1oud/billing/internal/plan/model"
)

type mockPlanService struct {
	createPlanErr error
	listPlansErr  error

	createPlanCalled         bool
	createPlanDurationCalled bool
	listActivePlansCalled   bool

	createdPlan planmodel.Plan
}

func (m *mockPlanService) CreatePlan(
	ctx context.Context,
	plan planmodel.Plan,
) (planmodel.Plan, error) {
	m.createPlanCalled = true

	if m.createPlanErr != nil {
		return planmodel.Plan{}, m.createPlanErr
	}

	plan.ID = 1
	plan.Version = 1

	if plan.EffectiveFrom.IsZero() {
		plan.EffectiveFrom = time.Now().UTC()
	}

	m.createdPlan = plan

	return plan, nil
}

func (m *mockPlanService) CreatePlanDuration(
	ctx context.Context,
	duration planmodel.PlanDuration,
) (planmodel.PlanDuration, error) {
	m.createPlanDurationCalled = true

	duration.ID = 1

	return duration, nil
}

func (m *mockPlanService) ListActivePlans(
	ctx context.Context,
) ([]planmodel.Plan, error) {
	m.listActivePlansCalled = true

	if m.listPlansErr != nil {
		return nil, m.listPlansErr
	}

	return []planmodel.Plan{
		{
			ID:       1,
			PlanCode: "PRO",
		},
	}, nil
}

func TestHandleCreatePlan(t *testing.T) {
	t.Run("creates plan successfully", func(t *testing.T) {
		svc := &mockPlanService{}

		h := planhandler.NewPlanHandler(svc)

		input := planmodel.Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: planmodel.LegacyPolicyKeepForever,
			Durations: []planmodel.PlanDuration{
				{
					TariffID:  1,
					Duration:  30 * 24 * time.Hour,
				},
			},
		}

		body, err := json.Marshal(input)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/plans",
			bytes.NewReader(body),
		)

		w := httptest.NewRecorder()

		h.HandleCreatePlan(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		require.True(t, svc.createPlanCalled)

		require.Equal(t, int64(1), svc.createdPlan.ID)
		require.Equal(t, "PRO", svc.createdPlan.PlanCode)
		require.Equal(t, 1, svc.createdPlan.Version)
	})

	t.Run("returns bad request for invalid json", func(t *testing.T) {
		svc := &mockPlanService{}

		h := planhandler.NewPlanHandler(svc)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/plans",
			bytes.NewBufferString(`{"plan_code":`),
		)

		w := httptest.NewRecorder()

		h.HandleCreatePlan(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.False(t, svc.createPlanCalled)
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		svc := &mockPlanService{
			createPlanErr: errors.New("create plan failed"),
		}

		h := planhandler.NewPlanHandler(svc)

		input := planmodel.Plan{
			PlanCode:              "PRO",
			LegacyPricePolicyCode: planmodel.LegacyPolicyKeepForever,
		}

		body, err := json.Marshal(input)
		require.NoError(t, err)

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/plans",
			bytes.NewReader(body),
		)

		w := httptest.NewRecorder()

		h.HandleCreatePlan(w, req)

		require.NotEqual(t, http.StatusCreated, w.Code)
		require.True(t, svc.createPlanCalled)
	})
}

func TestHandleListPlans(t *testing.T) {
	t.Run("lists active plans successfully", func(t *testing.T) {
		svc := &mockPlanService{}

		h := planhandler.NewPlanHandler(svc)

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/plans",
			nil,
		)

		w := httptest.NewRecorder()

		h.HandleListPlans(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, svc.listActivePlansCalled)
	})

	t.Run("returns error when service fails", func(t *testing.T) {
		svc := &mockPlanService{
			listPlansErr: errors.New("list plans failed"),
		}

		h := planhandler.NewPlanHandler(svc)

		req := httptest.NewRequest(
			http.MethodGet,
			"/api/v1/plans",
			nil,
		)

		w := httptest.NewRecorder()

		h.HandleListPlans(w, req)

		require.NotEqual(t, http.StatusOK, w.Code)
		require.True(t, svc.listActivePlansCalled)
	})
}