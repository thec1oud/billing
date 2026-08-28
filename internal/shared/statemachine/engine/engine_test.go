package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

// --- fakes ---

type constGuard struct {
	name   string
	result bool
}

func (g constGuard) Name() string { return g.name }
func (g constGuard) Evaluate(context.Context, model.ExecutionContext, json.RawMessage) (bool, error) {
	return g.result, nil
}

type incrementAction struct{}

func (incrementAction) Name() string { return "increment" }
func (incrementAction) Execute(_ context.Context, _ sqlcgen.DBTX, ec *model.ExecutionContext, _ json.RawMessage) error {
	data := map[string]any{}
	if len(ec.Context) > 0 {
		if err := json.Unmarshal(ec.Context, &data); err != nil {
			return err
		}
	}
	count, _ := data["count"].(float64)
	data["count"] = count + 1
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	ec.Context = b
	return nil
}

type failingAction struct{ err error }

func (a failingAction) Name() string { return "failing" }
func (a failingAction) Execute(context.Context, sqlcgen.DBTX, *model.ExecutionContext, json.RawMessage) error {
	return a.err
}

func newTestEngine(t *testing.T) (*engine.Engine, *pgxpool.Pool, *repository.PostgresRepository, *registry.Registry) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()

	for _, g := range []registry.Guard{
		constGuard{name: "always_true", result: true},
		constGuard{name: "always_false", result: false},
	} {
		if err := reg.RegisterGuard(g); err != nil {
			t.Fatalf("register guard: %v", err)
		}
	}
	for _, a := range []registry.Action{
		incrementAction{},
		failingAction{err: errors.New("boom")},
	} {
		if err := reg.RegisterAction(a); err != nil {
			t.Fatalf("register action: %v", err)
		}
	}

	return engine.NewEngine(pool, repo, reg), pool, repo, reg
}

func TestCreateInstanceAndFire_SyncActionsAndContextPersist(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()
	if err := reg.RegisterAction(incrementAction{}); err != nil {
		t.Fatalf("register action: %v", err)
	}
	eng := engine.NewEngine(pool, repo, reg)

	const machineType model.MachineType = "sync_action_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "DRAFT",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "DRAFT", OnEnter: []loader.ActionBindingSpec{
				{Seq: 1, ActionName: "increment", ParamsKind: model.ParamsKindStatic, Mode: model.ActionModeSync, OnError: model.OnErrorAbort},
			}},
			{Name: "DONE", IsFinal: true, OnEnter: []loader.ActionBindingSpec{
				{Seq: 1, ActionName: "increment", ParamsKind: model.ParamsKindStatic, Mode: model.ActionModeSync, OnError: model.OnErrorAbort},
			}},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "DRAFT", EventName: "submit", ToState: "DONE",
				Actions: []loader.ActionBindingSpec{
					{Seq: 1, ActionName: "increment", ParamsKind: model.ParamsKindStatic, Mode: model.ActionModeSync, OnError: model.OnErrorAbort},
				},
			},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if instance.CurrentState != "DRAFT" {
		t.Fatalf("expected DRAFT, got %s", instance.CurrentState)
	}
	assertCount(t, instance.Context, 1) // DRAFT's ON_ENTER ran once

	result, err := eng.Fire(ctx, instance.InstanceID, "submit", nil)
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	if result.ToState != "DONE" || !result.Completed {
		t.Fatalf("expected DONE/completed, got %+v", result)
	}

	final, err := repo.GetInstanceByID(ctx, nil, instance.InstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if final.Status != model.InstanceStatusCompleted {
		t.Fatalf("expected COMPLETED status, got %s", final.Status)
	}
	// ON_TRANSITION increment + DONE's ON_ENTER increment = 2 more, on top of the 1 from DRAFT's ON_ENTER.
	assertCount(t, final.Context, 3)
}

func assertCount(t *testing.T, ctxJSON json.RawMessage, want float64) {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal(ctxJSON, &data); err != nil {
		t.Fatalf("unmarshal context: %v", err)
	}
	got, _ := data["count"].(float64)
	if got != want {
		t.Fatalf("expected count=%v, got %v (context=%s)", want, got, ctxJSON)
	}
}

func TestFire_GuardFalse_FallsThroughToNextPriorityCandidate(t *testing.T) {
	ctx := context.Background()
	eng, pool, repo, reg := newTestEngine(t)
	_ = reg

	const machineType model.MachineType = "guard_fallthrough_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true}, {Name: "C", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "A", EventName: "go", ToState: "B", Priority: 0,
				Guards: []loader.GuardBindingSpec{{Seq: 1, ImplementationKind: model.GuardImplGoRegistry, GuardName: "always_false"}},
			},
			{
				FromState: "A", EventName: "go", ToState: "C", Priority: 1,
				Guards: []loader.GuardBindingSpec{{Seq: 1, ImplementationKind: model.GuardImplGoRegistry, GuardName: "always_true"}},
			},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	result, err := eng.Fire(ctx, instance.InstanceID, "go", nil)
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	if result.ToState != "C" {
		t.Fatalf("expected fallthrough to C (priority 0's guard fails), got %s", result.ToState)
	}
}

