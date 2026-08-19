// Package outbox is the background worker side of the transactional outbox:
// engine.Fire enqueues ASYNC action rows inside its DB transaction (see the
// engine package), and Publisher here claims and executes them afterward,
// outside any transaction, since the whole point of deferring to the outbox is
// to avoid holding a DB lock across network I/O.
package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
)

// Config tunes a Publisher's polling cadence and retry policy. Zero values are
// replaced with sane defaults by NewPublisher.
type Config struct {
	// Interval between claim polls.
	Interval time.Duration
	// ClaimTTL is both the reaper's polling cadence and the age past which a
	// PROCESSING row is considered abandoned by a crashed worker and reclaimed.
	// Must exceed the realistic max execution time of any registered Action.
	ClaimTTL time.Duration
	// BatchSize is the max rows claimed per poll.
	BatchSize int
	// MaxAttempts is how many claims a row gets before moving to FAILED instead
	// of back to PENDING.
	MaxAttempts int
	// WorkerID is recorded on claimed rows for observability. Defaults to
	// hostname-pid.
	WorkerID string
}

func (c Config) withDefaults() Config {
	if c.Interval <= 0 {
		c.Interval = 2 * time.Second
	}
	if c.ClaimTTL <= 0 {
		c.ClaimTTL = 5 * time.Minute
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 20
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 5
	}
	if c.WorkerID == "" {
		host, _ := os.Hostname()
		c.WorkerID = fmt.Sprintf("%s-%d", host, os.Getpid())
	}
	return c
}

// Publisher polls sm_action_outbox for PENDING rows, claims them atomically
// (repository.ClaimOutboxRows — see that method's docs for why claiming must be
// a single statement rather than an app-level transaction), and executes each
// claimed row's registered Action outside any DB transaction.
type Publisher struct {
	pool     *pgxpool.Pool
	repo     *repository.PostgresRepository
	registry *registry.Registry
	cfg      Config

	stopCh chan struct{}
	doneCh chan struct{}
}

func NewPublisher(pool *pgxpool.Pool, repo *repository.PostgresRepository, reg *registry.Registry, cfg Config) *Publisher {
	return &Publisher{
		pool:     pool,
		repo:     repo,
		registry: reg,
		cfg:      cfg.withDefaults(),
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start runs the polling loop in a background goroutine until ctx is canceled
// or Stop is called.
func (p *Publisher) Start(ctx context.Context) {
	go p.run(ctx)
}

// Stop signals the polling loop to exit and blocks until it has.
func (p *Publisher) Stop() {
	close(p.stopCh)
	<-p.doneCh
}

func (p *Publisher) run(ctx context.Context) {
	defer close(p.doneCh)

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()
	reapTicker := time.NewTicker(p.cfg.ClaimTTL)
	defer reapTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			if err := p.Tick(ctx); err != nil {
				slog.Error("outbox publisher tick failed", "error", err)
			}
		case <-reapTicker.C:
			if _, err := p.Reap(ctx); err != nil {
				slog.Error("outbox publisher reap failed", "error", err)
			}
		}
	}
}

// Tick claims one batch of PENDING rows and executes each. Exported so tests
// (and callers wanting synchronous control) can drive it directly instead of
// waiting on the ticker loop.
func (p *Publisher) Tick(ctx context.Context) error {
	rows, err := p.repo.ClaimOutboxRows(ctx, nil, p.cfg.BatchSize, p.cfg.WorkerID)
	if err != nil {
		return fmt.Errorf("claim outbox rows: %w", err)
	}
	for _, row := range rows {
		p.execute(ctx, row)
	}
	return nil
}

// Reap reclaims PROCESSING rows whose claim is older than ClaimTTL (a worker
// crashed after claiming but before writing a terminal status), moving them
// back to PENDING.
func (p *Publisher) Reap(ctx context.Context) (int64, error) {
	return p.repo.ReapStuckOutbox(ctx, nil, time.Now().Add(-p.cfg.ClaimTTL))
}

func (p *Publisher) execute(ctx context.Context, row repository.OutboxRow) {
	action, ok := p.registry.LookupAction(row.ActionName)
	if !ok {
		p.terminal(ctx, row, fmt.Errorf("action %q is not registered", row.ActionName))
		return
	}

	ec := &model.ExecutionContext{
		InstanceID: row.InstanceID,
		Now:        time.Now(),
	}
	// db is the pool, not a transaction: ASYNC actions execute outside any
	// Fire() transaction by design (see engine.runActionHooks docs).
	if err := action.Execute(ctx, p.pool, ec, row.Params); err != nil {
		p.terminal(ctx, row, err)
		return
	}

	if err := p.repo.MarkOutboxPublished(ctx, nil, row.OutboxID); err != nil {
		slog.Error("mark outbox published failed", "outbox_id", row.OutboxID, "error", err)
	}
}

func (p *Publisher) terminal(ctx context.Context, row repository.OutboxRow, cause error) {
	if row.Attempts >= p.cfg.MaxAttempts {
		if err := p.repo.MarkOutboxFailed(ctx, nil, row.OutboxID, cause.Error()); err != nil {
			slog.Error("mark outbox failed failed", "outbox_id", row.OutboxID, "error", err)
		}
		return
	}
	if err := p.repo.MarkOutboxRetry(ctx, nil, row.OutboxID, cause.Error()); err != nil {
		slog.Error("mark outbox retry failed", "outbox_id", row.OutboxID, "error", err)
	}
}
