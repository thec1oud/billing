package subscription

import (
	"context"
	"testing"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/plan"
	events "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/idempotency"
	"github.com/thec1oud/billing/internal/shared/money"
)

func TestBillingProjection(t *testing.T) {

	ctx := context.Background()

	//----------------------------------------------------------------------
	// Shared infrastructure
	//----------------------------------------------------------------------

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

	//----------------------------------------------------------------------
	// Create Account
	//----------------------------------------------------------------------

	acc, err := accountService.CreateAccount(
		ctx,
		"USD",
		"Africa/Addis_Ababa",
		"account-projection",
	)
	if err != nil {
		t.Fatal(err)
	}

	//----------------------------------------------------------------------
	// Create Plan
	//----------------------------------------------------------------------

	price, err := money.New(1000, "USD")
	if err != nil {
		t.Fatal(err)
	}

	p, err := planService.CreatePlan(
		ctx,
		"basic",
		price,
		plan.BillingIntervalMonth,
	)
	if err != nil {
		t.Fatal(err)
	}

	//----------------------------------------------------------------------
	// Create Subscription
	//----------------------------------------------------------------------

	sub, err := subscriptionService.CreateSubscription(
		ctx,
		acc.AccountID,
		p.PlanCode,
		"subscription-projection",
	)
	if err != nil {
		t.Fatal(err)
	}

	//----------------------------------------------------------------------
	// Read Billing Projection
	//----------------------------------------------------------------------

	projection, err := subscriptionRepo.BillingProjection(
		ctx,
		sub.SubscriptionID,
		planRepo,
	)
	if err != nil {
		t.Fatal(err)
	}

	//----------------------------------------------------------------------
	// Assertions
	//----------------------------------------------------------------------

	if projection.PlanID != p.PlanCode {
		t.Fatalf("expected plan %s got %s", p.PlanCode, projection.PlanID)
	}

	if projection.PlanVersion != p.Version {
		t.Fatalf("expected version %d got %d", p.Version, projection.PlanVersion)
	}

	if projection.Currency != "USD" {
		t.Fatalf("expected USD got %s", projection.Currency)
	}

	if projection.Amount.AmountMinor != 1000 {
		t.Fatalf("expected amount 1000 got %d", projection.Amount.AmountMinor)
	}

	if projection.PeriodStart.IsZero() {
		t.Fatal("period start should not be zero")
	}

	if projection.PeriodEnd.IsZero() {
		t.Fatal("period end should not be zero")
	}

	if !projection.PeriodEnd.After(projection.PeriodStart) {
		t.Fatal("period end must be after period start")
	}
}
