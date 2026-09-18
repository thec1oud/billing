package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	accountsvc "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/messaging"
	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	invoicerepo "github.com/thec1oud/billing/internal/invoice/repository"
	invoicesvc "github.com/thec1oud/billing/internal/invoice/service"
	"github.com/thec1oud/billing/internal/invoice/statemachine"
	invoicesub "github.com/thec1oud/billing/internal/invoice/subscriber"
	attemptrepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptsvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	plan "github.com/thec1oud/billing/internal/plan/model"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
	planservice "github.com/thec1oud/billing/internal/plan/service"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/handler"
	ppirepo "github.com/thec1oud/billing/internal/ppi/repository"
	ppisvc "github.com/thec1oud/billing/internal/ppi/service"
	ppisub "github.com/thec1oud/billing/internal/ppi/subscriber"
	purchasableitem "github.com/thec1oud/billing/internal/purchasable_item/model"
	itemrepo "github.com/thec1oud/billing/internal/purchasable_item/repository"
	itemservice "github.com/thec1oud/billing/internal/purchasable_item/service"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventsvc "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/money"
	sm_engine "github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_loader "github.com/thec1oud/billing/internal/shared/statemachine/loader"
	sm_outbox "github.com/thec1oud/billing/internal/shared/statemachine/outbox"
	sm_registry "github.com/thec1oud/billing/internal/shared/statemachine/registry"
	sm_repo "github.com/thec1oud/billing/internal/shared/statemachine/repository"
	sm_scheduler "github.com/thec1oud/billing/internal/shared/statemachine/scheduler"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
	"github.com/thec1oud/billing/internal/shared/testutil"
	subscriptionmodel "github.com/thec1oud/billing/internal/subscription/model"
	subscriptionrepo "github.com/thec1oud/billing/internal/subscription/repository"
	subscriptionsvc "github.com/thec1oud/billing/internal/subscription/service"
	tariff "github.com/thec1oud/billing/internal/tariff/model"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
	tariffservice "github.com/thec1oud/billing/internal/tariff/service"
)

var (
	toJSON = func(v any) string {
		b, _ := json.MarshalIndent(v, "", "  ")
		return string(b)
	}
	e2eLog = logger.ForComponent("e2e_script")
)

