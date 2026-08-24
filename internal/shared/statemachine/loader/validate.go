package loader

import (
	"errors"
	"fmt"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
)

var ErrInvalidSpec = errors.New("invalid state machine definition spec")

type transitionKey struct {
	from     model.StateName
	event    model.EventName
	priority int
}

// validate checks referential integrity (every from/to state exists, every
// guard/action name resolves in reg) and structural well-formedness before
// Publish writes anything. This is what lets a broken definition fail fast at
// publish time instead of at first Fire().
func validate(spec DefinitionSpec, reg *registry.Registry) error {
	if spec.MachineType == "" {
		return fmt.Errorf("%w: machine_type is required", ErrInvalidSpec)
	}
	if spec.Version <= 0 {
		return fmt.Errorf("%w: version must be positive", ErrInvalidSpec)
	}
	if spec.InitialState == "" {
		return fmt.Errorf("%w: initial_state is required", ErrInvalidSpec)
	}
	if len(spec.States) == 0 {
		return fmt.Errorf("%w: at least one state is required", ErrInvalidSpec)
	}

	stateNames := make(map[model.StateName]bool, len(spec.States))
	for _, s := range spec.States {
		if s.Name == "" {
			return fmt.Errorf("%w: state name cannot be empty", ErrInvalidSpec)
		}
		if stateNames[s.Name] {
			return fmt.Errorf("%w: duplicate state %q", ErrInvalidSpec, s.Name)
		}
		stateNames[s.Name] = true

		if err := validateActionBindings(s.OnEnter, reg); err != nil {
			return fmt.Errorf("state %q ON_ENTER: %w", s.Name, err)
		}
		if err := validateActionBindings(s.OnExit, reg); err != nil {
			return fmt.Errorf("state %q ON_EXIT: %w", s.Name, err)
		}
	}

	if !stateNames[spec.InitialState] {
		return fmt.Errorf("%w: initial_state %q is not defined in states", ErrInvalidSpec, spec.InitialState)
	}

	seen := make(map[transitionKey]bool)
	for _, t := range spec.Transitions {
		if !stateNames[t.FromState] {
			return fmt.Errorf("%w: transition from_state %q is not defined", ErrInvalidSpec, t.FromState)
		}
		if !stateNames[t.ToState] {
			return fmt.Errorf("%w: transition to_state %q is not defined", ErrInvalidSpec, t.ToState)
		}
		if t.EventName == "" {
			return fmt.Errorf("%w: transition event_name cannot be empty", ErrInvalidSpec)
		}

		key := transitionKey{from: t.FromState, event: t.EventName, priority: t.Priority}
		if seen[key] {
			return fmt.Errorf("%w: duplicate transition (from=%q, event=%q, priority=%d)",
				ErrInvalidSpec, t.FromState, t.EventName, t.Priority)
		}
		seen[key] = true

		for _, gb := range t.Guards {
			if err := validateGuardBinding(gb, reg); err != nil {
				return fmt.Errorf("transition %s/%s guard: %w", t.FromState, t.EventName, err)
			}
		}
		if err := validateActionBindings(t.Actions, reg); err != nil {
			return fmt.Errorf("transition %s/%s ON_TRANSITION: %w", t.FromState, t.EventName, err)
		}
	}

	return nil
}

func validateGuardBinding(gb GuardBindingSpec, reg *registry.Registry) error {
	if !gb.ImplementationKind.Valid() {
		return fmt.Errorf("%w: invalid implementation_kind %q", ErrInvalidSpec, gb.ImplementationKind)
	}
	switch gb.ImplementationKind {
	case model.GuardImplGoRegistry:
		if gb.GuardName == "" {
			return fmt.Errorf("%w: guard_name required for GO_REGISTRY guard", ErrInvalidSpec)
		}
		if _, ok := reg.LookupGuard(gb.GuardName); !ok {
			return fmt.Errorf("%w: guard %q: %w", ErrInvalidSpec, gb.GuardName, model.ErrUnregisteredGuard)
		}
	case model.GuardImplScript:
		if gb.Script == "" {
			return fmt.Errorf("%w: script required for SCRIPT guard", ErrInvalidSpec)
		}
		if err := scripting.ValidateGuardSource(gb.Script); err != nil {
			return fmt.Errorf("%w: guard script does not compile: %v", ErrInvalidSpec, err)
		}
	}
	return nil
}

func validateActionBindings(bindings []ActionBindingSpec, reg *registry.Registry) error {
	for _, ab := range bindings {
		if ab.ActionName == "" {
			return fmt.Errorf("%w: action_name is required", ErrInvalidSpec)
		}
		if _, ok := reg.LookupAction(ab.ActionName); !ok {
			return fmt.Errorf("%w: action %q: %w", ErrInvalidSpec, ab.ActionName, model.ErrUnregisteredAction)
		}
		if !ab.Mode.Valid() {
			return fmt.Errorf("%w: invalid mode %q for action %q", ErrInvalidSpec, ab.Mode, ab.ActionName)
		}
		if !ab.OnError.Valid() {
			return fmt.Errorf("%w: invalid on_error %q for action %q", ErrInvalidSpec, ab.OnError, ab.ActionName)
		}
		if !ab.ParamsKind.Valid() {
			return fmt.Errorf("%w: invalid params_kind %q for action %q", ErrInvalidSpec, ab.ParamsKind, ab.ActionName)
		}
		if ab.ParamsKind == model.ParamsKindScript {
			if ab.ParamsScript == "" {
				return fmt.Errorf("%w: params_script required when params_kind=SCRIPT for action %q", ErrInvalidSpec, ab.ActionName)
			}
			if err := scripting.ValidateActionParamsSource(ab.ParamsScript); err != nil {
				return fmt.Errorf("%w: params_script for action %q does not compile: %v", ErrInvalidSpec, ab.ActionName, err)
			}
		}
	}
	return nil
}
