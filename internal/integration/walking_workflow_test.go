package integration

import (
	"context"
	"reflect"
	"testing"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/invoice"
	"github.com/thec1oud/billing/internal/plan"
	"github.com/thec1oud/billing/internal/ppi/adapters"
	"github.com/thec1oud/billing/internal/subscription"
	events "github.com/thec1oud/billing/internal/substrate/eventstore"
	"github.com/thec1oud/billing/internal/substrate/idempotency"
	"github.com/thec1oud/billing/internal/substrate/money"
)

// testDeps wires every real module against a shared in-memory event store —
// no stubs anywhere. Building this fresh per test keeps each test's data
// isolated from the others.
type testDeps struct {
	store       events.EventStore
	accountSvc  *account.Service
	accountRepo *account.Repository
	planSvc     *plan.Service
	subSvc      *subscription.Service
	subRepo     *subscription.Repository
	invoiceSvc  *invoice.Service
}

func newTestDeps() *testDeps {
	store := events.NewMemoryEventStore()
	idem := idempotency.NewStore()

	planRepo := plan.NewRepository()
	planSvc := plan.NewService(planRepo)

	accountRepo := account.NewRepository(store)
	accountSvc := account.NewService(accountRepo, idem)

	subRepo := subscription.NewRepository(store)
	subSvc := subscription.NewService(subRepo, accountRepo, planRepo, idem)

	ppiAdapter := adapters.NewFakeAdapter(idem)

	planLookup := invoice.NewSubscriptionPlanLookup(subSvc)
	accountLookup := invoice.NewAccountPaymentMethodLookup(accountSvc)
	invoiceSvc := invoice.NewService(store, idem, planLookup, accountLookup, ppiAdapter)

	return &testDeps{
		store:       store,
		accountSvc:  accountSvc,
		accountRepo: accountRepo,
		planSvc:     planSvc,
		subSvc:      subSvc,
		subRepo:     subRepo,
		invoiceSvc:  invoiceSvc,
	}
}

