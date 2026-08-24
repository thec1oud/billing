// Package scheduler is the background worker for timer-driven transitions:
// sm_scheduled_transitions rows with a fire_at timestamp are claimed and fired
// through the same engine.Fire path any other trigger uses — one source of truth
// for transition legality, guards, and hooks, regardless of what triggered it.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
)

// Config tunes a Poller's polling cadence and retry policy. Zero values are
// replaced with sane defaults by NewPoller.
type Config struct {
	// Interval between claim polls.
	Interval time.Duration
	// ClaimTTL is both the reaper's polling cadence and the age past which a
	// PROCESSING row is considered abandoned by a crashed worker and reclaimed.
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

// Poller polls sm_scheduled_transitions for due (fire_at <= now()) PENDING rows,
// claims them atomically (repository.ClaimScheduledTransitions), and fires each
// through engine.Fire outside any claim-holding transaction.
type Poller struct {
	engine *engine.Engine
	repo   *repository.PostgresRepository
	cfg    Config

	stopCh chan struct{}
	doneCh chan struct{}
}

func NewPoller(eng *engine.Engine, repo *repository.PostgresRepository, cfg Config) *Poller {
	return &Poller{
		engine: eng,
		repo:   repo,
		cfg:    cfg.withDefaults(),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start runs the polling loop in a background goroutine until ctx is canceled
// or Stop is called.
func (p *Poller) Start(ctx context.Context) {
	go p.run(ctx)
}

// Stop signals the polling loop to exit and blocks until it has.
func (p *Poller) Stop() {
	close(p.stopCh)
	<-p.doneCh
}

func (p *Poller) run(ctx context.Context) {
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
				slog.Error("scheduler poller tick failed", "error", err)
			}
		case <-reapTicker.C:
			if _, err := p.Reap(ctx); err != nil {
				slog.Error("scheduler poller reap failed", "error", err)
			}
		}
	}
}

// Tick claims one batch of due rows and fires each. Exported so tests (and
// callers wanting synchronous control) can drive it directly instead of waiting
// on the ticker loop.
func (p *Poller) Tick(ctx context.Context) error {
	rows, err := p.repo.ClaimScheduledTransitions(ctx, nil, p.cfg.BatchSize, p.cfg.WorkerID)
	if err != nil {
		return fmt.Errorf("claim scheduled transitions: %w", err)
	}
	for _, row := range rows {
		p.fire(ctx, row)
	}
	return nil
}

// Reap reclaims PROCESSING rows whose claim is older than ClaimTTL, moving them
// back to PENDING.
func (p *Poller) Reap(ctx context.Context) (int64, error) {
	return p.repo.ReapStuckSchedules(ctx, nil, time.Now().Add(-p.cfg.ClaimTTL))
}

func (p *Poller) fire(ctx context.Context, row repository.ScheduleRow) {
	_, err := p.engine.Fire(ctx, row.InstanceID, model.EventName(row.EventName), row.EventPayload, engine.WithTriggeredBy("scheduler"))
	if err != nil {
		if row.Attempts >= p.cfg.MaxAttempts {
			if merr := p.repo.MarkScheduleFailed(ctx, nil, row.ScheduleID, err.Error()); merr != nil {
				slog.Error("mark schedule failed failed", "schedule_id", row.ScheduleID, "error", merr)
			}
			return
		}
		if merr := p.repo.MarkScheduleRetry(ctx, nil, row.ScheduleID, err.Error()); merr != nil {
			slog.Error("mark schedule retry failed", "schedule_id", row.ScheduleID, "error", merr)
		}
		return
	}

	if err := p.repo.MarkScheduleFired(ctx, nil, row.ScheduleID); err != nil {
		slog.Error("mark schedule fired failed", "schedule_id", row.ScheduleID, "error", err)
	}
}
