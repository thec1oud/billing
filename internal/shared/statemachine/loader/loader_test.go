package loader_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

type noopGuard struct{}

func (noopGuard) Name() string { return "always_true" }
func (noopGuard) Evaluate(ctx context.Context, ec model.ExecutionContext, params json.RawMessage) (bool, error) {
	return true, nil
}

type noopAction struct{}

func (noopAction) Name() string { return "noop" }
func (noopAction) Execute(ctx context.Context, db sqlcgen.DBTX, ec *model.ExecutionContext, params json.RawMessage) error {
	return nil
}

func TestPublish_ValidSpec_WritesQueryableDefinition(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()
	if err := reg.RegisterGuard(noopGuard{}); err != nil {
		t.Fatalf("register guard: %v", err)
	}
	if err := reg.RegisterAction(noopAction{}); err != nil {
		t.Fatalf("register action: %v", err)
	}

	spec := loader.DefinitionSpec{
		MachineType:  "loader_publish_machine",
		Version:      1,
		InitialState: "DRAFT",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "DRAFT"},
			{Name: "DONE", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{
				FromState: "DRAFT",
				EventName: "submit",
				ToState:   "DONE",
				Guards: []loader.GuardBindingSpec{
					{Seq: 1, ImplementationKind: model.GuardImplGoRegistry, GuardName: "always_true"},
				},
				Actions: []loader.ActionBindingSpec{
					{Seq: 1, ActionName: "noop", ParamsKind: model.ParamsKindStatic, Mode: model.ActionModeSync, OnError: model.OnErrorAbort},
				},
			},
		},
	}

	definitionID, err := loader.Publish(ctx, pool, repo, reg, spec)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	bundle, err := repo.GetDefinitionBundle(ctx, nil, definitionID)
	if err != nil {
		t.Fatalf("get definition bundle: %v", err)
	}
	if len(bundle.States) != 2 {
		t.Fatalf("expected 2 states, got %d", len(bundle.States))
	}
	if len(bundle.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(bundle.Transitions))
	}
	if len(bundle.Transitions[0].Guards) != 1 {
		t.Fatalf("expected 1 guard binding, got %d", len(bundle.Transitions[0].Guards))
	}
	if len(bundle.Actions) != 1 {
		t.Fatalf("expected 1 action binding, got %d", len(bundle.Actions))
	}

	active, err := repo.GetActiveDefinition(ctx, nil, "loader_publish_machine")
	if err != nil {
		t.Fatalf("get active definition: %v", err)
	}
	if active.DefinitionID != definitionID {
		t.Fatalf("expected active definition to be the published one")
	}
}

func TestPublish_InvalidSpec_WritesNothing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()

	spec := loader.DefinitionSpec{
		MachineType:  "loader_invalid_machine",
		Version:      1,
		InitialState: "DRAFT",
		States:       []loader.StateSpec{{Name: "DRAFT"}},
		Transitions: []loader.TransitionSpec{
			{FromState: "DRAFT", EventName: "submit", ToState: "GHOST"},
		},
	}

	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err == nil {
		t.Fatal("expected publish to fail validation")
	}

	if _, err := repo.GetActiveDefinition(ctx, nil, "loader_invalid_machine"); err != model.ErrDefinitionNotFound {
		t.Fatalf("expected no definition to have been written, got err=%v", err)
	}
}

func TestPublish_SecondActiveVersion_DeactivatesFirst(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()

	base := loader.DefinitionSpec{
		MachineType:  "loader_reactivate_machine",
		InitialState: "DRAFT",
		Activate:     true,
		States:       []loader.StateSpec{{Name: "DRAFT", IsFinal: true}},
	}

	v1 := base
	v1.Version = 1
	if _, err := loader.Publish(ctx, pool, repo, reg, v1); err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	v2 := base
	v2.Version = 2
	definitionIDv2, err := loader.Publish(ctx, pool, repo, reg, v2)
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	active, err := repo.GetActiveDefinition(ctx, nil, "loader_reactivate_machine")
	if err != nil {
		t.Fatalf("get active definition: %v", err)
	}
	if active.DefinitionID != definitionIDv2 || active.Version != 2 {
		t.Fatalf("expected v2 to be the sole active definition, got version=%d", active.Version)
	}
}
