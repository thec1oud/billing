package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestPostgresRepository_Get_NotFound(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	_, err = repo.Get(ctx, -999999)
	if !errors.Is(err, repository.ErrInvoiceNotFound) {
		t.Errorf("expected ErrInvoiceNotFound, got %v", err)
	}
}

func TestPostgresRepository_CreateAndGet(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	subtotal, _ := money.New(5000, "USD")
	tax, _ := money.New(500, "USD")
	discount, _ := money.New(0, "USD")
	total, _ := money.New(5500, "USD")
	amountPaid, _ := money.New(0, "USD")
	amountDue, _ := money.New(5500, "USD")

	inv := model.Invoice{
		AccountID:  1,
		Status:     model.StatusDraft,
		Currency:   "USD",
		Subtotal:   subtotal,
		Tax:        tax,
		Discount:   discount,
		Total:      total,
		AmountPaid: amountPaid,
		AmountDue:  amountDue,
		LineItems: []model.LineItem{
			{
				ItemID:        1,
				Description:   "Consulting",
				QuantityValue: 2.5,
				QuantityUnit:  "hours",
				UnitAmount:    subtotal,
				TotalAmount:   subtotal,
			},
		},
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	invoiceID, err := repo.Create(ctx, tx, inv)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit tx: %v", err)
	}

	fetched, err := repo.Get(ctx, invoiceID)
	if err != nil {
		t.Fatalf("failed to fetch invoice: %v", err)
	}

	if fetched.InvoiceID != invoiceID {
		t.Errorf("expected invoice ID %d, got %d", invoiceID, fetched.InvoiceID)
	}
	if fetched.Status != model.StatusDraft {
		t.Errorf("expected status draft, got %s", fetched.Status)
	}
	if len(fetched.LineItems) != 1 {
		t.Fatalf("expected 1 line item, got %d", len(fetched.LineItems))
	}
	if fetched.LineItems[0].QuantityValue != 2.5 {
		t.Errorf("expected quantity 2.5, got %f", fetched.LineItems[0].QuantityValue)
	}
}

func TestPostgresRepository_LifecycleTransitions(t *testing.T) {
	pool, err := testutil.GetTestPool()
	if err != nil {
		t.Skipf("Skipping integration test: failed to get test pool (%v)", err)
	}
	defer pool.Close()

	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	subtotal, _ := money.New(1000, "USD")
	tax, _ := money.New(0, "USD")
	discount, _ := money.New(0, "USD")
	total, _ := money.New(1000, "USD")
	amountPaid, _ := money.New(0, "USD")
	amountDue, _ := money.New(1000, "USD")

	inv := model.Invoice{
		AccountID:  1,
		Status:     model.StatusDraft,
		Currency:   "USD",
		Subtotal:   subtotal,
		Tax:        tax,
		Discount:   discount,
		Total:      total,
		AmountPaid: amountPaid,
		AmountDue:  amountDue,
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	invoiceID, err := repo.Create(ctx, tx, inv)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to create invoice: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit create: %v", err)
	}

	fetched, err := repo.Get(ctx, invoiceID)
	if err != nil {
		t.Fatalf("failed to get invoice: %v", err)
	}

	// Finalize
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin finalize tx: %v", err)
	}
	finalizedAt := time.Now().UTC()
	_, err = repo.Finalize(ctx, tx, invoiceID, subtotal, tax, discount, total, amountDue, *inv.DueAt, finalizedAt)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to finalize: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit finalize: %v", err)
	}

	fetched, err = repo.Get(ctx, invoiceID)
	if err != nil {
		t.Fatalf("failed to get invoice: %v", err)
	}
	if fetched.Status != model.StatusOpen {
		t.Errorf("expected status open after finalize, got %s", fetched.Status)
	}

	// Mark Paid
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin pay tx: %v", err)
	}
	paidAt := time.Now().UTC()
	zeroDue, _ := money.New(0, "USD")
	err = repo.MarkPaid(ctx, tx, invoiceID, fetched.Total, zeroDue, paidAt)
	if err != nil {
		tx.Rollback(ctx)
		t.Fatalf("failed to mark paid: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit pay: %v", err)
	}

	fetched, err = repo.Get(ctx, invoiceID)
	if err != nil {
		t.Fatalf("failed to get invoice: %v", err)
	}
	if fetched.Status != model.StatusPaid {
		t.Errorf("expected status paid, got %s", fetched.Status)
	}
}
