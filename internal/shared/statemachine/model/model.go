// Package model defines the core data types for the dynamic state machine engine:
// definitions/states/transitions/bindings as compiled in-memory structures, running
// instances, and the execution context passed to guards and actions.
package model

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDefinitionNotFound    = errors.New("state machine definition not found")
	ErrInstanceNotFound      = errors.New("state machine instance not found")
	ErrInstanceAlreadyExists = errors.New("state machine instance already exists for subject")
	ErrInstanceTerminal      = errors.New("state machine instance is already terminal")
	ErrNoValidTransition     = errors.New("no valid transition for current state and event")
	ErrUnregisteredGuard     = errors.New("guard is not registered")
	ErrUnregisteredAction    = errors.New("action is not registered")
	ErrOptimisticConflict    = errors.New("state machine instance optimistic concurrency conflict")
)

type MachineType string
type StateName string
type EventName string

// HookType selects when an ActionBinding runs: on entering a state, on exiting a
// state, or as part of firing a specific transition.
type HookType string

const (
	HookOnEnter      HookType = "ON_ENTER"
	HookOnExit       HookType = "ON_EXIT"
	HookOnTransition HookType = "ON_TRANSITION"
)

func (h HookType) Valid() bool {
	switch h {
	case HookOnEnter, HookOnExit, HookOnTransition:
		return true
	default:
		return false
	}
}

// ActionMode selects whether an action runs inline inside the Fire() transaction
// (SYNC) or is deferred to the transactional outbox for out-of-band execution (ASYNC).
type ActionMode string

const (
	ActionModeSync  ActionMode = "SYNC"
	ActionModeAsync ActionMode = "ASYNC"
)

func (m ActionMode) Valid() bool {
	switch m {
	case ActionModeSync, ActionModeAsync:
		return true
	default:
		return false
	}
}

// OnErrorPolicy determines what happens when a SYNC action (or a script computing
// its params) fails: ABORT rolls back the whole Fire() call, CONTINUE logs and proceeds.
type OnErrorPolicy string

const (
	OnErrorAbort    OnErrorPolicy = "ABORT"
	OnErrorContinue OnErrorPolicy = "CONTINUE"
)

func (p OnErrorPolicy) Valid() bool {
	switch p {
	case OnErrorAbort, OnErrorContinue:
		return true
	default:
		return false
	}
}

// InstanceStatus is derived from the state graph, not an independent authority:
// RUNNING until the instance lands on a state with IsFinal=true, then COMPLETED.
type InstanceStatus string

const (
	InstanceStatusRunning   InstanceStatus = "RUNNING"
	InstanceStatusCompleted InstanceStatus = "COMPLETED"
)

func (s InstanceStatus) Valid() bool {
	switch s {
	case InstanceStatusRunning, InstanceStatusCompleted:
		return true
	default:
		return false
	}
}

// GuardImplementationKind selects whether a guard binding is a registered Go
// implementation looked up by name, or Yaegi-interpreted Go source stored as data.
type GuardImplementationKind string

const (
	GuardImplGoRegistry GuardImplementationKind = "GO_REGISTRY"
	GuardImplScript     GuardImplementationKind = "SCRIPT"
)

func (k GuardImplementationKind) Valid() bool {
	switch k {
	case GuardImplGoRegistry, GuardImplScript:
		return true
	default:
		return false
	}
}

// ParamsKind selects whether an action binding's params are a static JSON blob or
// computed at fire-time by a Yaegi-interpreted script.
type ParamsKind string

const (
	ParamsKindStatic ParamsKind = "STATIC"
	ParamsKindScript ParamsKind = "SCRIPT"
)

func (k ParamsKind) Valid() bool {
	switch k {
	case ParamsKindStatic, ParamsKindScript:
		return true
	default:
		return false
	}
}

// OutboxStatus is the lifecycle of a queued ASYNC action side effect.
type OutboxStatus string

const (
	OutboxPending    OutboxStatus = "PENDING"
	OutboxProcessing OutboxStatus = "PROCESSING"
	OutboxPublished  OutboxStatus = "PUBLISHED"
	OutboxFailed     OutboxStatus = "FAILED"
)

// ScheduleStatus is the lifecycle of a timer-driven scheduled transition.
type ScheduleStatus string

const (
	SchedulePending    ScheduleStatus = "PENDING"
	ScheduleProcessing ScheduleStatus = "PROCESSING"
	ScheduleFired      ScheduleStatus = "FIRED"
	ScheduleCanceled   ScheduleStatus = "CANCELED"
	ScheduleFailed     ScheduleStatus = "FAILED"
)

// State is one node in a compiled Definition's graph.
type State struct {
	Name     StateName
	IsFinal  bool
	Metadata json.RawMessage
}

// GuardBinding is one guard evaluated (AND-chained, in Seq order) to decide whether
// its owning Transition is eligible.
type GuardBinding struct {
	Seq                int
	ImplementationKind GuardImplementationKind
	GuardName          string // registry key; empty when ImplementationKind == GuardImplScript
	Script             string // Yaegi source; empty when ImplementationKind == GuardImplGoRegistry
	Params             json.RawMessage
}

// ActionBinding is one action run at a hook point (state entry/exit, or a specific
// transition), in Seq order among bindings sharing the same hook point.
type ActionBinding struct {
	BindingID    uuid.UUID
	HookType     HookType
	StateName    StateName  // set for HookOnEnter / HookOnExit
	TransitionID *uuid.UUID // set for HookOnTransition
	Seq          int
	ActionName   string // always set: the registered Go Action that performs the side effect
	ParamsKind   ParamsKind
	Params       json.RawMessage
	ParamsScript string // Yaegi source; set when ParamsKind == ParamsKindScript
	Mode         ActionMode
	OnError      OnErrorPolicy
}

// Transition is one edge in the graph, keyed by (FromState, EventName), with its
// guard and ON_TRANSITION action bindings attached and pre-ordered.
type Transition struct {
	TransitionID  uuid.UUID
	FromState     StateName
	EventName     EventName
	ToState       StateName
	Priority      int
	EmitEventType string
	Guards        []GuardBinding  // Seq-ordered
	Actions       []ActionBinding // ON_TRANSITION bindings for this transition, Seq-ordered
}

// Instance is a running (or completed) execution of a Definition against one subject.
type Instance struct {
	InstanceID   uuid.UUID
	DefinitionID uuid.UUID
	MachineType  MachineType
	SubjectType  string
	SubjectID    string
	CurrentState StateName
	Context      json.RawMessage
	Version      int64
	Status       InstanceStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ExecutionContext is what guards and actions see when evaluated/executed for one
// Fire() (or CreateInstance) call. Context is an immutable snapshot when read by
// guards; the engine tracks a mutable working copy across action hooks and persists
// it back to the instance at the end of a successful Fire().
type ExecutionContext struct {
	InstanceID   uuid.UUID
	SubjectType  string
	SubjectID    string
	FromState    StateName
	EventName    EventName
	EventPayload json.RawMessage
	Context      json.RawMessage
	Now          time.Time
	TriggeredBy  string
}

// FireResult describes the outcome of a successful Fire() or CreateInstance() call.
type FireResult struct {
	FromState    StateName
	ToState      StateName
	TransitionID *uuid.UUID
	Version      int64
	Completed    bool
}
