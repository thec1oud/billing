package integration

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/invoice"
	"github.com/thec1oud/billing/internal/plan"
	"github.com/thec1oud/billing/internal/ppi/adapters"
	"github.com/thec1oud/billing/internal/purchasable_item"
	events "github.com/thec1oud/billing/internal/shared/eventstore"
	"github.com/thec1oud/billing/internal/shared/idempotency"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/subscription"
	"github.com/thec1oud/billing/internal/tariff"
)

type testPlanRepository struct {
	plans  map[string]map[int]plan.Plan
	nextID int64
}

func newTestPlanRepository() *testPlanRepository {
	return &testPlanRepository{
		plans: map[string]map[int]plan.Plan{},
	}
}

func (r *testPlanRepository) Save(_ context.Context, p plan.Plan) (plan.Plan, error) {
	r.nextID++
	p.ID = r.nextID
	p.TariffID = p.ID + 100
	if _, ok := r.plans[p.PlanCode]; !ok {
		r.plans[p.PlanCode] = map[int]plan.Plan{}
	}
	r.plans[p.PlanCode][p.Version] = p
	return p, nil
}

func (r *testPlanRepository) GetByCodeAndVersion(_ context.Context, code string, version int) (plan.Plan, error) {
	versions, ok := r.plans[code]
	if !ok {
		return plan.Plan{}, errors.New("plan not found")
	}
	p, ok := versions[version]
	if !ok {
		return plan.Plan{}, errors.New("version not found")
	}
	return p, nil
}