func TestFire_PriorityOrder_LowerPriorityWinsWhenBothPass(t *testing.T) {
	ctx := context.Background()
	eng, pool, repo, reg := newTestEngine(t)

	const machineType model.MachineType = "guard_priority_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true}, {Name: "C", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "A", EventName: "go", ToState: "C", Priority: 1,
				Guards: []loader.GuardBindingSpec{{Seq: 1, ImplementationKind: model.GuardImplGoRegistry, GuardName: "always_true"}},
			},
			{
				FromState: "A", EventName: "go", ToState: "B", Priority: 0,
				Guards: []loader.GuardBindingSpec{{Seq: 1, ImplementationKind: model.GuardImplGoRegistry, GuardName: "always_true"}},
			},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	result, err := eng.Fire(ctx, instance.InstanceID, "go", nil)
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	if result.ToState != "B" {
		t.Fatalf("expected priority 0 (B) to win, got %s", result.ToState)
	}
}

func TestFire_NoMatchingTransition_ReturnsErrNoValidTransition(t *testing.T) {
	ctx := context.Background()
	eng, pool, repo, reg := newTestEngine(t)

	const machineType model.MachineType = "no_match_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States:       []loader.StateSpec{{Name: "A"}},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	_, err = eng.Fire(ctx, instance.InstanceID, "nonexistent", nil)
	if !errors.Is(err, model.ErrNoValidTransition) {
		t.Fatalf("expected ErrNoValidTransition, got %v", err)
	}
}

func TestFire_OnTerminalInstance_ReturnsErrInstanceTerminal(t *testing.T) {
	ctx := context.Background()
	eng, pool, repo, reg := newTestEngine(t)

	const machineType model.MachineType = "terminal_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{FromState: "A", EventName: "go", ToState: "B"},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if _, err := eng.Fire(ctx, instance.InstanceID, "go", nil); err != nil {
		t.Fatalf("first fire: %v", err)
	}

	_, err = eng.Fire(ctx, instance.InstanceID, "go", nil)
	if !errors.Is(err, model.ErrInstanceTerminal) {
		t.Fatalf("expected ErrInstanceTerminal, got %v", err)
	}
}

func TestFire_SyncActionAbortsOnError_RollsBackWholeTransition(t *testing.T) {
	ctx := context.Background()
	eng, pool, repo, reg := newTestEngine(t)

	const machineType model.MachineType = "abort_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "A", EventName: "go", ToState: "B",
				Actions: []loader.ActionBindingSpec{
					{Seq: 1, ActionName: "failing", ParamsKind: model.ParamsKindStatic, Mode: model.ActionModeSync, OnError: model.OnErrorAbort},
				},
			},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if _, err := eng.Fire(ctx, instance.InstanceID, "go", nil); err == nil {
		t.Fatal("expected fire to fail due to aborting action")
	}

	after, err := repo.GetInstanceByID(ctx, nil, instance.InstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if after.CurrentState != "A" || after.Status != model.InstanceStatusRunning {
		t.Fatalf("expected instance to remain in state A/RUNNING after rollback, got state=%s status=%s", after.CurrentState, after.Status)
	}
}

