package invoice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/invoice/model"
	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/invoice/statemachine"
	"github.com/thec1oud/billing/internal/ppi"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
)

type Service struct {
	db           *pgxpool.Pool
	eventService *eventservice.Service
	ppi          ppi.PPI
	repo         *repository.PostgresRepository
	smEngine     *engine.Engine
}

func NewService(
	db *pgxpool.Pool,
	eventService *eventservice.Service,
	ppi ppi.PPI,
	repo *repository.PostgresRepository,
	smEngine *engine.Engine,
) *Service {
	return &Service{
		db:           db,
		eventService: eventService,
		ppi:          ppi,
		repo:         repo,
		smEngine:     smEngine,
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

	// 1. Initialize State Machine Instance (pinned to the active invoice_lifecycle spec)
	initialContext, _ := json.Marshal(map[string]any{
		"invoice_id": invoiceID,
		"account_id": accountID,
		"currency":   currency,
	})
	_, err = s.smEngine.CreateInstance(ctx, sm_model.MachineType(statemachine.InvoiceMachineType), "invoice", fmt.Sprintf("%d", invoiceID), initialContext)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("failed to create state machine instance: %w", err)
	}

	// 2. Append Event
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

func (s *Service) Get(ctx context.Context, invoiceID int64) (model.Invoice, error) {
	return s.repo.Get(ctx, invoiceID)
}

func (s *Service) FinalizeInvoice(
	ctx context.Context,
	actor eventmodel.Actor,
	invoiceID int64,
	dueDateDays int,
) (model.Invoice, error) {
	// 1. Fetch draft invoice and line items (lock-free read is fine here since state engine locks during Fire)
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

	// 3. Resolve the state machine instance
	instance, err := s.smEngine.GetInstanceBySubject(ctx, "invoice", fmt.Sprintf("%d", invoiceID), sm_model.MachineType(statemachine.InvoiceMachineType))
	if err != nil {
		return model.Invoice{}, fmt.Errorf("failed to find state machine instance: %w", err)
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"subtotal":   subtotal,
		"tax":        tax,
		"discount":   discount,
		"total":      total,
		"amount_due": amountDue,
		"due_at":     dueAt,
		"now":        now,
	})
	if err != nil {
		return model.Invoice{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 4. Fire the state machine transition
	_, err = s.smEngine.Fire(ctx, instance.InstanceID, "finalize", payloadBytes, engine.WithEventActor(actor))
	if err != nil {
		return model.Invoice{}, fmt.Errorf("failed to fire state machine transition: %w", err)
	}

	// 5. Fetch finalized invoice projection from DB
	finalizedInv, err := s.repo.Get(ctx, invoiceID)
	if err != nil {
		return model.Invoice{}, fmt.Errorf("failed to fetch finalized invoice: %w", err)
	}

	return finalizedInv, nil
}

func (s *Service) PayInvoice(
	ctx context.Context,
	actor eventmodel.Actor,
	invoiceID int64,
	amountPaid money.Money,
	amountDue money.Money,
	paidAt time.Time,
) error {
	instance, err := s.smEngine.GetInstanceBySubject(ctx, "invoice", fmt.Sprintf("%d", invoiceID), sm_model.MachineType(statemachine.InvoiceMachineType))
	if err != nil {
		return fmt.Errorf("failed to find state machine instance: %w", err)
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"amount_paid": amountPaid,
		"amount_due":  amountDue,
		"paid_at":     paidAt,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = s.smEngine.Fire(ctx, instance.InstanceID, "pay", payloadBytes, engine.WithEventActor(actor))
	return err
}

func (s *Service) VoidInvoice(
	ctx context.Context,
	actor eventmodel.Actor,
	invoiceID int64,
) error {
	instance, err := s.smEngine.GetInstanceBySubject(ctx, "invoice", fmt.Sprintf("%d", invoiceID), sm_model.MachineType(statemachine.InvoiceMachineType))
	if err != nil {
		return fmt.Errorf("failed to find state machine instance: %w", err)
	}

	_, err = s.smEngine.Fire(ctx, instance.InstanceID, "void", nil, engine.WithEventActor(actor))
	return err
}
