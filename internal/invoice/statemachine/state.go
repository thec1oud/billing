package statemachine

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thec1oud/billing/internal/invoice/repository"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
)

const InvoiceMachineType = "invoice_lifecycle"

//go:embed state_machine.json
var invoiceStateMachineJSON []byte

// BuildInvoiceDefinitionSpec returns the Version 1 blueprint parsed from embedded JSON
func BuildInvoiceDefinitionSpec() loader.DefinitionSpec {
	var spec loader.DefinitionSpec
	if err := json.Unmarshal(invoiceStateMachineJSON, &spec); err != nil {
		panic(fmt.Errorf("failed to unmarshal embedded invoice state machine: %w", err))
	}
	return spec
}

// RegisterStateMachineActions registers the Invoice-related Database Actions into the global registry
func RegisterStateMachineActions(
	reg *registry.Registry,
	repo *repository.PostgresRepository,
) {
	_ = reg.RegisterAction(&FinalizeAction{repo: repo})
	_ = reg.RegisterAction(&MarkPaidAction{repo: repo})
	_ = reg.RegisterAction(&VoidAction{repo: repo})
}

// --- Action Implementation: Finalize ---

type FinalizeAction struct {
	repo *repository.PostgresRepository
}

func (a *FinalizeAction) Name() string { return "invoice.action.finalize" }

func (a *FinalizeAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var p struct {
		Subtotal  money.Money `json:"subtotal"`
		Tax       money.Money `json:"tax"`
		Discount  money.Money `json:"discount"`
		Total     money.Money `json:"total"`
		AmountDue money.Money `json:"amount_due"`
		DueAt     time.Time   `json:"due_at"`
		Now       time.Time   `json:"now"`
	}
	if err := json.Unmarshal(ec.EventPayload, &p); err != nil {
		return fmt.Errorf("failed to parse finalize payload: %w", err)
	}

	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse invoice ID %q: %w", ec.SubjectID, err)
	}

	invoiceNumber, err := a.repo.Finalize(ctx, db, intID, p.Subtotal, p.Tax, p.Discount, p.Total, p.AmountDue, p.DueAt, p.Now)
	if err != nil {
		return err
	}

	// Update the State Machine's context so the published event payload is fully enriched
	newCtx, err := json.Marshal(map[string]any{
		"invoice_id":      intID,
		"invoice_number":  invoiceNumber,
		"subtotal_amount": p.Subtotal,
		"tax_amount":      p.Tax,
		"discount_amount": p.Discount,
		"total_amount":    p.Total,
		"amount_due":      p.AmountDue,
		"due_at":          p.DueAt,
		"finalized_at":    p.Now,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}

// --- Action Implementation: MarkPaid ---

type MarkPaidAction struct {
	repo *repository.PostgresRepository
}

func (a *MarkPaidAction) Name() string { return "invoice.action.mark_paid" }

func (a *MarkPaidAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var p struct {
		AmountPaid money.Money `json:"amount_paid"`
		AmountDue  money.Money `json:"amount_due"`
		PaidAt     time.Time   `json:"paid_at"`
	}
	if err := json.Unmarshal(ec.EventPayload, &p); err != nil {
		return fmt.Errorf("failed to parse pay payload: %w", err)
	}

	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse invoice ID %q: %w", ec.SubjectID, err)
	}

	err := a.repo.MarkPaid(ctx, db, intID, p.AmountPaid, p.AmountDue, p.PaidAt)
	if err != nil {
		return err
	}

	// Update Context for standard event publication
	newCtx, err := json.Marshal(map[string]any{
		"invoice_id":  intID,
		"amount_paid": p.AmountPaid,
		"amount_due":  p.AmountDue,
		"paid_at":     p.PaidAt,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}

// --- Action Implementation: Void ---

type VoidAction struct {
	repo *repository.PostgresRepository
}

func (a *VoidAction) Name() string { return "invoice.action.void" }

func (a *VoidAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse invoice ID %q: %w", ec.SubjectID, err)
	}

	return a.repo.MarkVoid(ctx, db, intID)
}
