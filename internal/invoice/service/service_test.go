package invoice_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thec1oud/billing/internal/invoice/model"
	invoicerepo "github.com/thec1oud/billing/internal/invoice/repository"
	invoiceservice "github.com/thec1oud/billing/internal/invoice/service"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestService_CreateDraftInvoice_Validation(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	eStore := eventrepo.NewPostgresEventStore(pool)
	eSvc := eventservice.NewService(eStore)
	repo := invoicerepo.NewPostgresRepository(pool)
	svc := invoiceservice.NewService(pool, eSvc, nil, repo)

	actor := eventmodel.Actor{Type: "USER", ID: "usr_test"}
	usd := money.Currency("USD")
	eur := money.Currency("EUR")

	usd10 := money.MustNew(1000, usd)
	eur10 := money.MustNew(1000, eur)

	t.Run("fails when line items list is empty", func(t *testing.T) {
		_, err := svc.CreateDraftInvoice(ctx, actor, 1, usd, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "without at least one purchased item") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("fails on currency mismatch between invoice and line item", func(t *testing.T) {
		lineItems := []model.LineItem{
			{ItemID: 1, Description: "Item A", TotalAmount: usd10},
			{ItemID: 2, Description: "Item B", TotalAmount: eur10},
		}
		_, err := svc.CreateDraftInvoice(ctx, actor, 1, usd, lineItems)
		if err == nil {
			t.Fatal("expected error on currency mismatch, got nil")
		}
		if !strings.Contains(err.Error(), "currency EUR does not match invoice currency USD") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestService_DraftAndFinalize_Lifecycle(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	eStore := eventrepo.NewPostgresEventStore(pool)
	eSvc := eventservice.NewService(eStore)
	repo := invoicerepo.NewPostgresRepository(pool)
	svc := invoiceservice.NewService(pool, eSvc, nil, repo)

	actor := eventmodel.Actor{Type: "SYSTEM", ID: "billing_test"}
	accountID := int64(1)
	usd := money.Currency("USD")

	itemAmount1 := money.MustNew(2000, usd) // $20.00
	itemAmount2 := money.MustNew(3000, usd) // $30.00

	// Note: item_id must exist in purchasable_items table per foreign key constraints
	lineItems := []model.LineItem{
		{ItemID: 1, Description: "Base Plan", QuantityValue: 1, UnitAmount: itemAmount1, TotalAmount: itemAmount1},
		{ItemID: 2, Description: "Extra Seats", QuantityValue: 1, UnitAmount: itemAmount2, TotalAmount: itemAmount2},
	}

	// 1. Create Draft
	draft, err := svc.CreateDraftInvoice(ctx, actor, accountID, usd, lineItems)
	if err != nil {
		t.Fatalf("CreateDraftInvoice failed: %v", err)
	}

	if draft.InvoiceID == 0 {
		t.Errorf("expected generated invoice ID, got 0")
	}
	if draft.Status != model.StatusDraft {
		t.Errorf("expected status DRAFT, got %s", draft.Status)
	}
	if draft.Total.AmountMinor != 0 {
		t.Errorf("expected draft total to be 0, got %d", draft.Total.AmountMinor)
	}

	// 2. Finalize Invoice
	finalized, err := svc.FinalizeInvoice(ctx, actor, draft.InvoiceID, 14)
	if err != nil {
		t.Fatalf("FinalizeInvoice failed: %v", err)
	}

	if finalized.Status != model.StatusOpen {
		t.Errorf("expected status OPEN, got %s", finalized.Status)
	}
	if finalized.InvoiceNumber == "" {
		t.Errorf("expected generated invoice number, got empty string")
	}
	if !strings.HasPrefix(finalized.InvoiceNumber, "INV-") {
		t.Errorf("expected invoice number to start with 'INV-', got %s", finalized.InvoiceNumber)
	}

	expectedTotal := int64(5000) // $50.00
	if finalized.Subtotal.AmountMinor != expectedTotal {
		t.Errorf("expected subtotal %d, got %d", expectedTotal, finalized.Subtotal.AmountMinor)
	}
	if finalized.Total.AmountMinor != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, finalized.Total.AmountMinor)
	}
	if finalized.AmountDue.AmountMinor != expectedTotal {
		t.Errorf("expected amount_due %d, got %d", expectedTotal, finalized.AmountDue.AmountMinor)
	}
	if finalized.FinalizedAt == nil {
		t.Errorf("expected finalized_at to be non-nil")
	}

	// 3. Re-Finalization Guard Test
	_, err = svc.FinalizeInvoice(ctx, actor, draft.InvoiceID, 14)
	if err == nil {
		t.Fatal("expected error when re-finalizing invoice, got nil")
	}
	if !errors.Is(err, invoicerepo.ErrInvalidStatus) {
		t.Errorf("expected ErrInvalidStatus when re-finalizing, got %v", err)
	}
}