func TestE2E_BillingWalkthrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	e2eLog.Info("Starting configuration and testcontainer setup")

	// 1. Spin up Postgres & RabbitMQ containers
	cluster, cleanup, err := testutil.SetupTestCluster(ctx)
	if err != nil {
		t.Fatalf("failed to setup test cluster: %v", err)
	}
	defer cleanup()

	broker, err := messaging.NewRabbitBroker(cluster.RabbitConn)
	if err != nil {
		t.Fatalf("failed to create rabbit broker: %v", err)
	}
	if err := broker.InitTopology(ctx); err != nil {
		t.Fatalf("failed to init broker topology: %v", err)
	}
	defer broker.Close()

	// 2. Wire State Machine Engine
	smRegistry := sm_registry.New()
	smRepository := sm_repo.NewPostgresRepository(cluster.DBPool)
	smEngine := sm_engine.NewEngine(cluster.DBPool, smRepository, smRegistry, sm_engine.WithScripting(scripting.NewPool(0, 0)))

	invoiceRepository := invoicerepo.NewPostgresRepository(cluster.DBPool)
	statemachine.RegisterStateMachineActions(smRegistry, invoiceRepository)

	_, err = sm_loader.Publish(ctx, cluster.DBPool, smRepository, smRegistry, statemachine.BuildInvoiceDefinitionSpec())
	if err != nil {
		t.Fatalf("failed to bootstrap invoice state machine: %v", err)
	}

	smPublisher := sm_outbox.NewPublisher(cluster.DBPool, smRepository, smRegistry, sm_outbox.Config{})
	smPublisher.Start(ctx)
	defer smPublisher.Stop()

	smScheduler := sm_scheduler.NewPoller(smEngine, smRepository, sm_scheduler.Config{Interval: 100 * time.Millisecond})
	smScheduler.Start(ctx)
	defer smScheduler.Stop()

	// 3. Wire Domain Services
	accountRepo := accountrepo.New(cluster.DBPool)
	accountSvc := accountsvc.New(accountRepo)

	planRepo := planrepo.NewPostgresRepository(cluster.DBPool)
	planSvc := planservice.NewService(cluster.DBPool, planRepo)
	tariffRepo := tariffrepo.NewPostgresRepository(cluster.DBPool)
	tariffSvc := tariffservice.NewService(cluster.DBPool, tariffRepo)

	itemRepo := itemrepo.NewPostgresRepository(cluster.DBPool)
	itemSvc := itemservice.NewService(cluster.DBPool,itemRepo)

	subscriptionRepo := subscriptionrepo.New(cluster.DBPool)
	subscriptionSvc := subscriptionsvc.New(subscriptionRepo, accountRepo, planRepo, tariffRepo)

	eventRepo := eventrepo.NewPostgresEventStore(cluster.DBPool)
	eventSvc := eventsvc.NewService(eventRepo)

	paymentAttemptRepo := attemptrepo.NewPostgresRepository(cluster.DBPool)
	paymentAttemptSvc := attemptsvc.NewService(paymentAttemptRepo)

	ppiRepo := ppirepo.NewPostgresRepository()
	ppiService := ppisvc.NewService(cluster.DBPool, ppiRepo, paymentAttemptSvc)
	ppiService.RegisterAdapter(fake.NewFakeAdapter())

	invoiceSvc := invoicesvc.NewService(cluster.DBPool, eventSvc, ppiService, invoiceRepository, smEngine)
	invoiceSubscriber := invoicesub.NewInvoiceSubscriber(cluster.DBPool, invoiceSvc, paymentAttemptSvc)
	webhookHandler := handler.NewWebhookHandler(ppiService, broker)

	// 4. Register Webhook Subscriber to RabbitMQ
	err = ppisub.RegisterModuleSubscriber(
		ctx,
		broker,
		"e2e_test_webhook_queue",
		[]string{"ppi.webhook.payment.*"},
		invoiceSubscriber.HandlePaymentWebhook,
	)
	if err != nil {
		t.Fatalf("failed to register module subscriber on rabbitmq container: %v", err)
	}

	// 5. Seed DB
	_, err = cluster.DBPool.Exec(ctx, `
		INSERT INTO payment_provider (payment_provider_code)
		VALUES ('fake')
		ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		t.Fatalf("failed to seed fake payment provider: %v", err)
	}

	// Phase 1: Account Creation
	e2eLog.Info("Finished configuration and setup")
	actor := eventmodel.Actor{Type: "system", ID: "e2e_test"}
	account, err := accountSvc.Create(ctx, actor, accountmodel.CreateInput{
		Currency: "ETB",
		Timezone: "Africa/Addis_Ababa",
		NetTerms: 0,
	})
	if err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	account, err = accountSvc.Activate(ctx, actor, account.AccountID)
	if err != nil {
		t.Fatalf("failed to activate account: %v", err)
	}

	e2eLog.Info("Account ready\n" + toJSON(account))

	// Phase 2: Plan & Subscription

	// Use Domain Services to setup pricing model
	tx, err := cluster.DBPool.Begin(ctx)
	if err != nil {
		t.Fatalf("failed to begin tx: %v", err)
	}
	defer tx.Rollback(ctx)

	createdTariff, err := tariffSvc.CreateTariff(
		ctx,
		"standard_per_unit_1000",
		"Standard Per Unit 1000 ETB",
		"Basic per unit pricing",
		tariff.TariffTypePerUnit,
		"",
		"",
		money.Money{AmountMinor: 1000, Currency: "ETB"},
		nil, // no tiers
		nil, // metadata
	)
	if err != nil {
		t.Fatalf("failed to create tariff: %v", err)
	}

	createdPlan, err := planSvc.CreatePlan(
		ctx,
		plan.Plan{
			PlanCode:              "e2e_premium",
			LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
			Durations: []plan.PlanDuration{
				{
					TariffID: createdTariff.ID,
					Duration: time.Hour * 24 * 30, // ~ 1 month
					IsActive: true,
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	planID := createdPlan.ID
	createdItem, err := itemSvc.Create(
		ctx,
		purchasableitem.PurchasableItem{
			ItemCode:     "api_usage",
			ItemTypeCode: purchasableitem.ItemTypePlan,
			Name:         "API Usage",
			PlanID:       &planID,
		},
	)
	if err != nil {
		t.Fatalf("failed to create purchasable item: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("failed to commit tx: %v", err)
	}
	sub, err := subscriptionSvc.Create(ctx, subscriptionmodel.CreateInput{
		AccountID:   account.AccountID,
		PlanID:      createdPlan.ID,
		PlanVersion: createdPlan.Version,
	})
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	e2eLog.Info("Subscription ready\n" + toJSON(sub))

	// Phase 3: Invoice Lifecycle


	lineItems := []invoicemodel.LineItem{
		{
			ItemID:         createdItem.ID,
			SubscriptionID: sub.SubscriptionID,
			Description:    "API Usage",
			QuantityValue:  100,
			QuantityUnit:   "requests",
			UnitAmount:     money.Money{AmountMinor: 1000, Currency: "ETB"},
			TotalAmount:    money.Money{AmountMinor: 100000, Currency: "ETB"},
		},
	}

	draftInv, err := invoiceSvc.CreateDraftInvoice(ctx, actor, account.AccountID, "ETB", lineItems)
	if err != nil {
		t.Fatalf("failed to create draft invoice: %v", err)
	}
	e2eLog.Info("Draft Invoice created\n" + toJSON(draftInv))

	finalInv, err := invoiceSvc.FinalizeInvoice(ctx, actor, draftInv.InvoiceID, 14)
	if err != nil {
		t.Fatalf("failed to finalize invoice: %v", err)
	}

	// Wait for State Machine Poller to process Finalize transition
	time.Sleep(500 * time.Millisecond)

	finalInv, err = invoiceRepository.Get(ctx, draftInv.InvoiceID)
	if err != nil || finalInv.Status != invoicemodel.StatusOpen {
		t.Fatalf("invoice failed to reach OPEN status: err=%v, status=%s", err, finalInv.Status)
	}
	e2eLog.Info("Invoice Finalized and Open\n" + toJSON(finalInv))

	// Phase 4: Payment Attempt & Mock Webhook Callback

	idempotencyKey := "tx_e2e_12345"
	chargeRes, err := ppiService.ChargePaymentMethod(ctx, "fake", finalInv.InvoiceID, finalInv.AmountDue, idempotencyKey)
	if err != nil {
		t.Fatalf("failed to charge payment method: %v", err)
	}
	e2eLog.Info("Payment Attempt created via PPI Charge\n" + toJSON(chargeRes))

	// Simulate receiving a webhook from the provider via HTTP
	webhookBody := []byte(`{
		"event_id": "wh_evt_9999",
		"event": "charge.success",
		"tx_ref": "` + idempotencyKey + `",
		"reference": "` + chargeRes.ProviderReference + `",
		"status": "success",
		"amount_minor": 100000,
		"currency": "ETB"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(webhookBody))
	req.SetPathValue("provider", "fake")
	rec := httptest.NewRecorder()
	webhookHandler.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK from webhook handler, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	// Phase 5: Verification

	// Wait for RabbitMQ delivery, Invoice Subscriber handling, and State Machine processing
	var finalStatus invoicemodel.Status
	for i := 0; i < 20; i++ { // Poll for up to 2 seconds
		time.Sleep(100 * time.Millisecond)
		inv, _ := invoiceRepository.Get(ctx, draftInv.InvoiceID)
		if inv.Status == invoicemodel.StatusPaid {
			finalStatus = inv.Status
			break
		}
	}

	if finalStatus != invoicemodel.StatusPaid {
		t.Fatalf("invoice failed to reach PAID status after webhook processing")
	}

	e2eLog.Info("E2E Billing Walkthrough Complete! Invoice is PAID.")
}
