package model

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

// DefinitionMeta is the top-level row for one immutable, versioned state machine
// definition.
type DefinitionMeta struct {
	DefinitionID uuid.UUID
	MachineType  MachineType
	Version      int
	InitialState StateName
	IsActive     bool
	CreatedAt    time.Time
}

// DefinitionBundle is the raw, unindexed shape returned by the repository: a
// definition's states, transitions (each already carrying its guard bindings), and
// all action bindings (both state-hook and transition-hook). Compile() turns this
// into a Definition with fast lookup indices.
type DefinitionBundle struct {
	Meta        DefinitionMeta
	States      []State
	Transitions []Transition
	Actions     []ActionBinding
}

// Definition is a compiled, immutable state machine graph ready for the engine's
// hot path: transition lookup by (from_state, event_name) pre-sorted by priority,
// and entry/exit action bindings pre-grouped by state.
type Definition struct {
	Meta   DefinitionMeta
	states map[StateName]State

	transitionsByFromEvent map[transitionKey][]Transition
	enterActions           map[StateName][]ActionBinding
	exitActions            map[StateName][]ActionBinding
}

type transitionKey struct {
	from  StateName
	event EventName
}

// Compile builds a Definition with fast lookup indices from a raw bundle. It
// assumes referential integrity was already enforced at publish time (DB FKs +
// loader.Publish validation) — this is index-building, not re-validation.
func Compile(bundle DefinitionBundle) *Definition {
	d := &Definition{
		Meta:                   bundle.Meta,
		states:                 make(map[StateName]State, len(bundle.States)),
		transitionsByFromEvent: make(map[transitionKey][]Transition),
		enterActions:           make(map[StateName][]ActionBinding),
		exitActions:            make(map[StateName][]ActionBinding),
	}

	for _, s := range bundle.States {
		d.states[s.Name] = s
	}

	actionsByTransition := make(map[uuid.UUID][]ActionBinding)
	for _, a := range bundle.Actions {
		switch a.HookType {
		case HookOnEnter:
			d.enterActions[a.StateName] = append(d.enterActions[a.StateName], a)
		case HookOnExit:
			d.exitActions[a.StateName] = append(d.exitActions[a.StateName], a)
		case HookOnTransition:
			if a.TransitionID != nil {
				actionsByTransition[*a.TransitionID] = append(actionsByTransition[*a.TransitionID], a)
			}
		}
	}
	for _, bindings := range d.enterActions {
		sortActionsBySeq(bindings)
	}
	for _, bindings := range d.exitActions {
		sortActionsBySeq(bindings)
	}

	for _, t := range bundle.Transitions {
		t.Actions = actionsByTransition[t.TransitionID]
		sortActionsBySeq(t.Actions)
		sortGuardsBySeq(t.Guards)

		key := transitionKey{from: t.FromState, event: t.EventName}
		d.transitionsByFromEvent[key] = append(d.transitionsByFromEvent[key], t)
	}
	for key, transitions := range d.transitionsByFromEvent {
		sortTransitionsByPriority(transitions)
		d.transitionsByFromEvent[key] = transitions
	}

	return d
}

// IsFinal reports whether the given state is a terminal state in this definition.
func (d *Definition) IsFinal(s StateName) bool {
	return d.states[s].IsFinal
}

// HasState reports whether the given state exists in this definition.
func (d *Definition) HasState(s StateName) bool {
	_, ok := d.states[s]
	return ok
}

// TransitionsFor returns the candidate transitions for (from, event), pre-sorted by
// priority ascending with a deterministic transition_id tie-break.
func (d *Definition) TransitionsFor(from StateName, event EventName) []Transition {
	return d.transitionsByFromEvent[transitionKey{from: from, event: event}]
}

// EventsFrom returns the distinct event names with at least one transition
// defined from the given state, in no particular order.
func (d *Definition) EventsFrom(from StateName) []EventName {
	seen := make(map[EventName]bool)
	var events []EventName
	for key := range d.transitionsByFromEvent {
		if key.from == from && !seen[key.event] {
			seen[key.event] = true
			events = append(events, key.event)
		}
	}
	return events
}

// EnterActions returns the ON_ENTER action bindings for a state, in Seq order.
func (d *Definition) EnterActions(s StateName) []ActionBinding {
	return d.enterActions[s]
}

// ExitActions returns the ON_EXIT action bindings for a state, in Seq order.
func (d *Definition) ExitActions(s StateName) []ActionBinding {
	return d.exitActions[s]
}

func sortTransitionsByPriority(transitions []Transition) {
	sort.Slice(transitions, func(i, j int) bool {
		if transitions[i].Priority != transitions[j].Priority {
			return transitions[i].Priority < transitions[j].Priority
		}
		return transitions[i].TransitionID.String() < transitions[j].TransitionID.String()
	})
}

func sortGuardsBySeq(guards []GuardBinding) {
	sort.Slice(guards, func(i, j int) bool { return guards[i].Seq < guards[j].Seq })
}

func sortActionsBySeq(actions []ActionBinding) {
	sort.Slice(actions, func(i, j int) bool { return actions[i].Seq < actions[j].Seq })
}
