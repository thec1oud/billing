package invoice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/ppi/adapters"
	"github.com/thec1oud/billing/internal/substrate/events"
	"github.com/thec1oud/billing/internal/substrate/idempotency"
)

type stubPlanLookup struct{}

func (stubPlanLookup) FlatFeeForSubscription(ctx context.Context, subscriptionID uuid.UUID) (uuid.UUID, int64, string, time.Time, time.Time, error) {
	now := time.Now().UTC()
	return uuid.New(), 2000, "USD", now, now.AddDate(0, 1, 0), nil
}

type stubAccountLookup struct {
	paymentMethodID string
}

func (s stubAccountLookup) DefaultPaymentMethodID(ctx context.Context, accountID uuid.UUID) (string, error) {
	return s.paymentMethodID, nil
}

func newTestService(paymentMethodID string) *Service {
	store := events.NewEventStore(events.NewMemoryRepository())
	idem := idempotency.NewStore()
	ppiAdapter := adapters.NewFakeAdapter(idem)
	accounts := stubAccountLookup{paymentMethodID: paymentMethodID}
	return NewService(store, idem, stubPlanLookup{}, accounts, ppiAdapter)
}

func TestCreateDraftInvoice_IdempotentOnRetry(t *testing.T) {
	svc := newTestService("pm_good")
	ctx := context.Background()
	subID := uuid.New()

	first, err := svc.CreateDraftInvoice(ctx, subID, "idem-1")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := svc.CreateDraftInvoice(ctx, subID, "idem-1")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if first.InvoiceID != second.InvoiceID {
		t.Errorf("expected same invoice_id on retry, got %s then %s", first.InvoiceID, second.InvoiceID)
	}
}

// TestFinalizeInvoice_RejectsAllMutationAfterFinalization proves the C2
// immutability requirement (§7.5): once an invoice is OPEN, the aggregate
// rejects further mutation. Re-finalization is used as the mutation attempt
// since Milestone 1 has no separate line-item-modification method — see the
// scope note on FinalizeInvoice.
func TestFinalizeInvoice_RejectsAllMutationAfterFinalization(t *testing.T) {
	svc := newTestService("pm_good")
	ctx := context.Background()

	inv, err := svc.CreateDraftInvoice(ctx, uuid.New(), "idem-1")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	if _, err := svc.FinalizeInvoice(ctx, inv.InvoiceID); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	_, err = svc.FinalizeInvoice(ctx, inv.InvoiceID)
	if !errors.Is(err, ErrInvoiceNotMutable) {
		t.Fatalf("expected ErrInvoiceNotMutable on mutation attempt after finalization, got %v", err)
	}
}

func TestAttemptPayment_SuccessTransitionsInvoiceToPaid(t *testing.T) {
	svc := newTestService("pm_good")
	ctx := context.Background()

	inv, err := svc.CreateDraftInvoice(ctx, uuid.New(), "idem-1")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := svc.FinalizeInvoice(ctx, inv.InvoiceID); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	paid, err := svc.AttemptPayment(ctx, inv.InvoiceID, "pay-idem-1")
	if err != nil {
		t.Fatalf("attempt payment: %v", err)
	}
	if paid.Status != StatusPaid {
		t.Errorf("expected status PAID, got %s", paid.Status)
	}
}

func TestAttemptPayment_FailureLeavesInvoiceOpen(t *testing.T) {
	svc := newTestService("pm_fail_card") // "fail" in payment_method_id triggers FakeAdapter's failure path
	ctx := context.Background()

	inv, err := svc.CreateDraftInvoice(ctx, uuid.New(), "idem-1")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := svc.FinalizeInvoice(ctx, inv.InvoiceID); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	result, err := svc.AttemptPayment(ctx, inv.InvoiceID, "pay-idem-1")
	if err != nil {
		t.Fatalf("attempt payment: %v", err)
	}
	if result.Status != StatusOpen {
		t.Errorf("expected status to remain OPEN after failed payment, got %s", result.Status)
	}
}

func TestAttemptPayment_RejectsPaymentOnDraftInvoice(t *testing.T) {
	svc := newTestService("pm_good")
	ctx := context.Background()

	inv, err := svc.CreateDraftInvoice(ctx, uuid.New(), "idem-1")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	_, err = svc.AttemptPayment(ctx, inv.InvoiceID, "pay-idem-1")
	if !errors.Is(err, ErrInvoiceNotMutable) {
		t.Fatalf("expected ErrInvoiceNotMutable when attempting payment on a DRAFT invoice, got %v", err)
	}
}

func TestAttemptPayment_IdempotentOnRetry(t *testing.T) {
	svc := newTestService("pm_good")
	ctx := context.Background()

	inv, err := svc.CreateDraftInvoice(ctx, uuid.New(), "idem-1")
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if _, err := svc.FinalizeInvoice(ctx, inv.InvoiceID); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	first, err := svc.AttemptPayment(ctx, inv.InvoiceID, "pay-idem-1")
	if err != nil {
		t.Fatalf("first attempt: %v", err)
	}
	second, err := svc.AttemptPayment(ctx, inv.InvoiceID, "pay-idem-1")
	if err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if second.Status != first.Status {
		t.Errorf("expected retry to return same status, got %s then %s", first.Status, second.Status)
	}
}