func TestFire_AsyncAction_EnqueuesOutboxRowWithoutExecuting(t *testing.T) {
	ctx := context.Background()
	eng, pool, repo, reg := newTestEngine(t)

	const machineType model.MachineType = "async_action_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "A", EventName: "go", ToState: "B",
				Actions: []loader.ActionBindingSpec{
					{Seq: 1, ActionName: "increment", ParamsKind: model.ParamsKindStatic, Mode: model.ActionModeAsync, OnError: model.OnErrorAbort},
				},
			},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if _, err := eng.Fire(ctx, instance.InstanceID, "go", nil); err != nil {
		t.Fatalf("fire: %v", err)
	}

	// increment must NOT have run synchronously (context untouched by it).
	final, err := repo.GetInstanceByID(ctx, nil, instance.InstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	assertCount(t, final.Context, 0)

	claimed, err := repo.ClaimOutboxRows(ctx, nil, 10, "test-worker")
	if err != nil {
		t.Fatalf("claim outbox rows: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ActionName != "increment" {
		t.Fatalf("expected exactly one enqueued 'increment' outbox row, got %+v", claimed)
	}
}

const scriptedGuardSource = `
package script

func Guard(context, payload, params map[string]any) bool {
	amount, _ := context["amount"].(float64)
	limit, _ := params["limit"].(float64)
	return amount <= limit
}
`

const scriptedParamsSource = `
package script

func Params(context, payload, params map[string]any) map[string]any {
	return map[string]any{"computed_from": context["amount"]}
}
`

type recordingAction struct {
	mu     sync.Mutex
	params []json.RawMessage
}

func (a *recordingAction) Name() string { return "recording" }
func (a *recordingAction) Execute(_ context.Context, _ sqlcgen.DBTX, _ *model.ExecutionContext, params json.RawMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.params = append(a.params, params)
	return nil
}

func TestFire_ScriptedGuardAndScriptedActionParams_WorksEndToEnd(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()
	rec := &recordingAction{}
	if err := reg.RegisterAction(rec); err != nil {
		t.Fatalf("register action: %v", err)
	}
	eng := engine.NewEngine(pool, repo, reg, engine.WithScripting(scripting.NewPool(4, 200*time.Millisecond)))

	const machineType model.MachineType = "scripted_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "A", EventName: "go", ToState: "B",
				Guards: []loader.GuardBindingSpec{
					{Seq: 1, ImplementationKind: model.GuardImplScript, Script: scriptedGuardSource, Params: json.RawMessage(`{"limit": 100}`)},
				},
				Actions: []loader.ActionBindingSpec{
					//nolint:lll // Kept together for readability.
					{Seq: 1, ActionName: "recording", ParamsKind: model.ParamsKindScript, ParamsScript: scriptedParamsSource, Mode: model.ActionModeSync, OnError: model.OnErrorAbort},
				},
			},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", json.RawMessage(`{"amount": 42}`))
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	result, err := eng.Fire(ctx, instance.InstanceID, "go", nil)
	if err != nil {
		t.Fatalf("fire: %v", err)
	}
	if result.ToState != "B" {
		t.Fatalf("expected scripted guard to pass (amount 42 <= limit 100) and land on B, got %s", result.ToState)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.params) != 1 {
		t.Fatalf("expected the recording action to run exactly once, got %d calls", len(rec.params))
	}
	var decoded map[string]any
	if err := json.Unmarshal(rec.params[0], &decoded); err != nil {
		t.Fatalf("unmarshal recorded params: %v", err)
	}
	if decoded["computed_from"] != float64(42) {
		t.Fatalf("expected script-computed params {computed_from: 42}, got %v", decoded)
	}
}

func TestFire_ScriptedGuardRejects_FallsBackToNoValidTransition(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()
	eng := engine.NewEngine(pool, repo, reg, engine.WithScripting(scripting.NewPool(4, 200*time.Millisecond)))

	const machineType model.MachineType = "scripted_reject_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "A", EventName: "go", ToState: "B",
				Guards: []loader.GuardBindingSpec{
					{Seq: 1, ImplementationKind: model.GuardImplScript, Script: scriptedGuardSource, Params: json.RawMessage(`{"limit": 10}`)},
				},
			},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", json.RawMessage(`{"amount": 42}`))
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	_, err = eng.Fire(ctx, instance.InstanceID, "go", nil)
	if !errors.Is(err, model.ErrNoValidTransition) {
		t.Fatalf("expected ErrNoValidTransition (amount 42 > limit 10), got %v", err)
	}
}

func TestFire_ConcurrentFireOnSameInstance_ExactlyOneSucceeds(t *testing.T) {
	ctx := context.Background()
	eng, pool, repo, reg := newTestEngine(t)

	const machineType model.MachineType = "concurrent_fire_machine"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "A",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "A"}, {Name: "B", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{FromState: "A", EventName: "go", ToState: "B"},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	instance, err := eng.CreateInstance(ctx, machineType, "widget", "w-1", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	const workers = 10
	var wg sync.WaitGroup
	successes := make([]bool, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := eng.Fire(ctx, instance.InstanceID, "go", nil)
			successes[i] = err == nil
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, ok := range successes {
		if ok {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful fire out of %d concurrent attempts, got %d", workers, successCount)
	}

	final, err := repo.GetInstanceByID(ctx, nil, instance.InstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	// version=2: CreateInstance's own persist step (running ON_ENTER hooks) bumps
	// version 0->1, then the single successful Fire() bumps 1->2. A second,
	// wrongly-applied Fire() would make this 3.
	if final.CurrentState != "B" || final.Version != 2 {
		t.Fatalf("expected exactly one transition applied (state=B, version=2), got state=%s version=%d", final.CurrentState, final.Version)
	}
}