func TestWalkingSkeleton_EndToEnd(t *testing.T) {
	ctx := context.Background()
	deps := newTestDeps()

	// --- 1. account ---
	acct, err := deps.accountSvc.CreateAccount(ctx, "USD", "UTC", "skeleton-account-1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// --- 2. attach payment method (prerequisite AttemptPayment needs) ---
	acct, err = deps.accountSvc.AddPaymentMethod(ctx, acct.AccountID, "pm_chapa_active")
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	// --- 3. plan (prerequisite CreateSubscription needs) ---
	fee, err := money.New(2000, "USD")
	if err != nil {
		t.Fatalf("build fee: %v", err)
	}
	pln, err := deps.planSvc.CreatePlan(ctx, "plan_basic", fee, plan.BillingPeriodMonthly)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	// --- 4. subscription ---
	sub, err := deps.subSvc.CreateSubscription(ctx, acct.AccountID, pln.ID, "skeleton-sub-1")
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	// --- 5. draft invoice ---
	inv, err := deps.invoiceSvc.CreateDraftInvoice(ctx, sub.SubscriptionID, "skeleton-invoice-1")
	if err != nil {
		t.Fatalf("create draft invoice: %v", err)
	}
	if inv.Status != invoice.StatusDraft {
		t.Fatalf("expected DRAFT, got %s", inv.Status)
	}

	// --- 6. finalize ---
	inv, err = deps.invoiceSvc.FinalizeInvoice(ctx, inv.InvoiceID)
	if err != nil {
		t.Fatalf("finalize invoice: %v", err)
	}
	if inv.Status != invoice.StatusOpen {
		t.Fatalf("expected OPEN, got %s", inv.Status)
	}

	// --- 7. attempt payment (success case) ---
	inv, err = deps.invoiceSvc.AttemptPayment(ctx, inv.InvoiceID, "skeleton-payment-1")
	if err != nil {
		t.Fatalf("attempt payment: %v", err)
	}
	if inv.Status != invoice.StatusPaid {
		t.Fatalf("expected PAID, got %s", inv.Status)
	}

	// ============================================================
	// Replay test — Plan is intentionally excluded: B2 specifies it as a
	// simple keyed store, not event-sourced, so there's no stream to replay.
	// Account, Subscription, and Invoice are event-sourced and covered.
	// ============================================================

	t.Run("replay reconstructs identical state from the event log", func(t *testing.T) {
		// Account
		accountStream, err := deps.store.ReadStream(ctx, events.AggregateAccount, acct.AccountID)
		if err != nil {
			t.Fatalf("read account stream: %v", err)
		}
		rebuiltAccount, err := account.Rebuild(accountStream)
		if err != nil {
			t.Fatalf("rebuild account: %v", err)
		}
		liveAccount, err := deps.accountSvc.GetAccount(ctx, acct.AccountID)
		if err != nil {
			t.Fatalf("read live account: %v", err)
		}
		if !reflect.DeepEqual(liveAccount, rebuiltAccount) {
			t.Errorf("account state mismatch:\nlive:     %+v\nrebuilt:  %+v", liveAccount, rebuiltAccount)
		}

		// Subscription
		subStream, err := deps.store.ReadStream(ctx, events.AggregateSubscription, sub.SubscriptionID)
		if err != nil {
			t.Fatalf("read subscription stream: %v", err)
		}
		rebuiltSub, err := subscription.Rebuild(subStream)
		if err != nil {
			t.Fatalf("rebuild subscription: %v", err)
		}
		liveSub, err := deps.subRepo.Get(ctx, sub.SubscriptionID)
		if err != nil {
			t.Fatalf("read live subscription: %v", err)
		}
		if !reflect.DeepEqual(liveSub, rebuiltSub) {
			t.Errorf("subscription state mismatch:\nlive:     %+v\nrebuilt:  %+v", liveSub, rebuiltSub)
		}

		// Invoice
		invStream, err := deps.store.ReadStream(ctx, events.AggregateInvoice, inv.InvoiceID)
		if err != nil {
			t.Fatalf("read invoice stream: %v", err)
		}
		rebuiltInvoice, err := events.Rebuild(invoice.Invoice{}, invStream, invoice.Reduce)
		if err != nil {
			t.Fatalf("rebuild invoice: %v", err)
		}
		if !reflect.DeepEqual(inv, rebuiltInvoice) {
			t.Errorf("invoice state mismatch:\nlive:     %+v\nrebuilt:  %+v", inv, rebuiltInvoice)
		}
	})

	// ============================================================
	// Canary — proves the replay check above isn't vacuous. If dropping a
	// real event doesn't produce a mismatch, the comparisons above aren't
	// actually testing anything.
	// ============================================================

	t.Run("replay detects drift when an event is missing", func(t *testing.T) {
		accountStream, err := deps.store.ReadStream(ctx, events.AggregateAccount, acct.AccountID)
		if err != nil {
			t.Fatalf("read account stream: %v", err)
		}
		if len(accountStream) < 2 {
			t.Fatalf("expected at least 2 account events, got %d — test setup assumption broke", len(accountStream))
		}
		truncatedAccount, err := account.Rebuild(accountStream[:len(accountStream)-1])
		if err != nil {
			t.Fatalf("rebuild truncated account: %v", err)
		}
		liveAccount, err := deps.accountSvc.GetAccount(ctx, acct.AccountID)
		if err != nil {
			t.Fatalf("read live account: %v", err)
		}
		if reflect.DeepEqual(liveAccount, truncatedAccount) {
			t.Fatal("expected mismatch when PaymentMethodAdded is dropped — account replay check may be vacuous")
		}

		invStream, err := deps.store.ReadStream(ctx, events.AggregateInvoice, inv.InvoiceID)
		if err != nil {
			t.Fatalf("read invoice stream: %v", err)
		}
		if len(invStream) < 2 {
			t.Fatalf("expected at least 2 invoice events, got %d", len(invStream))
		}
		truncatedInvoice, err := events.Rebuild(invoice.Invoice{}, invStream[:len(invStream)-1], invoice.Reduce)
		if err != nil {
			t.Fatalf("rebuild truncated invoice: %v", err)
		}
		if reflect.DeepEqual(inv, truncatedInvoice) {
			t.Fatal("expected mismatch when InvoicePaid is dropped — invoice replay check may be vacuous")
		}
	})
}

func TestWalkingSkeleton_PaymentFailure(t *testing.T) {
	ctx := context.Background()
	deps := newTestDeps()

	acct, err := deps.accountSvc.CreateAccount(ctx, "USD", "UTC", "fail-account-1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// this is a failing payment provider pre-determined from the ID
	acct, err = deps.accountSvc.AddPaymentMethod(ctx, acct.AccountID, "pm_stripe_card_fail")
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	fee, err := money.New(2000, "USD")
	if err != nil {
		t.Fatalf("build fee: %v", err)
	}
	pln, err := deps.planSvc.CreatePlan(ctx, "plan_basic", fee, plan.BillingPeriodMonthly)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	sub, err := deps.subSvc.CreateSubscription(ctx, acct.AccountID, pln.ID, "fail-sub-1")
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	inv, err := deps.invoiceSvc.CreateDraftInvoice(ctx, sub.SubscriptionID, "fail-invoice-1")
	if err != nil {
		t.Fatalf("create draft invoice: %v", err)
	}

	inv, err = deps.invoiceSvc.FinalizeInvoice(ctx, inv.InvoiceID)
	if err != nil {
		t.Fatalf("finalize invoice: %v", err)
	}

	inv, err = deps.invoiceSvc.AttemptPayment(ctx, inv.InvoiceID, "fail-payment-1")
	if err != nil {
		t.Fatalf("attempt payment: %v", err)
	}
	if inv.Status != invoice.StatusOpen {
		t.Fatalf("expected OPEN after failed payment, got %s", inv.Status)
	}
}
