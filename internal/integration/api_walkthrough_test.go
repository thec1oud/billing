package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/api"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/testutil"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	accountsvc "github.com/thec1oud/billing/internal/account/service"

	"github.com/thec1oud/billing/internal/plan"
	purchasableitem "github.com/thec1oud/billing/internal/purchasable_item"
	"github.com/thec1oud/billing/internal/tariff"
	tariffhandler "github.com/thec1oud/billing/internal/tariff/handler"

	subscriptionmodel "github.com/thec1oud/billing/internal/subscription/model"
	subscriptionrepo "github.com/thec1oud/billing/internal/subscription/repository"
	subscriptionsvc "github.com/thec1oud/billing/internal/subscription/service"

	attemptrepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptsvc "github.com/thec1oud/billing/internal/payment_attempt/service"

	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	ppirepo "github.com/thec1oud/billing/internal/ppi/repository"
	ppisvc "github.com/thec1oud/billing/internal/ppi/service"
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

	planRepo := plan.NewPostgresRepository(cluster.DBPool)
	itemRepo := purchasableitem.NewPostgresRepository(cluster.DBPool)
	itemSvc := purchasableitem.NewService(itemRepo)
	planSvc := plan.NewService(cluster.DBPool, planRepo, itemSvc)

	tariffRepo := tariff.NewPostgresRepository(cluster.DBPool)
	tariffSvc := tariff.NewService(tariffRepo)

	subscriptionRepo := subscriptionrepo.New(cluster.DBPool)
	subscriptionSvc := subscriptionsvc.New(subscriptionRepo, accountRepo, planRepo, tariffRepo)

	paymentAttemptRepo := attemptrepo.NewPostgresRepository(cluster.DBPool)
	paymentAttemptSvc := attemptsvc.NewService(paymentAttemptRepo)

	ppiRepo := ppirepo.NewPostgresRepository()
	ppiService := ppisvc.NewService(cluster.DBPool, ppiRepo, paymentAttemptSvc)
	ppiService.RegisterAdapter(fake.NewFakeAdapter())

	// 3. Setup Server
	cfg := &config.Config{AppPort: "8080"}
	deps := api.Deps{
		AccountService:      accountSvc,
		PlanService:         planSvc,
		TariffService:       tariffSvc,
		SubscriptionService: subscriptionSvc,
		PPIService:          ppiService,
		Broker:              broker,
	}
	server := api.NewServer(cfg, deps)
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()

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

	// Phase 2: Create Tariff
	var createdTariff tariff.Tariff
	resp = doJSON("POST", "/api/v1/tariffs", tariffhandler.CreateInput{
		Code:        "api_tariff",
		Name:        "API Tariff",
		Description: "API usage tariff",
		TariffType:  tariff.TariffTypePerUnit,
		Amount:      money.Money{AmountMinor: 1000, Currency: "ETB"},
	}, &createdTariff)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotZero(t, createdTariff.ID)

	// Phase 3: Create Plan
	var createdPlan plan.Plan
	resp = doJSON("POST", "/api/v1/plans", plan.Plan{
		PlanCode:              "api_premium",
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

	// Verify the PurchasableItem was created automatically via the DB
	item, err := itemSvc.GetByCode(ctx, "api_premium_v1")
	require.NoError(t, err)
	require.Equal(t, createdPlan.ID, *item.PlanID)

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
}
