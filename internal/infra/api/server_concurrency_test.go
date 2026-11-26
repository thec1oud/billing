package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	accountrepo "github.com/thec1oud/billing/internal/account/repository"
	accountsvc "github.com/thec1oud/billing/internal/account/service"
	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/api"
	invoicehandler "github.com/thec1oud/billing/internal/invoice/handler"
	invoicerepo "github.com/thec1oud/billing/internal/invoice/repository"
	invoicesvc "github.com/thec1oud/billing/internal/invoice/service"
	attemptrepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptsvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	ppirepo "github.com/thec1oud/billing/internal/ppi/repository"
	ppisvc "github.com/thec1oud/billing/internal/ppi/service"
	eventrepo "github.com/thec1oud/billing/internal/shared/eventstore/repository"
	eventsvc "github.com/thec1oud/billing/internal/shared/eventstore/service"
	sm_engine "github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_repo "github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestConcurrentWebhooksAndInvoicePayRequests(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)

	const workers = 20

	// Seed payment_provider, account, and OPEN invoices directly in Postgres.
	_, err := pool.Exec(ctx, `
		INSERT INTO payment_provider (payment_provider_code)
		VALUES ('fake')
		ON CONFLICT (payment_provider_code) DO NOTHING;
	`)
	require.NoError(t, err)

	var accountID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO accounts (currency, timezone)
		VALUES ('ETB', 'Africa/Addis_Ababa')
		RETURNING account_id;
	`).Scan(&accountID)
	require.NoError(t, err)

	invoiceIDs := make([]int64, workers)
	for i := range workers {
		err = pool.QueryRow(ctx, `
			INSERT INTO invoices (account_id, invoice_status_code, currency, total_amount, amount_due)
			VALUES ($1, 'OPEN', 'ETB', 1000, 1000)
			RETURNING invoice_id;
		`, accountID).Scan(&invoiceIDs[i])
		require.NoError(t, err)
	}

	// Wire real repositories, services, and HTTP server backed by pgxpool.Pool.
	eventStore := eventrepo.NewPostgresEventStore(pool)
	eventService := eventsvc.NewService(eventStore)
	smRepo := sm_repo.NewPostgresRepository(pool)
	smEngine := sm_engine.NewEngine(pool, smRepo, nil)

	accountRepo := accountrepo.New(pool)
	accountService := accountsvc.NewWithEvents(accountRepo, eventService, smEngine)

	attemptRepo := attemptrepo.NewPostgresRepository(pool)
	attemptService := attemptsvc.NewService(attemptRepo)

	ppiRepo := ppirepo.NewPostgresRepository()
	ppiService := ppisvc.NewService(pool, ppiRepo, attemptService, nil)
	ppiService.RegisterAdapter(fake.NewFakeAdapter())

	invoiceRepo := invoicerepo.NewPostgresRepository(pool)
	invoiceService := invoicesvc.NewService(pool, eventService, ppiService, invoiceRepo, smEngine)

	srv := api.NewServer(&config.Config{AppPort: "0"}, api.Deps{
		Pool:           pool,
		AccountService: accountService,
		InvoiceService: invoiceService,
		PPIService:     ppiService,
	})

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	start := make(chan struct{})
	errs := make(chan error, workers*3)
	var wg sync.WaitGroup
	wg.Add(workers * 3)

	// 1. Fire concurrent POST /api/v1/invoices/{id}/pay requests.
	for i := range workers {
		go func(workerID int, invID int64) {
			defer wg.Done()
			<-start

			body, marshalErr := json.Marshal(invoicehandler.PayInvoiceInput{
				ProviderCode:   "fake",
				IdempotencyKey: fmt.Sprintf("tx_worker_%d", workerID),
			})
			if marshalErr != nil {
				errs <- fmt.Errorf("pay worker %d marshal: %w", workerID, marshalErr)
				return
			}

			resp, postErr := ts.Client().Post(
				fmt.Sprintf("%s/api/v1/invoices/%d/pay", ts.URL, invID),
				"application/json",
				bytes.NewReader(body),
			)
			if postErr != nil {
				errs <- fmt.Errorf("pay worker %d post: %w", workerID, postErr)
				return
			}
			respBody, _ := io.ReadAll(resp.Body)
			if closeErr := resp.Body.Close(); closeErr != nil {
				errs <- fmt.Errorf("pay worker %d close body: %w", workerID, closeErr)
				return
			}
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("pay worker %d status=%d body=%s", workerID, resp.StatusCode, string(respBody))
			}
		}(i, invoiceIDs[i])
	}

	// 2. Fire concurrent POST /api/v1/webhooks/fake requests persisting via real ppiRepo.
	for i := range workers {
		go func(workerID int) {
			defer wg.Done()
			<-start

			payload := map[string]any{
				"event_id":     fmt.Sprintf("evt_concurrent_%d", workerID),
				"event":        "charge.success",
				"tx_ref":       fmt.Sprintf("tx_ref_%d", workerID),
				"reference":    fmt.Sprintf("ref_%d", workerID),
				"status":       "success",
				"amount_minor": 1000,
				"currency":     "ETB",
			}
			body, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				errs <- fmt.Errorf("webhook worker %d marshal: %w", workerID, marshalErr)
				return
			}

			resp, postErr := ts.Client().Post(
				fmt.Sprintf("%s/api/v1/webhooks/fake", ts.URL),
				"application/json",
				bytes.NewReader(body),
			)
			if postErr != nil {
				errs <- fmt.Errorf("webhook worker %d post: %w", workerID, postErr)
				return
			}
			respBody, _ := io.ReadAll(resp.Body)
			if closeErr := resp.Body.Close(); closeErr != nil {
				errs <- fmt.Errorf("webhook worker %d close body: %w", workerID, closeErr)
				return
			}
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("webhook worker %d status=%d body=%s", workerID, resp.StatusCode, string(respBody))
			}
		}(i)
	}

	// 3. Fire concurrent GET /api/v1/invoices/{id} reads on the same pool.
	for i := range workers {
		go func(workerID int, invID int64) {
			defer wg.Done()
			<-start

			resp, getErr := ts.Client().Get(fmt.Sprintf("%s/api/v1/invoices/%d", ts.URL, invID))
			if getErr != nil {
				errs <- fmt.Errorf("get invoice worker %d: %w", workerID, getErr)
				return
			}
			respBody, _ := io.ReadAll(resp.Body)
			if closeErr := resp.Body.Close(); closeErr != nil {
				errs <- fmt.Errorf("get invoice worker %d close body: %w", workerID, closeErr)
				return
			}
			if resp.StatusCode != http.StatusOK {
				errs <- fmt.Errorf("get invoice worker %d status=%d body=%s", workerID, resp.StatusCode, string(respBody))
			}
		}(i, invoiceIDs[i])
	}

	close(start)
	wg.Wait()
	close(errs)

	for callErr := range errs {
		require.NoError(t, callErr)
	}

	var attemptCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_attempts`).Scan(&attemptCount)
	require.NoError(t, err)
	require.Equal(t, workers, attemptCount)

	var webhookCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM ppi_webhooks`).Scan(&webhookCount)
	require.NoError(t, err)
	require.Equal(t, workers, webhookCount)
}
