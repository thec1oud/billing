package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/outbox"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

type recordingAction struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (a *recordingAction) Name() string { return "recording" }
func (a *recordingAction) Execute(context.Context, sqlcgen.DBTX, *model.ExecutionContext, json.RawMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	return a.err
}

func (a *recordingAction) callCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls
}

func seedInstance(t *testing.T, ctx context.Context, repo *repository.PostgresRepository, machineType model.MachineType) model.Instance {
	t.Helper()
	def, err := repo.InsertDefinition(ctx, nil, machineType, 1, "A", true)
	if err != nil {
		t.Fatalf("insert definition: %v", err)
	}
	if err := repo.InsertState(ctx, nil, def.DefinitionID, model.State{Name: "A"}); err != nil {
		t.Fatalf("insert state: %v", err)
	}
	instance, err := repo.CreateInstance(ctx, nil, def.DefinitionID, machineType, "widget", "w-1", "A", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	return instance
}

func TestPublisher_Tick_ExecutesClaimedRowAndMarksPublished(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()
	action := &recordingAction{}
	if err := reg.RegisterAction(action); err != nil {
		t.Fatalf("register action: %v", err)
	}

	instance := seedInstance(t, ctx, repo, "publisher_tick_machine")
	if err := repo.InsertOutboxRow(ctx, nil, instance.InstanceID, nil, nil, "recording", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}

	pub := outbox.NewPublisher(pool, repo, reg, outbox.Config{BatchSize: 10})
	if err := pub.Tick(ctx); err != nil {
		t.Fatalf("tick: %v", err)
	}

	if action.callCount() != 1 {
		t.Fatalf("expected action to run once, got %d", action.callCount())
	}

	remaining, err := repo.ClaimOutboxRows(ctx, nil, 10, "verify")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected no PENDING rows left after publish, got %d", len(remaining))
	}
}

func TestPublisher_Tick_FailingAction_RetriesThenFails(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()
	action := &recordingAction{err: errors.New("boom")}
	if err := reg.RegisterAction(action); err != nil {
		t.Fatalf("register action: %v", err)
	}

	instance := seedInstance(t, ctx, repo, "publisher_retry_machine")
	if err := repo.InsertOutboxRow(ctx, nil, instance.InstanceID, nil, nil, "recording", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}

	const maxAttempts = 3
	pub := outbox.NewPublisher(pool, repo, reg, outbox.Config{BatchSize: 10, MaxAttempts: maxAttempts})

	for i := 0; i < maxAttempts; i++ {
		if err := pub.Tick(ctx); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}
	if action.callCount() != maxAttempts {
		t.Fatalf("expected action to run %d times, got %d", maxAttempts, action.callCount())
	}

	// After exhausting attempts the row must be FAILED (terminal) — no longer claimable.
	remaining, err := repo.ClaimOutboxRows(ctx, nil, 10, "verify-terminal")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected row to be terminal (FAILED) after %d attempts, but it's still claimable", maxAttempts)
	}

	// One more tick must be a no-op: nothing PENDING left to claim/execute.
	if err := pub.Tick(ctx); err != nil {
		t.Fatalf("tick after exhaustion: %v", err)
	}
	if action.callCount() != maxAttempts {
		t.Fatalf("expected no further execution after terminal FAILED, got %d calls", action.callCount())
	}
}

func TestPublisher_Reap_ReclaimsStaleClaim(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()

	instance := seedInstance(t, ctx, repo, "publisher_reap_machine")
	if err := repo.InsertOutboxRow(ctx, nil, instance.InstanceID, nil, nil, "recording", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}

	// Claim it directly (simulating a worker that crashed before the Publisher
	// could mark a terminal status).
	claimed, err := repo.ClaimOutboxRows(ctx, nil, 1, "crashed-worker")
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: rows=%d err=%v", len(claimed), err)
	}

	pub := outbox.NewPublisher(pool, repo, reg, outbox.Config{ClaimTTL: time.Millisecond})
	time.Sleep(5 * time.Millisecond)

	n, err := pub.Reap(ctx)
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row reaped, got %d", n)
	}

	reclaimed, err := repo.ClaimOutboxRows(ctx, nil, 1, "second-worker")
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("expected reaped row to be claimable again: rows=%d err=%v", len(reclaimed), err)
	}
}
