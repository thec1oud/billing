// Package engine is the universal executor for the dynamic state machine: it
// loads compiled Definitions (cached by immutable definition_id), and runs the
// Fire()/CreateInstance() transactional algorithm that evaluates guards, runs
// entry/exit/transition action hooks (sync inline or deferred to the outbox for
// async), and persists the resulting state transition.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
)

// ScriptEvaluator is the subset of the scripting package's Pool the engine needs.
// Defined here (rather than importing the scripting package directly) so the
// engine has no compile-time dependency on Yaegi; passing nil is valid and simply
// means SCRIPT-kind guards/actions are unavailable (a clear error, not a panic).
// Implementations own their own timeout policy internally (see scripting.Pool).
type ScriptEvaluator interface {
	EvalGuard(ctx context.Context, source string, ec model.ExecutionContext, params json.RawMessage) (bool, error)
	EvalActionParams(ctx context.Context, source string, ec model.ExecutionContext, params json.RawMessage) (json.RawMessage, error)
}

type Engine struct {
	pool     *pgxpool.Pool
	repo     *repository.PostgresRepository
	registry *registry.Registry
	events   *eventservice.Service // optional; nil disables transition.EmitEventType handling
	scripts  ScriptEvaluator       // optional; nil disables SCRIPT-kind guards/actions

	cacheMu sync.RWMutex
	cache   map[uuid.UUID]*model.Definition
}

// Option configures optional Engine dependencies at construction time.
type Option func(*Engine)

func WithEventStore(events *eventservice.Service) Option {
	return func(e *Engine) { e.events = events }
}

func WithScripting(scripts ScriptEvaluator) Option {
	return func(e *Engine) { e.scripts = scripts }
}

func NewEngine(pool *pgxpool.Pool, repo *repository.PostgresRepository, reg *registry.Registry, opts ...Option) *Engine {
	e := &Engine{
		pool:     pool,
		repo:     repo,
		registry: reg,
		cache:    make(map[uuid.UUID]*model.Definition),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// loadDefinition returns the compiled Definition for definitionID, using the
// in-memory cache when possible. Definitions are immutable once published, so a
// cache hit never needs invalidation — only population. On a cache miss, the
// bundle is read via the pool (not a caller's tx), and every guard/action name in
// it is cross-validated against this process's registry so a stale or
// differently-configured registry fails fast here rather than as a nil-map panic
// during hook execution.
func (e *Engine) loadDefinition(ctx context.Context, definitionID uuid.UUID) (*model.Definition, error) {
	e.cacheMu.RLock()
	def, ok := e.cache[definitionID]
	e.cacheMu.RUnlock()
	if ok {
		return def, nil
	}

	bundle, err := e.repo.GetDefinitionBundle(ctx, nil, definitionID)
	if err != nil {
		return nil, fmt.Errorf("load definition %s: %w", definitionID, err)
	}

	if err := e.validateBundleAgainstRegistry(bundle); err != nil {
		return nil, err
	}

	compiled := model.Compile(bundle)

	e.cacheMu.Lock()
	e.cache[definitionID] = compiled
	e.cacheMu.Unlock()

	return compiled, nil
}

func (e *Engine) validateBundleAgainstRegistry(bundle model.DefinitionBundle) error {
	for _, t := range bundle.Transitions {
		for _, gb := range t.Guards {
			if gb.ImplementationKind == model.GuardImplGoRegistry {
				if _, ok := e.registry.LookupGuard(gb.GuardName); !ok {
					return fmt.Errorf("definition %s: guard %q: %w", bundle.Meta.DefinitionID, gb.GuardName, model.ErrUnregisteredGuard)
				}
			}
		}
	}
	for _, ab := range bundle.Actions {
		if _, ok := e.registry.LookupAction(ab.ActionName); !ok {
			return fmt.Errorf("definition %s: action %q: %w", bundle.Meta.DefinitionID, ab.ActionName, model.ErrUnregisteredAction)
		}
	}
	return nil
}
