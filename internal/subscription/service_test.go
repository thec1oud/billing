package subscription

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/plan"
	events "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/idempotency"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/timeutil"
)

type fakePlanRepository struct {
	nextID int64
	plans  map[string]map[int]plan.Plan
}

func newFakePlanRepository() *fakePlanRepository {
	return &fakePlanRepository{
		plans: map[string]map[int]plan.Plan{},
	}
}

func (r *fakePlanRepository) Save(ctx context.Context, p plan.Plan) (plan.Plan, error) {
	r.nextID++
	p.ID = r.nextID
	if _, ok := r.plans[p.PlanCode]; !ok {
		r.plans[p.PlanCode] = map[int]plan.Plan{}
	}
	r.plans[p.PlanCode][p.Version] = p
	return p, nil
}

func (r *fakePlanRepository) Get(ctx context.Context, code string, version int) (plan.Plan, error) {
	return r.GetByCodeAndVersion(ctx, code, version)
}

func (r *fakePlanRepository) GetByCodeAndVersion(ctx context.Context, code string, version int) (plan.Plan, error) {
	versions, ok := r.plans[code]
	if !ok {
		return plan.Plan{}, fmt.Errorf("plan %s version %d not found", code, version)
	}
	p, ok := versions[version]
	if !ok {
		return plan.Plan{}, fmt.Errorf("plan %s version %d not found", code, version)
	}
	return p, nil
}

func (r *fakePlanRepository) LatestVersion(ctx context.Context, code string) (int, error) {
	versions, ok := r.plans[code]
	if !ok {
		return 0, nil
	}
	latest := 0
	for version := range versions {
		if version > latest {
			latest = version
		}
	}
	return latest, nil
}

func TestCreateSubscription(t *testing.T) {

	ctx := context.Background()

	eventStore := events.NewMemoryEventStore()

	accountRepo := account.NewRepository(eventStore)
	planRepo := newFakePlanRepository()
	subscriptionRepo := NewRepository(eventStore)

	accountService := account.NewService(
		accountRepo,
		idempotency.NewStore(),
	)

	planService := plan.NewService(planRepo)

	subscriptionService := NewService(
		subscriptionRepo,
		accountRepo,
		planRepo,
		idempotency.NewStore(),
	)

	//----------------------------------------------------
	// Create Account
	//----------------------------------------------------

	acc, err := accountService.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-key",
	)
	if err != nil {
		t.Fatal(err)
	}

	//----------------------------------------------------
	// Create Plan
	//----------------------------------------------------

	price, err := money.New(1000, "USD")
	if err != nil {
		t.Fatal(err)
	}

	createdPlan, err := planService.CreatePlan(
		ctx,
		"basic",
		price,
		plan.BillingIntervalMonth,
	)
	if err != nil {
		t.Fatal(err)
	}

	//----------------------------------------------------
	// Create Subscription
	//----------------------------------------------------

	sub, err := subscriptionService.CreateSubscription(
		ctx,
		acc.AccountID,
		createdPlan.PlanCode,
		"subscription-key",
	)
	if err != nil {
		t.Fatal(err)
	}

	//----------------------------------------------------
	// Assertions
	//----------------------------------------------------

	if sub.AccountID != acc.AccountID {
		t.Fatal("wrong account")
	}

	if sub.PlanID != createdPlan.PlanCode {
		t.Fatal("wrong plan")
	}

	if sub.PlanVersion != createdPlan.Version {
		t.Fatal("wrong version")
	}

	if sub.Status != StatusActive {
		t.Fatal("subscription should be ACTIVE")
	}

	if !sub.CurrentPeriodEnd.After(sub.CurrentPeriodStart) {
		t.Fatal("period end should be after start")
	}

	if sub.BillingCycleAnchor != sub.CurrentPeriodStart.Day() {
		t.Fatal("billing anchor incorrect")
	}
}

func TestMonthlyBillingPeriodHandlesShortMonths(t *testing.T) {

	start := time.Date(
		2025,
		time.January,
		31,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	end, err := timeutil.MonthlyPeriodEnd(
		start,
		31,
	)
	if err != nil {
		t.Fatal(err)
	}

	expected := time.Date(
		2025,
		time.February,
		28,
		10,
		0,
		0,
		0,
		time.UTC,
	)

	if !end.Equal(expected) {
		t.Fatalf(
			"expected %v got %v",
			expected,
			end,
		)
	}
}
