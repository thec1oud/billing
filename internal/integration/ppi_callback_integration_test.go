package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	attemptRepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptSvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/handler"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/ppi/service"
	"github.com/thec1oud/billing/internal/ppi/subscriber"
	"github.com/thec1oud/billing/internal/infra/messaging"
	"github.com/thec1oud/billing/internal/infra/logger"
	"log/slog"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

var log = logger.ForComponent("integration_test")

func TestPPI_PaymentAttemptAndWebhookFlow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	log.Info("🚀 [1/5] Spinning up Postgres 16 & RabbitMQ Testcontainers...")

	// 1. Spin up Postgres & RabbitMQ containers with migrations via testcontainers
	cluster, cleanup, err := testutil.SetupTestCluster(ctx)
	if err != nil {
		t.Fatalf("failed to setup test cluster testcontainers: %v", err)
	}
	defer cleanup()

	// Seed required payment_provider record in Postgres testcontainer
	_, err = cluster.DBPool.Exec(ctx, `
		INSERT INTO payment_provider (payment_provider_code)
		VALUES ('fake')
		ON CONFLICT (payment_provider_code) DO NOTHING;
	`)
	if err != nil {
		t.Fatalf("failed to seed fake payment_provider in test db: %v", err)
	}

	// Seed account & invoice records in Postgres testcontainer to satisfy FK constraints
	var accountID int64
	err = cluster.DBPool.QueryRow(ctx, `
		INSERT INTO accounts (currency, timezone)
		VALUES ('ETB', 'Africa/Addis_Ababa')
		RETURNING account_id;
	`).Scan(&accountID)
	if err != nil {
		t.Fatalf("failed to seed account in test db: %v", err)
	}

	var invoiceID int64
	err = cluster.DBPool.QueryRow(ctx, `
		INSERT INTO invoices (account_id, currency, total_amount, amount_due)
		VALUES ($1, 'ETB', 15000, 15000)
		RETURNING invoice_id;
	`, accountID).Scan(&invoiceID)
	if err != nil {
		t.Fatalf("failed to seed invoice in test db: %v", err)
	}

	log.Info("🌱 [2/5] Database seeded successfully", slog.Int64("account_id", accountID), slog.Int64("invoice_id", invoiceID), slog.String("provider", "fake"))

	// 2. Wire RabbitMQ Broker
	broker, err := messaging.NewRabbitBroker(cluster.RabbitConn)
	if err != nil {
		t.Fatalf("failed to create rabbit broker: %v", err)
	}
	if err := broker.InitTopology(ctx); err != nil {
		t.Fatalf("failed to init broker topology: %v", err)
	}
	defer broker.Close()

	// 3. Wire Payment Attempt & PPI Services
	paymentAttemptRepo := attemptRepo.NewPostgresRepository(cluster.DBPool)
	paymentAttemptSvc := attemptSvc.NewService(paymentAttemptRepo)

	ppiRepo := repository.NewPostgresRepository()
	ppiService := service.NewService(cluster.DBPool, ppiRepo, paymentAttemptSvc)

	fakeAdapter := fake.NewFakeAdapter()
	ppiService.RegisterAdapter(fakeAdapter)

	webhookHandler := handler.NewWebhookHandler(ppiService, broker)

	// 4. Test Outbound Charge Flow (Transactional PaymentAttempt creation)
	amt, _ := money.New(15000, "ETB")
	idempotencyKey := "tx_test_integration_888"

	chargeResult, err := ppiService.ChargePaymentMethod(ctx, "fake", invoiceID, amt, idempotencyKey)
	if err != nil {
		t.Fatalf("unexpected error charging payment method: %v", err)
	}

	if chargeResult.Status != ppi.ChargeStatusPending {
		t.Errorf("expected PENDING charge status for fake provider, got %q", chargeResult.Status)
	}
	if chargeResult.CheckoutURL == "" {
		t.Error("expected non-empty CheckoutURL in charge result")
	}

	log.Info("💳 [3/5] Charged payment method via PPI Service",
		slog.String("status", string(chargeResult.Status)),
		slog.String("provider_ref", chargeResult.ProviderReference),
		slog.String("checkout_url", chargeResult.CheckoutURL),
	)

	// Verify PaymentAttempt record was committed in Postgres container
	var savedInvoiceID int64
	var savedStatus string
	err = cluster.DBPool.QueryRow(ctx,
		"SELECT invoice_id, status FROM payment_attempts WHERE internal_tx_id = $1",
		idempotencyKey,
	).Scan(&savedInvoiceID, &savedStatus)
	if err != nil {
		t.Fatalf("failed to fetch saved payment attempt from test DB: %v", err)
	}
	if savedInvoiceID != invoiceID {
		t.Errorf("expected invoice_id %d, got %d", invoiceID, savedInvoiceID)
	}

	log.Info("💾 PaymentAttempt record committed in PostgreSQL DB", slog.Int64("invoice_id", savedInvoiceID), slog.String("tx_id", idempotencyKey), slog.String("status", savedStatus))

	// 5. Test Inbound Webhook HTTP Callback + RabbitMQ Message Dispatch
	receivedMsgChan := make(chan ppi.ProviderWebhookPayload, 1)

	// Register test subscriber on RabbitMQ container
	err = subscriber.RegisterModuleSubscriber(
		ctx,
		broker,
		"integration_test_webhook_queue",
		[]string{"ppi.webhook.payment.*"},
		func(ctx context.Context, payload ppi.ProviderWebhookPayload) error {
			receivedMsgChan <- payload
			return nil
		},
	)
	if err != nil {
		t.Fatalf("failed to register module subscriber on rabbitmq container: %v", err)
	}

	webhookBody := []byte(`{
		"event_id": "evt_integration_100",
		"event": "charge.success",
		"tx_ref": "tx_test_integration_888",
		"reference": "` + chargeResult.ProviderReference + `",
		"status": "success",
		"amount_minor": 15000,
		"currency": "ETB"
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/fake", bytes.NewBuffer(webhookBody))
	req.SetPathValue("provider", "fake")
	rec := httptest.NewRecorder()

	log.Info("📩 [4/5] Sending HTTP POST Webhook callback to /api/v1/webhooks/fake...")

	webhookHandler.HandleWebhook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK from webhook handler, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "processed" {
		t.Errorf("expected status 'processed', got %q", resp["status"])
	}

	log.Info("HTTP Webhook response from handler", slog.Int("status", rec.Code), slog.String("response_body", rec.Body.String()))

	// Wait for message arrival from RabbitMQ container
	select {
	case payload := <-receivedMsgChan:
		if payload.InternalTxID != idempotencyKey {
			t.Errorf("expected InternalTxID %q from RabbitMQ, got %q", idempotencyKey, payload.InternalTxID)
		}
		if payload.Status != ppi.WebhookPaymentSucceeded {
			t.Errorf("expected WebhookPaymentSucceeded status, got %q", payload.Status)
		}
		log.Info("🎉 [5/5] Webhook payload successfully received from RabbitMQ broker queue!",
			slog.String("webhook_id", payload.WebhookID),
			slog.String("provider_code", payload.ProviderCode),
			slog.String("internal_tx_id", payload.InternalTxID),
			slog.String("provider_tx_id", payload.ProviderTxID),
			slog.String("status", string(payload.Status)),
			slog.String("currency", string(payload.Amount.Currency)),
			slog.Int64("amount_minor", payload.Amount.AmountMinor),
		)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for webhook message from RabbitMQ testcontainer")
	}
}
