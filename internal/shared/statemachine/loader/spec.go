// Package loader compiles a JSON-friendly DefinitionSpec into a new, immutable
// version of a state machine definition in Postgres. This is the authoring path:
// humans or config write a DefinitionSpec, Publish validates it and inserts it as
// one new sm_definitions row (plus its states/transitions/bindings) in a single
// transaction.
package loader

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

// GuardBindingSpec describes one guard evaluated (AND-chained, in Seq order) to
// decide whether its owning transition is eligible.
type GuardBindingSpec struct {
	Seq                int
	ImplementationKind model.GuardImplementationKind
	GuardName          string // registry key; required when ImplementationKind == GO_REGISTRY
	Script             string // Yaegi source; required when ImplementationKind == SCRIPT
	Params             json.RawMessage
}

// ActionBindingSpec describes one action run at a hook point, in Seq order among
// bindings sharing the same hook point. ActionName always identifies a registered
// Go Action — even when ParamsKind is SCRIPT, the script only computes the params
// handed to that Action; it never performs the side effect itself.
type ActionBindingSpec struct {
	Seq          int
	ActionName   string
	ParamsKind   model.ParamsKind
	Params       json.RawMessage // used when ParamsKind == STATIC
	ParamsScript string          // Yaegi source; required when ParamsKind == SCRIPT
	Mode         model.ActionMode
	OnError      model.OnErrorPolicy
}

func (ab ActionBindingSpec) toModel(hookType model.HookType, stateName model.StateName, transitionID *uuid.UUID) model.ActionBinding {
	return model.ActionBinding{
		HookType:     hookType,
		StateName:    stateName,
		TransitionID: transitionID,
		Seq:          ab.Seq,
		ActionName:   ab.ActionName,
		ParamsKind:   ab.ParamsKind,
		Params:       ab.Params,
		ParamsScript: ab.ParamsScript,
		Mode:         ab.Mode,
		OnError:      ab.OnError,
	}
}

// StateSpec describes one node in the graph, along with the actions that run when
// an instance enters or exits it.
type StateSpec struct {
	Name     model.StateName
	IsFinal  bool
	Metadata json.RawMessage
	OnEnter  []ActionBindingSpec
	OnExit   []ActionBindingSpec
}

// TransitionSpec describes one edge in the graph, keyed by (FromState, EventName).
type TransitionSpec struct {
	FromState     model.StateName
	EventName     model.EventName
	ToState       model.StateName
	Priority      int
	EmitEventType string
	Guards        []GuardBindingSpec
	Actions       []ActionBindingSpec // ON_TRANSITION bindings for this transition
}

// DefinitionSpec is the full, JSON-friendly description of one state machine
// definition version. Publish validates it and compiles it into a new immutable
// sm_definitions row.
type DefinitionSpec struct {
	MachineType  model.MachineType
	Version      int
	InitialState model.StateName
	// Activate marks this version as the one new instances are created against
	// (CreateInstance looks up the active version by MachineType), deactivating
	// any previously active version for the same MachineType in the same
	// transaction. Publishing an inactive version is useful for validating a
	// draft before promoting it.
	Activate    bool
	States      []StateSpec
	Transitions []TransitionSpec
}
