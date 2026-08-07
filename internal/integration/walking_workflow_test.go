package integration

import (
	"context"
	"reflect"
	"testing"

	"github.com/thec1oud/billing/internal/account"
	events "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/idempotency"
)

func TestWalkingSkeleton_EndToEnd(t *testing.T) {
	ctx := context.Background()
	store := events.NewMemoryEventStore()
	idem := idempotency.NewStore()

	accountRepo := account.NewRepository(store)
	accountSvc := account.NewService(accountRepo, idem)

	acct, err := accountSvc.CreateAccount(ctx, "USD", "UTC", "skeleton-account-1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	acct, err = accountSvc.AddPaymentMethod(ctx, acct.AccountID, "pm_chapa_active")
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	accountStream, err := store.ReadStream(ctx, events.AggregateAccount, acct.AccountID.String())
	if err != nil {
		t.Fatalf("read account stream: %v", err)
	}

	rebuiltAccount, err := account.Rebuild(accountStream)
	if err != nil {
		t.Fatalf("rebuild account: %v", err)
	}

	liveAccount, err := accountSvc.GetAccount(ctx, acct.AccountID)
	if err != nil {
		t.Fatalf("read live account: %v", err)
	}

	if !reflect.DeepEqual(liveAccount, rebuiltAccount) {
		t.Errorf("account state mismatch:\nlive:     %+v\nrebuilt:  %+v", liveAccount, rebuiltAccount)
	}
}

func TestWalkingSkeleton_ReplayDetectsDrift(t *testing.T) {
	ctx := context.Background()
	store := events.NewMemoryEventStore()
	idem := idempotency.NewStore()

	accountRepo := account.NewRepository(store)
	accountSvc := account.NewService(accountRepo, idem)

	acct, err := accountSvc.CreateAccount(ctx, "USD", "UTC", "drift-account-1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	acct, err = accountSvc.AddPaymentMethod(ctx, acct.AccountID, "pm_chapa_active")
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	accountStream, err := store.ReadStream(ctx, events.AggregateAccount, acct.AccountID.String())
	if err != nil {
		t.Fatalf("read account stream: %v", err)
	}

	truncatedAccount, err := account.Rebuild(accountStream[:len(accountStream)-1])
	if err != nil {
		t.Fatalf("rebuild truncated account: %v", err)
	}

	liveAccount, err := accountSvc.GetAccount(ctx, acct.AccountID)
	if err != nil {
		t.Fatalf("read live account: %v", err)
	}

	if reflect.DeepEqual(liveAccount, truncatedAccount) {
		t.Fatal("expected mismatch when PaymentMethodAdded is dropped")
	}
}
