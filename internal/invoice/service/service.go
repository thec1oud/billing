package invoice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/ppi"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
)

type PlanLookup interface {
	FlatFeeForSubscription(
		ctx context.Context,
		subscriptionID int64,
	) (accountID int64, fee money.Money, periodStart, periodEnd time.Time, err error)
}

type AccountLookup interface {
	DefaultPaymentMethodID(ctx context.Context, accountID int64) (string, error)
}

type Service struct {
	db           *pgxpool.Pool
	eventService *eventservice.Service
	plans        PlanLookup
	accounts     AccountLookup
	ppi          ppi.PPI
	repo         *repository.PostgresRepository
}

func NewService(
	db *pgxpool.Pool,
	eventService *eventservice.Service,
	plans PlanLookup,
	accounts AccountLookup,
	ppi ppi.PPI,
	repo *repository.PostgresRepository,
) *Service {
	return &Service{
		db:           db,
		eventService: eventService,
		plans:        plans,
		accounts:     accounts,
		ppi:          ppi,
		repo:         repo,
	}
}
func (s *Service) CreateDraftInvoice(
	ctx context.Context,
	actor eventmodel.Actor,
	accountID int64,
	currency money.Currency,
	lineItems []model.LineItem,
) (model.Invoice, error) {
	if len(lineItems) == 0 {
		return model.Invoice{}, errors.New("cannot create draft invoice without at least one purchased item")
	}

	zeroMoney, err := money.Zero(currency)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("invalid currency for invoice: %w", err)
	}

	// Validate line item currencies match the target invoice currency
	for i, li := range lineItems {
		if li.TotalAmount.Currency != currency {
			return model.Invoice{}, fmt.Errorf("line item %d currency %s does not match invoice currency %s", i, li.TotalAmount.Currency, currency)
		}
	}

	inv := model.Invoice{
		AccountID:  accountID,
		Status:     model.StatusDraft,
		Currency:   currency,
		Subtotal:   zeroMoney,
		Tax:        zeroMoney,
		Discount:   zeroMoney,
		Total:      zeroMoney,
		AmountPaid: zeroMoney,
		AmountDue:  zeroMoney,
		LineItems:  lineItems,
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	invoiceID, err := s.repo.Create(ctx, tx, inv)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("create invoice projection: %w", err)
	}

	payload := map[string]any{
		"invoice_id": invoiceID,
		"account_id": accountID,
		"currency":   currency,
		"line_items": lineItems,
	}

	if _, err := s.eventService.AppendEvent(ctx, tx, eventmodel.AppendRequest{
		AggregateType: eventmodel.AggregateInvoice,
		AggregateID:   fmt.Sprintf("%d", invoiceID),
		EventType:     eventmodel.InvoiceCreated,
		EventVersion:  1,
		Actor:         actor,
		Payload:       payload,
	}); err != nil {
		return model.Invoice{}, fmt.Errorf("append InvoiceCreated: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Invoice{}, fmt.Errorf("commit transaction: %w", err)
	}

	inv.InvoiceID = invoiceID
	return inv, nil
}

func (s *Service) FinalizeInvoice(
	ctx context.Context,
	actor eventmodel.Actor,
	invoiceID int64,
	dueDateDays int,
) (model.Invoice, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Fetch draft invoice and line items under transaction (FOR UPDATE lock)
	inv, err := s.repo.Get(ctx, invoiceID)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("fetch invoice for finalization: %w", err)
	}

	if inv.Status != model.StatusDraft {
		return model.Invoice{}, fmt.Errorf("%w: cannot finalize invoice %d in status %s", repository.ErrInvalidStatus, invoiceID, inv.Status)
	}

	if len(inv.LineItems) == 0 {
		return model.Invoice{}, errors.New("cannot finalize an invoice with no line items")
	}

	// 2. Calculate line item subtotal
	zeroMoney, err := money.Zero(inv.Currency)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("invalid currency %s: %w", inv.Currency, err)
	}

	subtotal := zeroMoney
	for _, li := range inv.LineItems {
		var err error
		subtotal, err = subtotal.Add(li.TotalAmount)
		if err != nil {
			return model.Invoice{}, fmt.Errorf("calculate subtotal: %w", err)
		}
	}

	// TODO: Replace with dynamic pricing engine / tax provider strategy lookups.
	// Discounts and taxes will be computed via rule evaluation on line items & account metadata.
	tax := zeroMoney
	discount := zeroMoney

	total, err := subtotal.Add(tax)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("calculate total tax: %w", err)
	}
	total, err = total.Subtract(discount)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("apply discount to total: %w", err)
	}

	amountDue := total
	now := time.Now().UTC()
	dueAt := now.AddDate(0, 0, dueDateDays)

	// 3. Persist finalization and retrieve the sequentially generated invoice number
	invoiceNumber, err := s.repo.Finalize(
		ctx,
		tx,
		invoiceID,
		subtotal,
		tax,
		discount,
		total,
		amountDue,
		dueAt,
		now,
	)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("repo finalize invoice %d: %w", invoiceID, err)
	}

	// 4. Construct and append domain event
	payload := map[string]any{
		"invoice_id":      invoiceID,
		"invoice_number":  invoiceNumber,
		"account_id":      inv.AccountID,
		"currency":        inv.Currency,
		"subtotal_amount": subtotal,
		"tax_amount":      tax,
		"discount_amount": discount,
		"total_amount":    total,
		"amount_due":      amountDue,
		"due_at":          dueAt,
		"finalized_at":    now,
	}

	if _, err := s.eventService.AppendEvent(ctx, tx, eventmodel.AppendRequest{
		AggregateType: eventmodel.AggregateInvoice,
		AggregateID:   fmt.Sprintf("%d", invoiceID),
		EventType:     eventmodel.InvoiceFinalized,
		EventVersion:  1,
		Actor:         actor,
		Payload:       payload,
	}); err != nil {
		return model.Invoice{}, fmt.Errorf("append InvoiceFinalized event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Invoice{}, fmt.Errorf("commit finalization transaction: %w", err)
	}

	// 5. Update domain model representation for return
	inv.Status = model.StatusOpen
	inv.InvoiceNumber = invoiceNumber
	inv.Subtotal = subtotal
	inv.Tax = tax
	inv.Discount = discount
	inv.Total = total
	inv.AmountDue = amountDue
	inv.FinalizedAt = &now
	inv.DueAt = &dueAt

	return inv, nil
}