func (r *testPlanRepository) LatestVersion(_ context.Context, code string) (int, error) {
	versions, ok := r.plans[code]
	if !ok || len(versions) == 0 {
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

// testDeps wires every real module against a shared in-memory event store.
type testDeps struct {
	store       events.EventStore
	accountSvc  *account.Service
	accountRepo *account.Repository
	itemSvc     *purchasable_item.Service
	tariffSvc   *tariff.Service
	planSvc     *plan.Service
	subSvc      *subscription.Service
	subRepo     *subscription.Repository
	invoiceSvc  *invoice.Service
}

func newTestDeps() *testDeps {
	store := events.NewMemoryEventStore()
	idem := idempotency.NewStore()

	// 1. Catalog Repositories & Services
	itemRepo := purchasable_item.NewRepository(nil)
	itemSvc := purchasable_item.NewService(itemRepo)

	tariffRepo := tariff.NewRepository(nil)
	tariffSvc := tariff.NewService(tariffRepo)

	planRepo := newTestPlanRepository()
	planSvc := plan.NewService(planRepo)

	// 2. Account & Subscription Services
	accountRepo := account.NewRepository(store)
	accountSvc := account.NewService(accountRepo, idem)

	subRepo := subscription.NewRepository(store)
	subSvc := subscription.NewService(subRepo, accountRepo, planRepo, idem)

	// 3. Payment & Invoice Services
	ppiAdapter := adapters.NewFakeAdapter(idem)
	planLookup := invoice.NewSubscriptionPlanLookup(subSvc)
	accountLookup := invoice.NewAccountPaymentMethodLookup(accountSvc)
	invoiceSvc := invoice.NewService(store, idem, planLookup, accountLookup, ppiAdapter)

	return &testDeps{
		store:       store,
		accountSvc:  accountSvc,
		accountRepo: accountRepo,
		itemSvc:     itemSvc,
		tariffSvc:   tariffSvc,
		planSvc:     planSvc,
		subSvc:      subSvc,
		subRepo:     subRepo,
		invoiceSvc:  invoiceSvc,
	}
}

func TestWalkingSkeleton_EndToEnd(t *testing.T) {
	ctx := context.Background()
	deps := newTestDeps()

	// --- 1. Account Setup ---
	acct, err := deps.accountSvc.CreateAccount(ctx, "USD", "UTC", "skeleton-account-1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	acct, err = deps.accountSvc.AddPaymentMethod(ctx, acct.AccountID, "pm_chapa_active")
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	// --- 2. Tariff Definition ---
	fee, err := money.New(2000, "USD")
	if err != nil {
		t.Fatalf("build fee: %v", err)
	}

	trf, err := deps.tariffSvc.CreateTariff(
		ctx,
		"TRF_BASIC_MONTHLY",
		"Basic Monthly Flat Tariff",
		tariff.TariffTypeFlatFee,
		fee,
		tariff.BillingIntervalMonth,
		nil,
	)
	if err != nil {
		t.Fatalf("create tariff: %v", err)
	}

	// --- 3. Plan Definition ---
	pln, err := deps.planSvc.CreatePlan(ctx, "plan_basic", trf.Amount, plan.BillingIntervalMonth)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	// --- 4. Purchasable Item Definition ---
	itemDesc := "Base SaaS platform access plan"
	item, err := deps.itemSvc.CreateItem(
		ctx,
		"ITEM_PLAN_BASIC",
		purchasable_item.ItemTypePlan,
		"Basic Plan Item",
		&itemDesc,
		&pln.ID,
	)
	if err != nil {
		t.Fatalf("create purchasable item: %v", err)
	}
	if item.PlanID == nil || *item.PlanID != pln.ID {
		t.Fatalf("purchasable item plan reference mismatch")
	}

	// Subscription creation needs the immutable plan code, not the numeric DB id.
	planIDStr := pln.PlanCode

	// --- 5. Subscription ---
	sub, err := deps.subSvc.CreateSubscription(ctx, acct.AccountID, planIDStr, "skeleton-sub-1")
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	// --- 6. Draft Invoice ---
	inv, err := deps.invoiceSvc.CreateDraftInvoice(ctx, sub.SubscriptionID, "skeleton-invoice-1")
	if err != nil {
		t.Fatalf("create draft invoice: %v", err)
	}
	if inv.Status != invoice.StatusDraft {
		t.Fatalf("expected DRAFT, got %s", inv.Status)
	}

	// --- 7. Finalize Invoice ---
	inv, err = deps.invoiceSvc.FinalizeInvoice(ctx, inv.InvoiceID)
	if err != nil {
		t.Fatalf("finalize invoice: %v", err)
	}
	if inv.Status != invoice.StatusOpen {
		t.Fatalf("expected OPEN, got %s", inv.Status)
	}

	// --- 8. Attempt Payment ---
	inv, err = deps.invoiceSvc.AttemptPayment(ctx, inv.InvoiceID, "skeleton-payment-1")
	if err != nil {
		t.Fatalf("attempt payment: %v", err)
	}
	if inv.Status != invoice.StatusPaid {
		t.Fatalf("expected PAID, got %s", inv.Status)
	}

	// ============================================================
	// Replay tests
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
	// Canary Test
	// ============================================================

	t.Run("replay detects drift when an event is missing", func(t *testing.T) {
		accountStream, err := deps.store.ReadStream(ctx, events.AggregateAccount, acct.AccountID)
		if err != nil {
			t.Fatalf("read account stream: %v", err)
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
			t.Fatal("expected mismatch when PaymentMethodAdded is dropped")
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

	acct, err = deps.accountSvc.AddPaymentMethod(ctx, acct.AccountID, "pm_stripe_card_fail")
	if err != nil {
		t.Fatalf("add payment method: %v", err)
	}

	fee, err := money.New(2000, "USD")
	if err != nil {
		t.Fatalf("build fee: %v", err)
	}

	trf, err := deps.tariffSvc.CreateTariff(
		ctx,
		"TRF_BASIC_FAIL",
		"Failing Tariff",
		tariff.TariffTypeFlatFee,
		fee,
		tariff.BillingIntervalMonth,
		nil,
	)
	if err != nil {
		t.Fatalf("create tariff: %v", err)
	}

	pln, err := deps.planSvc.CreatePlan(ctx, "plan_basic", trf.Amount, plan.BillingIntervalMonth)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	planIDStr := pln.PlanCode

	sub, err := deps.subSvc.CreateSubscription(ctx, acct.AccountID, planIDStr, "fail-sub-1")
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
