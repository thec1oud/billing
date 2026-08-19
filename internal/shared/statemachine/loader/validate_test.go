package loader

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
)

type fakeGuard struct{ name string }

func (g fakeGuard) Name() string { return g.name }
func (g fakeGuard) Evaluate(context.Context, model.ExecutionContext, json.RawMessage) (bool, error) {
	return true, nil
}

type fakeAction struct{ name string }

func (a fakeAction) Name() string { return a.name }
func (a fakeAction) Execute(context.Context, sqlcgen.DBTX, *model.ExecutionContext, json.RawMessage) error {
	return nil
}

func newTestRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	reg := registry.New()
	if err := reg.RegisterGuard(fakeGuard{name: "always_true"}); err != nil {
		t.Fatalf("register guard: %v", err)
	}
	if err := reg.RegisterAction(fakeAction{name: "noop"}); err != nil {
		t.Fatalf("register action: %v", err)
	}
	return reg
}

func validSpec() DefinitionSpec {
	return DefinitionSpec{
		MachineType:  "test_machine",
		Version:      1,
		InitialState: "DRAFT",
		Activate:     true,
		States: []StateSpec{
			{Name: "DRAFT"},
			{Name: "DONE", IsFinal: true},
		},
		Transitions: []TransitionSpec{
			{
				FromState: "DRAFT",
				EventName: "submit",
				ToState:   "DONE",
				Guards: []GuardBindingSpec{
					{Seq: 1, ImplementationKind: model.GuardImplGoRegistry, GuardName: "always_true"},
				},
				Actions: []ActionBindingSpec{
					{Seq: 1, ActionName: "noop", ParamsKind: model.ParamsKindStatic, Mode: model.ActionModeSync, OnError: model.OnErrorAbort},
				},
			},
		},
	}
}

func TestValidate_ValidSpec_NoError(t *testing.T) {
	if err := validate(validSpec(), newTestRegistry(t)); err != nil {
		t.Fatalf("expected valid spec to pass, got: %v", err)
	}
}

func TestValidate_MissingInitialState(t *testing.T) {
	spec := validSpec()
	spec.InitialState = "NOT_A_STATE"
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("expected ErrInvalidSpec, got %v", err)
	}
}

func TestValidate_TransitionReferencesUnknownState(t *testing.T) {
	spec := validSpec()
	spec.Transitions[0].ToState = "GHOST"
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("expected ErrInvalidSpec, got %v", err)
	}
}

func TestValidate_UnregisteredGuard(t *testing.T) {
	spec := validSpec()
	spec.Transitions[0].Guards[0].GuardName = "does_not_exist"
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, model.ErrUnregisteredGuard) {
		t.Fatalf("expected ErrUnregisteredGuard, got %v", err)
	}
}

func TestValidate_UnregisteredAction(t *testing.T) {
	spec := validSpec()
	spec.Transitions[0].Actions[0].ActionName = "does_not_exist"
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, model.ErrUnregisteredAction) {
		t.Fatalf("expected ErrUnregisteredAction, got %v", err)
	}
}

func TestValidate_ScriptGuardMissingScript(t *testing.T) {
	spec := validSpec()
	spec.Transitions[0].Guards[0] = GuardBindingSpec{Seq: 1, ImplementationKind: model.GuardImplScript}
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("expected ErrInvalidSpec for missing script, got %v", err)
	}
}

func TestValidate_ScriptActionParamsMissingScript(t *testing.T) {
	spec := validSpec()
	spec.Transitions[0].Actions[0].ParamsKind = model.ParamsKindScript
	spec.Transitions[0].Actions[0].ParamsScript = ""
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("expected ErrInvalidSpec for missing params_script, got %v", err)
	}
}

func TestValidate_DuplicateTransition(t *testing.T) {
	spec := validSpec()
	spec.Transitions = append(spec.Transitions, spec.Transitions[0])
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("expected ErrInvalidSpec for duplicate transition, got %v", err)
	}
}

func TestValidate_BrokenGuardScript_FailsCompile(t *testing.T) {
	spec := validSpec()
	spec.Transitions[0].Guards[0] = GuardBindingSpec{
		Seq:                1,
		ImplementationKind: model.GuardImplScript,
		Script:             "package script\n\nfunc Guard(context, payload, params map[string]any) bool {\n\treturn this is not valid go\n}",
	}
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("expected ErrInvalidSpec for a script that fails to compile, got %v", err)
	}
}

func TestValidate_ValidGuardScript_Passes(t *testing.T) {
	spec := validSpec()
	spec.Transitions[0].Guards[0] = GuardBindingSpec{
		Seq:                1,
		ImplementationKind: model.GuardImplScript,
		Script:             "package script\n\nfunc Guard(context, payload, params map[string]any) bool {\n\treturn true\n}",
	}
	if err := validate(spec, newTestRegistry(t)); err != nil {
		t.Fatalf("expected a valid guard script to pass validation, got %v", err)
	}
}

func TestValidate_DuplicateState(t *testing.T) {
	spec := validSpec()
	spec.States = append(spec.States, StateSpec{Name: "DRAFT"})
	err := validate(spec, newTestRegistry(t))
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("expected ErrInvalidSpec for duplicate state, got %v", err)
	}
}
