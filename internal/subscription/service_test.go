package subscription

import (
	"context"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/plan"
	"github.com/thec1oud/billing/internal/substrate/events"
	"github.com/thec1oud/billing/internal/substrate/idempotency"
	"github.com/thec1oud/billing/internal/substrate/money"
	"github.com/thec1oud/billing/internal/substrate/timeutil"
)

func TestCreateSubscription(t *testing.T) {

	ctx := context.Background()

	eventStore := events.NewEventStore(
		events.NewMemoryRepository(),
	)

	accountRepo := account.NewRepository(eventStore)
	planRepo := plan.NewRepository()
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
		plan.BillingPeriodMonthly,
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
		createdPlan.ID,
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

	if sub.PlanID != createdPlan.ID {
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
