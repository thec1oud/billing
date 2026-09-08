package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	invoicerepo "github.com/thec1oud/billing/internal/invoice/repository"
	invoiceservice "github.com/thec1oud/billing/internal/invoice/service"
	"github.com/thec1oud/billing/internal/invoice/statemachine"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
	sm_engine "github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_loader "github.com/thec1oud/billing/internal/shared/statemachine/loader"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
	sm_registry "github.com/thec1oud/billing/internal/shared/statemachine/registry"
	sm_repo "github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestInvoiceStateMachine_E2E(t *testing.T) {
	ctx := context.Background()

	// 1. Spin up Postgres & RabbitMQ Testcontainer cluster
	cluster, cleanup, err := testutil.SetupTestCluster(ctx)
	if err != nil {
		t.Fatalf("failed to setup test cluster: %v", err)
	}
	defer cleanup()

	// 2. Initialize State Machine repositories, engine, and registry
	smRegistry := sm_registry.New()
	smRepository := sm_repo.NewPostgresRepository(cluster.DBPool)

	eventStore := eventrepo.NewPostgresEventStore(cluster.DBPool)
	eventSvc := eventservice.NewService(eventStore)

	smEngine := sm_engine.NewEngine(cluster.DBPool, smRepository, smRegistry, sm_engine.WithEventStore(eventSvc))

	// 3. Initialize Invoice repository and register SM actions
	invoiceRepository := invoicerepo.NewPostgresRepository(cluster.DBPool)
	statemachine.RegisterStateMachineActions(smRegistry, invoiceRepository)

	// 4. Publish the invoice state machine spec v1
	spec := statemachine.BuildInvoiceDefinitionSpec()
	_, err = sm_loader.Publish(ctx, cluster.DBPool, smRepository, smRegistry, spec)
	if err != nil {
		t.Fatalf("failed to publish invoice state machine spec: %v", err)
	}

	// 5. Initialize the Invoice service with the SM Engine
	svc := invoiceservice.NewService(cluster.DBPool, eventSvc, invoiceRepository, smEngine)

	// Seed Account in Postgres to satisfy FK constraints
	var accountID int64
	err = cluster.DBPool.QueryRow(ctx, `
		INSERT INTO accounts (currency, timezone)
		VALUES ('ETB', 'Africa/Addis_Ababa')
		RETURNING account_id;
	`).Scan(&accountID)
	if err != nil {
		t.Fatalf("failed to seed account in test db: %v", err)
	}

	// Seed Purchasable Item in Postgres to satisfy FK constraints
	var itemID int64
	err = cluster.DBPool.QueryRow(ctx, `
		INSERT INTO purchasable_items (item_code, item_type_code, name, description)
		VALUES ('test-item-sm', 'PLAN', 'Test Plan Item', 'Seeded item for SM integration test')
		RETURNING item_id;
	`).Scan(&itemID)
	if err != nil {
		t.Fatalf("failed to seed purchasable_item in test db: %v", err)
	}

	actor := eventmodel.Actor{Type: "USER", ID: "usr_test_sm"}
	currency := money.Currency("ETB")
	amount100 := money.MustNew(10000, currency) // 100.00 ETB

	lineItems := []invoicemodel.LineItem{
		{
			ItemID:        itemID, // Use the dynamically seeded item ID
			Description:   "E2E Test Base Plan Line Item",
			QuantityValue: 1,
			QuantityUnit:  "MONTH",
			UnitAmount:    amount100,
			TotalAmount:   amount100,
		},
	}

	// ==========================================
	// Step 1: Create Draft Invoice
	// ==========================================
	draft, err := svc.CreateDraftInvoice(ctx, actor, accountID, currency, lineItems)
	if err != nil {
		t.Fatalf("CreateDraftInvoice failed: %v", err)
	}

	// Verify DB state
	dbInvoice, err := invoiceRepository.Get(ctx, draft.InvoiceID)
	if err != nil {
		t.Fatalf("failed to fetch draft invoice from DB: %v", err)
	}
	if dbInvoice.Status != invoicemodel.StatusDraft {
		t.Errorf("expected draft status DRAFT, got %s", dbInvoice.Status)
	}

	// Verify State Machine state is initialized to DRAFT
	instance, err := smEngine.GetInstanceBySubject(ctx, "invoice", fmt.Sprintf("%d", draft.InvoiceID), sm_model.MachineType(statemachine.InvoiceMachineType))
	if err != nil {
		t.Fatalf("failed to fetch state machine instance: %v", err)
	}
	if instance.CurrentState != "DRAFT" {
		t.Errorf("expected SM state to be DRAFT, got %s", instance.CurrentState)
	}

	// ==========================================
	// Step 2: Finalize Invoice
	// ==========================================
	finalized, err := svc.FinalizeInvoice(ctx, actor, draft.InvoiceID, 15)
	if err != nil {
		t.Fatalf("FinalizeInvoice failed: %v", err)
	}

	if finalized.Status != invoicemodel.StatusOpen {
		t.Errorf("expected finalized invoice status OPEN, got %s", finalized.Status)
	}
	if finalized.InvoiceNumber == "" {
		t.Errorf("expected generated invoice number, got empty")
	}

	// Verify DB state
	dbInvoice, err = invoiceRepository.Get(ctx, draft.InvoiceID)
	if err != nil {
		t.Fatalf("failed to fetch finalized invoice from DB: %v", err)
	}
	if dbInvoice.Status != invoicemodel.StatusOpen {
		t.Errorf("expected DB invoice status to be OPEN, got %s", dbInvoice.Status)
	}

	// Verify State Machine state transitioned to OPEN
	instance, err = smEngine.GetInstanceBySubject(ctx, "invoice", fmt.Sprintf("%d", draft.InvoiceID), sm_model.MachineType(statemachine.InvoiceMachineType))
	if err != nil {
		t.Fatalf("failed to fetch state machine instance: %v", err)
	}
	if instance.CurrentState != "OPEN" {
		t.Errorf("expected SM state to be OPEN, got %s", instance.CurrentState)
	}

	// Verify state machine context was enriched during finalization action
	var smContext map[string]any
	if err := json.Unmarshal(instance.Context, &smContext); err != nil {
		t.Fatalf("failed to unmarshal SM context: %v", err)
	}
	if smContext["invoice_number"] != finalized.InvoiceNumber {
		t.Errorf("expected SM context invoice_number %q, got %v", finalized.InvoiceNumber, smContext["invoice_number"])
	}

	// ==========================================
	// Step 3: Pay Invoice
	// ==========================================
	err = svc.PayInvoice(ctx, actor, draft.InvoiceID, amount100, money.MustNew(0, currency), time.Now().UTC())
	if err != nil {
		t.Fatalf("PayInvoice failed: %v", err)
	}

	// Verify DB state updated to PAID
	dbInvoice, err = invoiceRepository.Get(ctx, draft.InvoiceID)
	if err != nil {
		t.Fatalf("failed to fetch paid invoice from DB: %v", err)
	}
	if dbInvoice.Status != invoicemodel.StatusPaid {
		t.Errorf("expected DB invoice status to be PAID, got %s", dbInvoice.Status)
	}

	// Verify State Machine state transitioned to PAID and marked COMPLETED
	instance, err = smEngine.GetInstanceBySubject(ctx, "invoice", fmt.Sprintf("%d", draft.InvoiceID), sm_model.MachineType(statemachine.InvoiceMachineType))
	if err != nil {
		t.Fatalf("failed to fetch state machine instance: %v", err)
	}
	if instance.CurrentState != "PAID" {
		t.Errorf("expected SM state to be PAID, got %s", instance.CurrentState)
	}
	if instance.Status != sm_model.InstanceStatusCompleted {
		t.Errorf("expected SM status to be COMPLETED, got %s", instance.Status)
	}

	// ==========================================
	// Step 4: Invalid Transition Guard Test
	// ==========================================
	// Attempt to pay again (an invalid event "pay" since the machine is in terminal "PAID" state)
	err = svc.PayInvoice(ctx, actor, draft.InvoiceID, amount100, money.MustNew(0, currency), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error when firing event on terminal instance, got nil")
	}
}
