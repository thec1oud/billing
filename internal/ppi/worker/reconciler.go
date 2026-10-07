package worker

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/thec1oud/billing/internal/infra/logger"
	attemptmodel "github.com/thec1oud/billing/internal/payment_attempt/model"
	attemptrepo "github.com/thec1oud/billing/internal/payment_attempt/repository"
	attemptsrv "github.com/thec1oud/billing/internal/payment_attempt/service"
	"github.com/thec1oud/billing/internal/ppi"
	ppisvc "github.com/thec1oud/billing/internal/ppi/service"
)

var log = logger.ForComponent("ppi_reconciler")

// Config tunes the Reconciler's polling cadence and thresholds.
type Config struct {
	// Interval between reconciliation sweeps.
	Interval time.Duration
	// StaleThreshold is how old a PENDING attempt must be before it's considered stale.
	StaleThreshold time.Duration
	// BatchSize is the max number of stale attempts processed per sweep.
	BatchSize int
	// WorkerID is recorded on claimed rows for observability (not strictly needed yet).
	WorkerID string
}

func (c Config) withDefaults() Config {
	if c.Interval <= 0 {
		c.Interval = 1 * time.Minute
	}
	if c.StaleThreshold <= 0 {
		c.StaleThreshold = 15 * time.Minute
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.WorkerID == "" {
		host, _ := os.Hostname()
		c.WorkerID = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	return c
}

// Reconciler polls for stale PENDING payment attempts and resolves them.
// It uses the payment provider's adapter to verify the actual transaction
// status and updates the attempt to SUCCESS or FAILED accordingly.
type Reconciler struct {
	attemptRepo attemptrepo.Repository
	attemptSvc  *attemptsrv.Service
	ppiSvc      *ppisvc.Service
	cfg         Config

	stopCh chan struct{}
	doneCh chan struct{}
}

func NewReconciler(attemptRepo attemptrepo.Repository, attemptSvc *attemptsrv.Service, ppiSvc *ppisvc.Service, cfg Config) *Reconciler {
	return &Reconciler{
		attemptRepo: attemptRepo,
		attemptSvc:  attemptSvc,
		ppiSvc:      ppiSvc,
		cfg:         cfg.withDefaults(),
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
}

// Start runs the polling loop in a background goroutine until ctx is canceled
// or Stop is called.
func (r *Reconciler) Start(ctx context.Context) {
	go r.run(ctx)
}

// Stop signals the polling loop to exit and blocks until it has.
func (r *Reconciler) Stop() {
	close(r.stopCh)
	<-r.doneCh
}

func (r *Reconciler) run(ctx context.Context) {
	defer close(r.doneCh)

	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.Tick(ctx); err != nil {
				log.Error("reconciler sweep failed", logger.Err(err))
			}
		}
	}
}

// Tick performs one sweep of stale pending attempts. Exported for testing.
func (r *Reconciler) Tick(ctx context.Context) error {
	before := time.Now().Add(-r.cfg.StaleThreshold)

	attempts, err := r.attemptRepo.GetStalePendingAttempts(ctx, nil, before, int32(r.cfg.BatchSize))
	if err != nil {
		return fmt.Errorf("fetch stale pending attempts: %w", err)
	}

	if len(attempts) == 0 {
		return nil // Nothing to do
	}

	log.Info("Found stale pending payment attempts", slog.Int("count", len(attempts)))

	for _, attempt := range attempts {
		r.reconcileAttempt(ctx, attempt)
	}

	return nil
}

func (r *Reconciler) reconcileAttempt(ctx context.Context, attempt attemptmodel.PaymentAttempt) {
	log.Info("Reconciling stale attempt", slog.Int64("attempt_id", attempt.AttemptID))

	adapter, exists := r.ppiSvc.GetAdapter(attempt.ProviderCode)
	if !exists {
		log.Error("Provider adapter not found during reconciliation", slog.String("provider_code", attempt.ProviderCode))
		return
	}

	res, err := adapter.VerifyPayment(ctx, attempt.ProviderCode, attempt.InternalTxID, attempt.ProviderTxID)

	if err != nil {
		log.Warn(
			"VerifyPayment encountered a system/network error; leaving attempt PENDING for next sweep",
			slog.Int64("attempt_id", attempt.AttemptID),
			logger.Err(err),
		)
		return
	}

	status := attemptmodel.StatusFailed
	rawResp := res.RawResponse

	switch res.Status {
	case ppi.ChargeStatusSuccess:
		status = attemptmodel.StatusSuccess
	case ppi.ChargeStatusPending:
		// Still pending on provider side, wait for next sweep
		log.Info("Attempt is still pending on provider side", slog.Int64("attempt_id", attempt.AttemptID))
		return
	}

	var providerTxID *string
	if res.ProviderReference != "" {
		providerTxID = &res.ProviderReference
	}

	err = r.attemptSvc.UpdatePaymentAttemptResult(ctx, nil, attemptmodel.UpdatePaymentAttemptResultInput{
		AttemptID:    attempt.AttemptID,
		Status:       status,
		ProviderTxID: providerTxID,
		RawResponse:  rawResp,
	})

	if err != nil {
		log.Error("Failed to update stale payment attempt", slog.Int64("attempt_id", attempt.AttemptID), logger.Err(err))
	} else {
		log.Info("Successfully reconciled stale attempt", slog.Int64("attempt_id", attempt.AttemptID), slog.String("new_status", string(status)))
	}
}
