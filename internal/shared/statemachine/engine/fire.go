package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
)

type fireOptions struct {
	triggeredBy string
	actor       *eventmodel.Actor
}

// FireOption configures one Fire() call.
type FireOption func(*fireOptions)

// WithTriggeredBy records who/what triggered this transition in the audit trail.
func WithTriggeredBy(who string) FireOption {
	return func(o *fireOptions) { o.triggeredBy = who }
}

// WithEventActor sets the eventstore Actor used when a fired transition's
// EmitEventType is set. Defaults to an ActorTypeSystem actor if not supplied.
func WithEventActor(actor eventmodel.Actor) FireOption {
	return func(o *fireOptions) { o.actor = &actor }
}

type instanceOptions struct {
	triggeredBy string
	tx          sqlcgen.DBTX // optional: share caller's transaction
}

// InstanceOption configures one CreateInstance() call.
type InstanceOption func(*instanceOptions)

// WithCreatedBy records who/what created this instance in the audit trail.
func WithCreatedBy(who string) InstanceOption {
	return func(o *instanceOptions) { o.triggeredBy = who }
}

// WithTx makes CreateInstance run inside an existing transaction supplied by
// the caller instead of opening its own. The caller is responsible for
// committing or rolling back that transaction — CreateInstance will not call
// Commit or Rollback on it.
func WithTx(tx sqlcgen.DBTX) InstanceOption {
	return func(o *instanceOptions) { o.tx = tx }
}

// CreateInstance creates a new instance against the currently active definition
// for machineType, runs the initial state's ON_ENTER hooks, and records the
// creation as a transition_id=nil history row.
func (e *Engine) CreateInstance(
	ctx context.Context,
	machineType model.MachineType,
	subjectType, subjectID string,
	initialContext json.RawMessage,
	opts ...InstanceOption,
) (model.Instance, error) {
	var o instanceOptions
	for _, opt := range opts {
		opt(&o)
	}

	active, err := e.repo.GetActiveDefinition(ctx, nil, machineType)
	if err != nil {
		return model.Instance{}, err
	}

	def, err := e.loadDefinition(ctx, active.DefinitionID)
	if err != nil {
		return model.Instance{}, err
	}

	// Use the caller-supplied transaction when provided; otherwise open one.
	// In the injected-tx case the caller owns commit/rollback — we must not
	// touch it here so that all work stays in the caller's atomic unit.
	var tx sqlcgen.DBTX
	var ownedTx pgx.Tx // non-nil only when we opened the transaction ourselves
	if o.tx != nil {
		tx = o.tx
	} else {
		var err error
		ownedTx, err = e.pool.Begin(ctx)
		if err != nil {
			return model.Instance{}, fmt.Errorf("begin transaction: %w", err)
		}
		defer ownedTx.Rollback(ctx) //nolint:errcheck
		tx = ownedTx
	}

	//nolint:lll // Kept together for readability.
	instance, err := e.repo.CreateInstance(ctx, tx, def.Meta.DefinitionID, machineType, subjectType, subjectID, def.Meta.InitialState, initialContext)
	if err != nil {
		return model.Instance{}, err
	}

	ec := model.ExecutionContext{
		InstanceID:  instance.InstanceID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		FromState:   "",
		EventName:   "",
		Context:     instance.Context,
		Now:         time.Now(),
		TriggeredBy: o.triggeredBy,
	}

	historyID, _, err := e.repo.InsertTransitionHistoryPending(ctx, tx, repository.TransitionHistoryPending{
		InstanceID:    instance.InstanceID,
		TransitionID:  nil,
		FromState:     "",
		ToState:       def.Meta.InitialState,
		EventName:     "",
		EventPayload:  nil,
		ContextBefore: instance.Context,
		TriggeredBy:   o.triggeredBy,
	})
	if err != nil {
		return model.Instance{}, fmt.Errorf("insert creation history: %w", err)
	}

	if err := e.runActionHooks(ctx, tx, def.EnterActions(def.Meta.InitialState), instance.InstanceID, historyID, &ec); err != nil {
		return model.Instance{}, err
	}

	newStatus := model.InstanceStatusRunning
	if def.IsFinal(def.Meta.InitialState) {
		newStatus = model.InstanceStatusCompleted
	}
	// Always persist here: ON_ENTER hooks may have mutated ec.Context even when
	// the initial state isn't final, and that mutation must not be silently
	// dropped just because status doesn't need to change.
	//nolint:lll // Kept together for readability.
	if _, err := e.repo.UpdateInstanceState(ctx, tx, instance.InstanceID, instance.Version, def.Meta.InitialState, ec.Context, newStatus); err != nil {
		return model.Instance{}, fmt.Errorf("persist initial context: %w", err)
	}

	if err := e.repo.FinalizeTransitionHistory(ctx, tx, historyID, ec.Context); err != nil {
		return model.Instance{}, err
	}

	// Only commit when we own the transaction. If the caller injected their tx,
	// they are responsible for committing it after all their work is done.
	if ownedTx != nil {
		if err := ownedTx.Commit(ctx); err != nil {
			return model.Instance{}, fmt.Errorf("commit create instance: %w", err)
		}
	}

	instance.Context = ec.Context
	instance.Status = newStatus
	return instance, nil
}

