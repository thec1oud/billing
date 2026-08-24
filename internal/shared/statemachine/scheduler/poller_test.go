package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scheduler"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

func setup(t *testing.T) (*engine.Engine, *repository.PostgresRepository, model.Instance) {
	t.Helper()
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()
	eng := engine.NewEngine(pool, repo, reg)

	const machineType model.MachineType = "scheduler_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{FromState: "A", EventName: "timeout", ToState: "B"},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	return eng, repo, instance
}

func TestPoller_Tick_FiresDueScheduledTransition(t *testing.T) {
	ctx := context.Background()
	eng, repo, instance := setup(t)

	if _, err := repo.InsertScheduledTransition(ctx, nil, instance.InstanceID, "timeout", nil, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("insert scheduled transition: %v", err)
	}

	poller := scheduler.NewPoller(eng, repo, scheduler.Config{BatchSize: 10})
	if err := poller.Tick(ctx); err != nil {
		t.Fatalf("tick: %v", err)
	}

	final, err := repo.GetInstanceByID(ctx, nil, instance.InstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if final.CurrentState != "B" || final.Status != model.InstanceStatusCompleted {
		t.Fatalf("expected scheduled transition to fire (state=B, COMPLETED), got state=%s status=%s", final.CurrentState, final.Status)
	}
}

func TestPoller_Tick_NotYetDue_DoesNotFire(t *testing.T) {
	ctx := context.Background()
	eng, repo, instance := setup(t)

	if _, err := repo.InsertScheduledTransition(ctx, nil, instance.InstanceID, "timeout", nil, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("insert scheduled transition: %v", err)
	}

	poller := scheduler.NewPoller(eng, repo, scheduler.Config{BatchSize: 10})
	if err := poller.Tick(ctx); err != nil {
		t.Fatalf("tick: %v", err)
	}

	final, err := repo.GetInstanceByID(ctx, nil, instance.InstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if final.CurrentState != "A" {
		t.Fatalf("expected instance to remain in A (transition not yet due), got %s", final.CurrentState)
	}
}

func TestPoller_Tick_FailingFire_RetriesThenFails(t *testing.T) {
	ctx := context.Background()
	eng, repo, instance := setup(t)

	// "nonexistent" has no transition defined from A, so Fire() will always
	// return ErrNoValidTransition — a stand-in for "the scheduled event no
	// longer applies," exercising the retry/fail path without needing a
	// separately-registered failing action.
	if _, err := repo.InsertScheduledTransition(ctx, nil, instance.InstanceID, "nonexistent", nil, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("insert scheduled transition: %v", err)
	}

	const maxAttempts = 2
	poller := scheduler.NewPoller(eng, repo, scheduler.Config{BatchSize: 10, MaxAttempts: maxAttempts})

	for i := 0; i < maxAttempts; i++ {
		if err := poller.Tick(ctx); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}

	remaining, err := repo.ClaimScheduledTransitions(ctx, nil, 10, "verify-terminal")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected schedule row to be terminal (FAILED) after %d attempts, but it's still claimable", maxAttempts)
	}
}

func TestPoller_Reap_ReclaimsStaleClaim(t *testing.T) {
	ctx := context.Background()
	eng, repo, instance := setup(t)

	if _, err := repo.InsertScheduledTransition(ctx, nil, instance.InstanceID, "timeout", nil, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("insert scheduled transition: %v", err)
	}

	claimed, err := repo.ClaimScheduledTransitions(ctx, nil, 1, "crashed-worker")
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: rows=%d err=%v", len(claimed), err)
	}

	poller := scheduler.NewPoller(eng, repo, scheduler.Config{ClaimTTL: time.Millisecond})
	time.Sleep(5 * time.Millisecond)

	n, err := poller.Reap(ctx)
	if err != nil {
		t.Fatalf("reap: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row reaped, got %d", n)
	}

	reclaimed, err := repo.ClaimScheduledTransitions(ctx, nil, 1, "second-worker")
	if err != nil || len(reclaimed) != 1 {
		t.Fatalf("expected reaped row to be claimable again: rows=%d err=%v", len(reclaimed), err)
	}
}
