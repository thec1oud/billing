//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	attemptRepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptSvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/ppi/service"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

type countingTestProvider struct {
	code        string
	chargeCalls atomic.Int32
	delay       time.Duration
}

func (p *countingTestProvider) ProviderCode() string { return p.code }

func (p *countingTestProvider) ChargePaymentMethod(
	ctx context.Context,
	amount money.Money,
	providerCode, idempotencyKey string,
) (ppi.ChargeResult, error) {
	n := p.chargeCalls.Add(1)
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	return ppi.ChargeResult{
		Status:            ppi.ChargeStatusSuccess,
		IdempotencyKey:    idempotencyKey,
		ProviderReference: fmt.Sprintf("prov_ref_%d", n),
	}, nil
}

func (p *countingTestProvider) VerifyPayment(
	ctx context.Context,
	providerCode, internalTxID string,
	providerTxID *string,
) (ppi.ChargeResult, error) {
	return ppi.ChargeResult{}, nil
}

func (p *countingTestProvider) ParseWebhook(r *http.Request) (ppi.ProviderWebhookPayload, error) {
	return ppi.ProviderWebhookPayload{}, nil
}

func (p *countingTestProvider) RespondWebhook(
	w http.ResponseWriter,
	r *http.Request,
	code ppi.WebhookResponseCode,
	payload ppi.ProviderWebhookPayload,
) {
}

func TestPaymentRetry_ConcurrentPaysDBConstraint_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cluster, cleanup, err := testutil.SetupTestCluster(ctx)
	require.NoError(t, err)
	defer cleanup()

	_, err = cluster.DBPool.Exec(ctx, `
		INSERT INTO payment_provider (payment_provider_code)
		VALUES ('fake')
		ON CONFLICT (payment_provider_code) DO NOTHING;
	`)
	require.NoError(t, err)

	var accountID int64
	err = cluster.DBPool.QueryRow(ctx, `
		INSERT INTO accounts (currency, timezone)
		VALUES ('ETB', 'Africa/Addis_Ababa')
		RETURNING account_id;
	`).Scan(&accountID)
	require.NoError(t, err)

	var invoiceID1 int64
	err = cluster.DBPool.QueryRow(ctx, `
		INSERT INTO invoices (account_id, currency, total_amount, amount_due)
		VALUES ($1, 'ETB', 15000, 15000)
		RETURNING invoice_id;
	`, accountID).Scan(&invoiceID1)
	require.NoError(t, err)

	attRepo := attemptRepo.NewPostgresRepository(cluster.DBPool)
	attService := attemptSvc.NewService(attRepo)
	ppiRepo := repository.NewPostgresRepository()

	amt, err := money.New(15000, "ETB")
	require.NoError(t, err)

	t.Run("distinct keys concurrently on same invoice produce exactly one provider charge", func(t *testing.T) {
		provider := &countingTestProvider{code: "fake", delay: 100 * time.Millisecond}
		ppiService := service.NewService(cluster.DBPool, ppiRepo, attService, nil)
		ppiService.RegisterAdapter(provider)

		const workers = 6
		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, workers)
		results := make(chan ppi.ChargeResult, workers)

		for i := range workers {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				res, callErr := ppiService.ChargePaymentMethod(
					ctx,
					"fake",
					invoiceID1,
					amt,
					fmt.Sprintf("concurrent_distinct_%d", idx),
				)
				if callErr != nil {
					errs <- callErr
					return
				}
				results <- res
			}(i)
		}

		close(start)
		wg.Wait()
		close(errs)
		close(results)

		require.Equal(t, int32(1), provider.chargeCalls.Load(), "DB partial unique index must enforce a single provider charge")
		require.Len(t, results, 1)
		for callErr := range errs {
			require.True(t, errors.Is(callErr, attemptRepo.ErrPendingAttemptExists), "unexpected error: %v", callErr)
		}
	})

	t.Run("same key concurrently on invoice replays cleanly with one provider charge", func(t *testing.T) {
		var invoiceID2 int64
		err = cluster.DBPool.QueryRow(ctx, `
			INSERT INTO invoices (account_id, currency, total_amount, amount_due)
			VALUES ($1, 'ETB', 15000, 15000)
			RETURNING invoice_id;
		`, accountID).Scan(&invoiceID2)
		require.NoError(t, err)

		provider := &countingTestProvider{code: "fake", delay: 100 * time.Millisecond}
		ppiService := service.NewService(cluster.DBPool, ppiRepo, attService, nil)
		ppiService.RegisterAdapter(provider)

		const workers = 5
		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make(chan error, workers)

		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, callErr := ppiService.ChargePaymentMethod(ctx, "fake", invoiceID2, amt, "concurrent_same_key_1")
				if callErr != nil {
					errs <- callErr
				}
			}()
		}

		close(start)
		wg.Wait()
		close(errs)

		require.Equal(t, int32(1), provider.chargeCalls.Load(), "same key concurrently must charge provider once")
		for callErr := range errs {
			t.Fatalf("expected nil error on concurrent same-key replay, got %v", callErr)
		}
	})
}
