package plan_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thec1oud/billing/internal/plan"
	"github.com/thec1oud/billing/internal/purchasable_item"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestPlanService_Integration(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	planRepo := plan.NewPostgresRepository(pool)
	itemRepo := purchasable_item.NewPostgresRepository(pool)
	itemSvc := purchasable_item.NewService(itemRepo)
	svc := plan.NewService(pool, planRepo, itemSvc)

	t.Run("CreatePlan success", func(t *testing.T) {
		code := "INT_PRO_" + time.Now().Format("150405.000")
		p := plan.Plan{
			PlanCode:              code,
			LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
			EffectiveFrom:         time.Now().UTC(),
		}

		created, err := svc.CreatePlan(ctx, p)
		require.NoError(t, err)
		require.NotZero(t, created.ID)
		require.Equal(t, code, created.PlanCode)
		require.Equal(t, 1, created.Version)

		// Verify PurchasableItem was created automatically
		item, err := itemSvc.GetByCode(ctx, code+"_v1")
		require.NoError(t, err)
		require.Equal(t, code+" Plan (v1)", item.Name)
		require.Equal(t, purchasable_item.ItemTypePlan, item.ItemTypeCode)
		require.Equal(t, created.ID, *item.PlanID)
	})

	t.Run("CreatePlan invalid plan code", func(t *testing.T) {
		_, err := svc.CreatePlan(ctx, plan.Plan{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "plan code is required")
	})

	t.Run("CreatePlan invalid price policy", func(t *testing.T) {
		_, err := svc.CreatePlan(ctx, plan.Plan{
			PlanCode:              "INT_TEST",
			LegacyPricePolicyCode: "invalid",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported legacy price policy")
	})

	t.Run("GetPlanVersion", func(t *testing.T) {
		code := "GET_TEST_" + time.Now().Format("150405.000")
		created, err := svc.CreatePlan(ctx, plan.Plan{
			PlanCode:              code,
			LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
		})
		require.NoError(t, err)

		fetched, err := svc.GetPlanVersion(ctx, code, created.Version)
		require.NoError(t, err)
		require.Equal(t, created.ID, fetched.ID)
	})

	t.Run("LatestVersion", func(t *testing.T) {
		code := "VERSION_TEST_" + time.Now().Format("150405.000")
		// Version 1
		_, err := svc.CreatePlan(ctx, plan.Plan{
			PlanCode:              code,
			LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
		})
		require.NoError(t, err)

		// Version 2
		_, err = svc.CreatePlan(ctx, plan.Plan{
			PlanCode:              code,
			LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
		})
		require.NoError(t, err)

		v, err := svc.LatestVersion(ctx, code)
		require.NoError(t, err)
		require.Equal(t, 2, v)
	})
}
