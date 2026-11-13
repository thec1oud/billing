//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	attemptRepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptSvc "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi/adapters/fake"
	"github.com/thec1oud/billing/internal/ppi/repository"
	"github.com/thec1oud/billing/internal/ppi/service"
	"github.com/thec1oud/billing/internal/ppi/worker"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func TestReconciler_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Spin up Postgres via TestCluster
	cluster, cleanup, err := testutil.SetupTestCluster(ctx)
	require.NoError(t, err, "failed to setup postgres test cluster")
	defer cleanup()

	// 2. Spin up Redis container directly
	redisReq := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: redisReq,
		Started:          true,
	})
	require.NoError(t, err, "failed to start redis container")
	defer func() {
		_ = redisContainer.Terminate(context.Background())
	}()

	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisHost + ":" + redisPort.Port(),
	})
	defer redisClient.Close()

	// 3. Wire up dependencies
	paymentAttemptRepo := attemptRepo.NewPostgresRepository(cluster.DBPool)
	paymentAttemptSvc := attemptSvc.NewService(paymentAttemptRepo)

	ppiRepo := repository.NewPostgresRepository()
	ppiService := service.NewService(cluster.DBPool, ppiRepo, paymentAttemptSvc, nil)
	ppiService.RegisterAdapter(fake.NewFakeAdapter())

	// Reconciler with 15-minute stale threshold
	cfg := worker.Config{
		StaleThreshold: 15 * time.Minute,
		BatchSize:      10,
	}
	reconciler := worker.NewReconciler(paymentAttemptRepo, paymentAttemptSvc, ppiService, redisClient, cfg)

	// 4. Seed basic database schema (Provider, Account)
	_, err = cluster.DBPool.Exec(ctx, `
		INSERT INTO payment_provider (payment_provider_code)
		VALUES ('fake') ON CONFLICT DO NOTHING;
	`)
	require.NoError(t, err)

	var accountID int64
	err = cluster.DBPool.QueryRow(ctx, `
		INSERT INTO accounts (currency, timezone) VALUES ('ETB', 'Africa/Addis_Ababa') RETURNING account_id;
	`).Scan(&accountID)
	require.NoError(t, err)

	// Helper to seed a stale PENDING attempt
	seedStaleAttempt := func(txID string) int64 {
		var invoiceID int64
		err := cluster.DBPool.QueryRow(ctx, `
			INSERT INTO invoices (account_id, currency, total_amount, amount_due) VALUES ($1, 'ETB', 15000, 15000) RETURNING invoice_id;
		`, accountID).Scan(&invoiceID)
		require.NoError(t, err)

		_, err = cluster.DBPool.Exec(ctx, `
			INSERT INTO payment_attempts (invoice_id, provider_code, internal_tx_id, amount_minor, currency, status, updated_at)
			VALUES ($1, 'fake', $2, 1000, 'ETB', 'PENDING', NOW() - INTERVAL '30 minutes')
		`, invoiceID, txID)
		require.NoError(t, err)
		return invoiceID
	}

	// 5. Test Scenarios
	t.Run("Reconcile_Success", func(t *testing.T) {
		internalTxID := fmt.Sprintf("success_verify_%d", time.Now().UnixNano())
		seedStaleAttempt(internalTxID)

		err := reconciler.Tick(ctx)
		require.NoError(t, err)

		var status string
		err = cluster.DBPool.QueryRow(ctx, "SELECT status FROM payment_attempts WHERE internal_tx_id = $1", internalTxID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "SUCCESS", status)
	})

	t.Run("Reconcile_Failed", func(t *testing.T) {
		internalTxID := fmt.Sprintf("fail_verify_%d", time.Now().UnixNano())
		seedStaleAttempt(internalTxID)

		err := reconciler.Tick(ctx)
		require.NoError(t, err)

		var status string
		err = cluster.DBPool.QueryRow(ctx, "SELECT status FROM payment_attempts WHERE internal_tx_id = $1", internalTxID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "FAILED", status)
	})

	t.Run("Reconcile_Pending", func(t *testing.T) {
		internalTxID := fmt.Sprintf("pending_verify_%d", time.Now().UnixNano())
		seedStaleAttempt(internalTxID)

		err := reconciler.Tick(ctx)
		require.NoError(t, err)

		var status string
		var updatedAt time.Time
		err = cluster.DBPool.QueryRow(ctx, "SELECT status, updated_at FROM payment_attempts WHERE internal_tx_id = $1", internalTxID).Scan(&status, &updatedAt)
		require.NoError(t, err)
		require.Equal(t, "PENDING", status)
		// updated_at should have been refreshed
		require.WithinDuration(t, time.Now(), updatedAt, 2*time.Minute)
	})

	t.Run("Skip_Not_Stale", func(t *testing.T) {
		var invoiceID int64
		err := cluster.DBPool.QueryRow(ctx, `
			INSERT INTO invoices (account_id, currency, total_amount, amount_due) VALUES ($1, 'ETB', 15000, 15000) RETURNING invoice_id;
		`, accountID).Scan(&invoiceID)
		require.NoError(t, err)

		internalTxID := fmt.Sprintf("not_stale_%d", time.Now().UnixNano())
		_, err = cluster.DBPool.Exec(ctx, `
			INSERT INTO payment_attempts (invoice_id, provider_code, internal_tx_id, amount_minor, currency, status, updated_at)
			VALUES ($1, 'fake', $2, 1000, 'ETB', 'PENDING', NOW())
		`, invoiceID, internalTxID)
		require.NoError(t, err)

		err = reconciler.Tick(ctx)
		require.NoError(t, err)

		var status string
		err = cluster.DBPool.QueryRow(ctx, "SELECT status FROM payment_attempts WHERE internal_tx_id = $1", internalTxID).Scan(&status)
		require.NoError(t, err)
		require.Equal(t, "PENDING", status)
	})

	t.Run("Lock_Contention", func(t *testing.T) {
		lockKey := "billing:ppi:reconciler:lock"
		ok, err := redisClient.SetNX(ctx, lockKey, "other-worker", time.Minute).Result()
		require.NoError(t, err)
		require.True(t, ok)

		internalTxID := fmt.Sprintf("locked_%d", time.Now().UnixNano())
		seedStaleAttempt(internalTxID)

		err = reconciler.Tick(ctx)
		require.NoError(t, err)

		var status string
		var updatedAt time.Time
		err = cluster.DBPool.QueryRow(ctx, "SELECT status, updated_at FROM payment_attempts WHERE internal_tx_id = $1", internalTxID).Scan(&status, &updatedAt)
		require.NoError(t, err)
		require.Equal(t, "PENDING", status)

		// The updated_at should remain stale (30 mins ago) since it was never processed
		require.True(t, time.Now().Sub(updatedAt) > 25*time.Minute, "updated_at should be unmodified")

		// Cleanup lock
		redisClient.Del(ctx, lockKey)
	})

	t.Run("Concurrent_Workers", func(t *testing.T) {
		// 1. Setup two reconcilers representing two separate server instances
		cfg1 := cfg
		cfg1.WorkerID = "worker-1"
		r1 := worker.NewReconciler(paymentAttemptRepo, paymentAttemptSvc, ppiService, redisClient, cfg1)

		cfg2 := cfg
		cfg2.WorkerID = "worker-2"
		r2 := worker.NewReconciler(paymentAttemptRepo, paymentAttemptSvc, ppiService, redisClient, cfg2)

		// 2. Seed a batch of stale attempts
		for i := 0; i < 5; i++ {
			// using the success_verify_ prefix so they are successfully reconciled
			txID := fmt.Sprintf("success_verify_concurrent_%d_%d", time.Now().UnixNano(), i)
			seedStaleAttempt(txID)
		}

		// 3. Fire them at the exact same time using a WaitGroup
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			_ = r1.Tick(ctx)
		}()

		go func() {
			defer wg.Done()
			_ = r2.Tick(ctx)
		}()

		wg.Wait()

		// 4. Verify all were processed successfully.
		// Even though both workers fired at the same time, the Redis lock ensures
		// one sweeps the DB while the other skips, preventing duplicate processing.
		var count int
		err = cluster.DBPool.QueryRow(ctx, "SELECT COUNT(*) FROM payment_attempts WHERE internal_tx_id LIKE 'success_verify_concurrent_%' AND status = 'SUCCESS'").Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 5, count)
	})
}