// Fire evaluates candidate transitions for (instance's current state, eventName)
// in priority order, running the first one whose guards all pass, then runs its
// ON_EXIT/ON_TRANSITION/ON_ENTER action hooks and persists the resulting state
// change — all within one database transaction. See package docs and the plan for
// the full algorithm and its concurrency rationale.
func (e *Engine) Fire(
	ctx context.Context,
	instanceID uuid.UUID,
	eventName model.EventName,
	payload json.RawMessage,
	opts ...FireOption,
) (model.FireResult, error) {
	var o fireOptions
	for _, opt := range opts {
		opt(&o)
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return model.FireResult{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	instance, err := e.repo.GetInstanceForUpdate(ctx, tx, instanceID)
	if err != nil {
		return model.FireResult{}, err
	}
	if instance.Status != model.InstanceStatusRunning {
		return model.FireResult{}, model.ErrInstanceTerminal
	}

	def, err := e.loadDefinition(ctx, instance.DefinitionID)
	if err != nil {
		return model.FireResult{}, err
	}

	candidates := def.TransitionsFor(instance.CurrentState, eventName)
	if len(candidates) == 0 {
		return model.FireResult{}, model.ErrNoValidTransition
	}

	ec := model.ExecutionContext{
		InstanceID:   instance.InstanceID,
		SubjectType:  instance.SubjectType,
		SubjectID:    instance.SubjectID,
		FromState:    instance.CurrentState,
		EventName:    eventName,
		EventPayload: payload,
		Context:      instance.Context,
		Now:          time.Now(),
		TriggeredBy:  o.triggeredBy,
	}

	selected, err := e.selectTransition(ctx, candidates, ec)
	if err != nil {
		return model.FireResult{}, err
	}
	if selected == nil {
		return model.FireResult{}, model.ErrNoValidTransition
	}

	toState := selected.ToState
	isFinal := def.IsFinal(toState)
	transitionID := selected.TransitionID

	historyID, _, err := e.repo.InsertTransitionHistoryPending(ctx, tx, repository.TransitionHistoryPending{
		InstanceID:    instance.InstanceID,
		TransitionID:  &transitionID,
		FromState:     instance.CurrentState,
		ToState:       toState,
		EventName:     eventName,
		EventPayload:  payload,
		ContextBefore: instance.Context,
		TriggeredBy:   o.triggeredBy,
	})
	if err != nil {
		return model.FireResult{}, fmt.Errorf("insert transition history: %w", err)
	}

	if err := e.runActionHooks(ctx, tx, def.ExitActions(instance.CurrentState), instance.InstanceID, historyID, &ec); err != nil {
		return model.FireResult{}, err
	}
	if err := e.runActionHooks(ctx, tx, selected.Actions, instance.InstanceID, historyID, &ec); err != nil {
		return model.FireResult{}, err
	}
	if err := e.runActionHooks(ctx, tx, def.EnterActions(toState), instance.InstanceID, historyID, &ec); err != nil {
		return model.FireResult{}, err
	}

	if selected.EmitEventType != "" && e.events != nil {
		actor := eventmodel.Actor{Type: eventmodel.ActorTypeSystem}
		if o.actor != nil {
			actor = *o.actor
		}
		_, err := e.events.AppendEvent(ctx, tx, eventmodel.AppendRequest{
			AggregateType: eventmodel.AggregateType(instance.SubjectType),
			AggregateID:   instance.SubjectID,
			EventType:     eventmodel.EventType(selected.EmitEventType),
			EventVersion:  1,
			Actor:         actor,
			Payload:       json.RawMessage(ec.Context),
		})
		if err != nil {
			return model.FireResult{}, fmt.Errorf("emit event %q: %w", selected.EmitEventType, err)
		}
	}

	newStatus := model.InstanceStatusRunning
	if isFinal {
		newStatus = model.InstanceStatusCompleted
	}

	rowsAffected, err := e.repo.UpdateInstanceState(ctx, tx, instanceID, instance.Version, toState, ec.Context, newStatus)
	if err != nil {
		return model.FireResult{}, fmt.Errorf("update instance state: %w", err)
	}
	if rowsAffected == 0 {
		// Should be unreachable: GetInstanceForUpdate's row lock serializes all
		// concurrent Fire() calls on this instance for the duration of this
		// transaction. Surfacing as a distinct error rather than silently
		// retrying makes a violation of that invariant loud.
		return model.FireResult{}, model.ErrOptimisticConflict
	}

	if err := e.repo.FinalizeTransitionHistory(ctx, tx, historyID, ec.Context); err != nil {
		return model.FireResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return model.FireResult{}, fmt.Errorf("commit fire: %w", err)
	}

	return model.FireResult{
		FromState:    instance.CurrentState,
		ToState:      toState,
		TransitionID: &transitionID,
		Version:      instance.Version + 1,
		Completed:    isFinal,
	}, nil
}

// selectTransition evaluates candidates in priority order (pre-sorted by
// model.Compile) and returns the first whose guards all pass (AND semantics,
// short-circuiting on the first failing guard). A guard error aborts selection
// entirely — it is never silently treated as pass or fail.
//
//nolint:lll // Kept together for readability.
func (e *Engine) selectTransition(ctx context.Context, candidates []model.Transition, ec model.ExecutionContext) (*model.Transition, error) {
	for i := range candidates {
		t := candidates[i]

		allPass := true
		for _, gb := range t.Guards {
			pass, err := e.evaluateGuard(ctx, gb, ec)
			if err != nil {
				return nil, fmt.Errorf("evaluate guard on transition %s: %w", t.TransitionID, err)
			}
			if !pass {
				allPass = false
				break
			}
		}
		if allPass {
			return &t, nil
		}
	}
	return nil, nil
}

func (e *Engine) evaluateGuard(ctx context.Context, gb model.GuardBinding, ec model.ExecutionContext) (bool, error) {
	switch gb.ImplementationKind {
	case model.GuardImplGoRegistry:
		guard, ok := e.registry.LookupGuard(gb.GuardName)
		if !ok {
			return false, fmt.Errorf("guard %q: %w", gb.GuardName, model.ErrUnregisteredGuard)
		}
		return guard.Evaluate(ctx, ec, gb.Params)
	case model.GuardImplScript:
		if e.scripts == nil {
			return false, fmt.Errorf("guard is SCRIPT-kind but no ScriptEvaluator is configured")
		}
		return e.scripts.EvalGuard(ctx, gb.Script, ec, gb.Params)
	default:
		return false, fmt.Errorf("unknown guard implementation_kind %q", gb.ImplementationKind)
	}
}

// runActionHooks executes one hook point's action bindings in Seq order. SYNC
// actions run inline against tx (so their writes are part of the same atomic
// transition); ASYNC actions are enqueued to the outbox (also within tx, so the
// enqueue is guaranteed exactly-once-per-commit) for outbox.Publisher to execute
// later, outside any transaction. A binding's on_error policy governs both
// params-resolution errors and execution errors.
func (e *Engine) runActionHooks(
	ctx context.Context, tx sqlcgen.DBTX,
	bindings []model.ActionBinding, instanceID uuid.UUID, historyID int64,
	ec *model.ExecutionContext,
) error {
	for _, ab := range bindings {
		params, err := e.resolveActionParams(ctx, ab, *ec)
		if err != nil {
			if ab.OnError == model.OnErrorContinue {
				continue
			}
			return fmt.Errorf("resolve params for action %q: %w", ab.ActionName, err)
		}

		switch ab.Mode {
		case model.ActionModeSync:
			action, ok := e.registry.LookupAction(ab.ActionName)
			if !ok {
				if ab.OnError == model.OnErrorContinue {
					continue
				}
				return fmt.Errorf("action %q: %w", ab.ActionName, model.ErrUnregisteredAction)
			}
			if err := action.Execute(ctx, tx, ec, params); err != nil {
				if ab.OnError == model.OnErrorContinue {
					continue
				}
				return fmt.Errorf("execute action %q: %w", ab.ActionName, err)
			}
		case model.ActionModeAsync:
			bindingID := ab.BindingID
			hID := historyID
			if err := e.repo.InsertOutboxRow(ctx, tx, instanceID, &hID, &bindingID, ab.ActionName, params); err != nil {
				if ab.OnError == model.OnErrorContinue {
					continue
				}
				return fmt.Errorf("enqueue outbox action %q: %w", ab.ActionName, err)
			}
		default:
			return fmt.Errorf("unknown action mode %q for action %q", ab.Mode, ab.ActionName)
		}
	}
	return nil
}

func (e *Engine) resolveActionParams(ctx context.Context, ab model.ActionBinding, ec model.ExecutionContext) (json.RawMessage, error) {
	switch ab.ParamsKind {
	case model.ParamsKindStatic:
		return ab.Params, nil
	case model.ParamsKindScript:
		if e.scripts == nil {
			return nil, fmt.Errorf("action %q has SCRIPT params but no ScriptEvaluator is configured", ab.ActionName)
		}
		return e.scripts.EvalActionParams(ctx, ab.ParamsScript, ec, ab.Params)
	default:
		return nil, fmt.Errorf("unknown params_kind %q for action %q", ab.ParamsKind, ab.ActionName)
	}
}
