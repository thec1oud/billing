package plan

import (
	"context"
	"testing"

	"github.com/thec1oud/billing/internal/substrate/money"
)

func TestPlanVersionsAreImmutable(t *testing.T) {

	repo := NewRepository()
	service := NewService(repo)

	price1, _ := money.New(1000, "USD")

	first, err := service.CreatePlan(
		context.Background(),
		"basic",
		price1,
		BillingPeriodMonthly,
	)

	if err != nil {
		t.Fatal(err)
	}

	if first.Version != 1 {
		t.Fatalf("expected version 1, got %d", first.Version)
	}

	price2, _ := money.New(1500, "USD")

	second, err := service.CreatePlan(
		context.Background(),
		"basic",
		price2,
		BillingPeriodMonthly,
	)

	if err != nil {
		t.Fatal(err)
	}

	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}

	old, err := repo.Get(
		context.Background(),
		"basic",
		1,
	)

	if err != nil {
		t.Fatal(err)
	}

	if old.FlatFeeAmount.AmountMinor != 1000 {
		t.Fatal("version 1 was modified")
	}

	newest, err := repo.Get(
		context.Background(),
		"basic",
		2,
	)

	if err != nil {
		t.Fatal(err)
	}

	if newest.FlatFeeAmount.AmountMinor != 1500 {
		t.Fatal("version 2 incorrect")
	}
}
