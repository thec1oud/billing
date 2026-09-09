package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/api"
	"github.com/thec1oud/billing/internal/infra/messaging"
	plan "github.com/thec1oud/billing/internal/plan/model"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/testutil"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	accountsvc "github.com/thec1oud/billing/internal/account/service"

	planrepo "github.com/thec1oud/billing/internal/plan/repository"
	planservice "github.com/thec1oud/billing/internal/plan/service"
	itemrepo "github.com/thec1oud/billing/internal/purchasable_item/repository"
	itemservice "github.com/thec1oud/billing/internal/purchasable_item/service"
	tariffhandler "github.com/thec1oud/billing/internal/tariff/handler"
	tariff "github.com/thec1oud/billing/internal/tariff/model"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
	tariffservice "github.com/thec1oud/billing/internal/tariff/service"

	subscriptionmodel "github.com/thec1oud/billing/internal/subscription/model"
	subscriptionrepo "github.com/thec1oud/billing/internal/subscription/repository"
	subscriptionsvc "github.com/thec1oud/billing/internal/subscription/service"

	attemptrepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptsvc "github.com/thec1oud/billing/internal/payment_attempt/service"

	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	ppirepo "github.com/thec1oud/billing/internal/ppi/repository"
	ppisvc "github.com/thec1oud/billing/internal/ppi/service"

	invoicemodel "github.com/thec1oud/billing/internal/invoice/model"
	invoicerepo "github.com/thec1oud/billing/internal/invoice/repository"
	invoicesvc "github.com/thec1oud/billing/internal/invoice/service"
	"github.com/thec1oud/billing/internal/invoice/statemachine"
	invoicesub "github.com/thec1oud/billing/internal/invoice/subscriber"
	ppisub "github.com/thec1oud/billing/internal/ppi/subscriber"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventsvc "github.com/thec1oud/billing/internal/shared/eventstore/service"
	sm_engine "github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_loader "github.com/thec1oud/billing/internal/shared/statemachine/loader"
	sm_registry "github.com/thec1oud/billing/internal/shared/statemachine/registry"
	sm_repo "github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
)

func TestAPI_E2E_Walkthrough(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e api test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Setup Test Cluster
	cluster, cleanup, err := testutil.SetupTestCluster(ctx)
	require.NoError(t, err)
	defer cleanup()

	broker, err := messaging.NewRabbitBroker(cluster.RabbitConn)
	require.NoError(t, err)
	require.NoError(t, broker.InitTopology(ctx))
	defer broker.Close()

	// 2. Setup Services
	accountRepo := accountrepo.New(cluster.DBPool)
	accountSvc := accountsvc.New(accountRepo)

	planRepo := planrepo.NewPostgresRepository(cluster.DBPool)
	itemRepo := itemrepo.NewPostgresRepository(cluster.DBPool)
	itemSvc := itemservice.NewService(itemRepo)
	planSvc := planservice.NewService(planRepo)

	tariffRepo := tariffrepo.NewPostgresRepository(cluster.DBPool)
	tariffSvc := tariffservice.NewService(tariffRepo)

	subscriptionRepo := subscriptionrepo.New(cluster.DBPool)
	subscriptionSvc := subscriptionsvc.New(subscriptionRepo, accountRepo, planRepo, tariffRepo)

	paymentAttemptRepo := attemptrepo.NewPostgresRepository(cluster.DBPool)
	paymentAttemptSvc := attemptsvc.NewService(paymentAttemptRepo)

	ppiRepo := ppirepo.NewPostgresRepository()
	ppiService := ppisvc.NewService(cluster.DBPool, ppiRepo, paymentAttemptSvc)
	ppiService.RegisterAdapter(fake.NewFakeAdapter())

	smRegistry := sm_registry.New()
	smRepository := sm_repo.NewPostgresRepository(cluster.DBPool)
	smEngine := sm_engine.NewEngine(cluster.DBPool, smRepository, smRegistry, sm_engine.WithScripting(scripting.NewPool(0, 0)))

	eventRepo := eventrepo.NewPostgresEventStore(cluster.DBPool)
	eventSvc := eventsvc.NewService(eventRepo)

	invoiceRepository := invoicerepo.NewPostgresRepository(cluster.DBPool)

	statemachine.RegisterStateMachineActions(smRegistry, invoiceRepository)
	_, err = sm_loader.Publish(ctx, cluster.DBPool, smRepository, smRegistry, statemachine.BuildInvoiceDefinitionSpec())
	require.NoError(t, err)

	invoiceSvc := invoicesvc.NewService(cluster.DBPool, eventSvc, ppiService, invoiceRepository, smEngine)
	invoiceSubscriber := invoicesub.NewInvoiceSubscriber(cluster.DBPool, invoiceSvc, paymentAttemptSvc)

	// Register Webhook Subscriber to RabbitMQ
	err = ppisub.RegisterModuleSubscriber(
		ctx,
		broker,
		"api_e2e_test_webhook_queue",
		[]string{"ppi.webhook.payment.*"},
		invoiceSubscriber.HandlePaymentWebhook,
	)
	require.NoError(t, err)

	// 3. Setup Server
	cfg := &config.Config{AppPort: "8080"}
	deps := api.Deps{
		Pool:                   cluster.DBPool,
		AccountService:         accountSvc,
		PlanService:            planSvc,
		TariffService:          tariffSvc,
		PurchasableItemService: itemSvc,
		SubscriptionService:    subscriptionSvc,
		InvoiceService:         invoiceSvc,
		PPIService:             ppiService,
		Broker:                 broker,
	}
	server := api.NewServer(cfg, deps)
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()

	printObj := func(label string, obj any) {
		b, _ := json.MarshalIndent(obj, "", "  ")
		t.Logf("=== %s ===\n%s\n", label, string(b))
	}

	// Helper for HTTP requests
	doJSON := func(method, path string, body any, out any) *http.Response {
		var reqBody []byte
		if body != nil {
			reqBody, _ = json.Marshal(body)
		}
		req, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		if out != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&envelope))
			require.NoError(t, json.Unmarshal(envelope.Data, out))
		}
		return resp
	}

	// Phase 1: Account Creation & Activation
	var account accountmodel.Account
	resp := doJSON("POST", "/api/v1/accounts", accountmodel.CreateInput{
		Currency: "ETB",
		Timezone: "Africa/Addis_Ababa",
		NetTerms: 0,
	}, &account)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotZero(t, account.AccountID)

	resp = doJSON("POST", fmt.Sprintf("/api/v1/accounts/%d/activate", account.AccountID), nil, &account)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, accountmodel.StatusActive, account.Status)

	printObj("Activated Account", account)

	// Phase 2: Create Tariff
	var createdTariff tariff.Tariff
	resp = doJSON("POST", "/api/v1/tariffs", tariffhandler.CreateInput{
		Code:        "standard_per_unit_1000",
		Name:        "Standard Per Unit 1000 ETB",
		Description: "Basic per unit pricing",
		TariffType:  tariff.TariffTypePerUnit,
		Amount:      money.Money{AmountMinor: 1000, Currency: "ETB"},
	}, &createdTariff)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotZero(t, createdTariff.ID)

	printObj("Created Tariff", createdTariff)

	// Phase 3: Create Plan
	var createdPlan plan.Plan
	resp = doJSON("POST", "/api/v1/plans", plan.Plan{
		PlanCode:              "e2e_premium",
		LegacyPricePolicyCode: plan.LegacyPolicyKeepForever,
		Durations: []plan.PlanDuration{
			{
				TariffID: createdTariff.ID,
				Duration: time.Hour * 24 * 30, // 30 days
			},
		},
	}, &createdPlan)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotZero(t, createdPlan.ID)

	printObj("Created Plan", createdPlan)

	// Verify the PurchasableItem was created automatically via the DB
	item, err := itemSvc.GetByCode(ctx, fmt.Sprintf("e2e_premium_v%d", createdPlan.Version))
	require.NoError(t, err)
	require.Equal(t, createdPlan.ID, *item.PlanID)

	printObj("Generated Purchasable Item", item)

	// Phase 4: Create Subscription
	var sub subscriptionmodel.Subscription
	resp = doJSON("POST", "/api/v1/subscriptions", subscriptionmodel.CreateInput{
		AccountID:   account.AccountID,
		PlanID:      createdPlan.ID,
		PlanVersion: createdPlan.Version,
	}, &sub)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotZero(t, sub.SubscriptionID)
	require.Equal(t, subscriptionmodel.StatusActive, sub.Status)

	printObj("Created Subscription", sub)

	// Phase 5: Create dummy invoice and Pay via API
	// Insert an OPEN invoice directly to test the Pay endpoint
	_, err = cluster.DBPool.Exec(ctx, `
		INSERT INTO payment_provider (payment_provider_code)
		VALUES ('fake')
		ON CONFLICT DO NOTHING;
	`)
	require.NoError(t, err)

	// Use InvoiceService to properly create and finalize the invoice instead of raw SQL
	actor := eventmodel.Actor{Type: "system", ID: "api_e2e_test"}
	lineItems := []invoicemodel.LineItem{
		{
			ItemID:        item.ID,
			Description:   "Standard Usage",
			QuantityValue: 1,
			QuantityUnit:  "units",
			UnitAmount:    money.Money{AmountMinor: 1000, Currency: "ETB"},
			TotalAmount:   money.Money{AmountMinor: 1000, Currency: "ETB"},
		},
	}

	draftInv, err := invoiceSvc.CreateDraftInvoice(ctx, actor, account.AccountID, "ETB", lineItems)
	require.NoError(t, err)

	printObj("Draft Invoice", draftInv)

	// Finalize to make it OPEN, so the checkout payload is accepted
	finalInv, err := invoiceSvc.FinalizeInvoice(ctx, actor, draftInv.InvoiceID, 14)
	require.NoError(t, err)
	require.Equal(t, invoicemodel.StatusOpen, finalInv.Status)

	printObj("Finalized Invoice", finalInv)

	var payRes map[string]any
	payReqBody := map[string]string{
		"provider_code":   "fake",
		"idempotency_key": "req_" + strconv.FormatInt(time.Now().UnixNano(), 10),
	}
	resp = doJSON("POST", fmt.Sprintf("/api/v1/invoices/%d/pay", finalInv.InvoiceID), payReqBody, &payRes)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	printObj("Pay Invoice Response", payRes)

	checkoutURL, ok := payRes["checkout_url"].(string)
	require.True(t, ok, "checkout_url must be a string")
	require.NotEmpty(t, checkoutURL)

	internalTxID, ok := payRes["internal_tx_id"].(string)
	require.True(t, ok, "internal_tx_id must be a string")

	providerRef, ok := payRes["provider_reference"].(string)
	require.True(t, ok, "provider_reference must be a string")

	// Phase 6: Simulate Webhook from Provider
	webhookPayload := map[string]any{
		"event_id":     "wh_evt_9999",
		"event":        "charge.success",
		"tx_ref":       internalTxID,
		"reference":    providerRef,
		"status":       "success",
		"amount_minor": 1000,
		"currency":     "ETB",
	}

	resp = doJSON("POST", "/api/v1/webhooks/fake", webhookPayload, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Phase 7: Verification
	// Wait a moment for RabbitMQ to deliver the message to our subscriber which pays the invoice
	var finalStatus invoicemodel.Status
	for i := 0; i < 2; i++ {
		time.Sleep(100 * time.Millisecond)
		inv, _ := invoiceRepository.Get(ctx, draftInv.InvoiceID)
		if inv.Status == invoicemodel.StatusPaid {
			finalStatus = inv.Status
			printObj("Paid Invoice", inv)
			break
		}
	}
	require.Equal(t, invoicemodel.StatusPaid, finalStatus, "invoice should reach PAID status after webhook processing")
}
