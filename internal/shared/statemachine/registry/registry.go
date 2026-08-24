// Package registry holds the process's Go-code implementations of guards and
// actions, registered by name at startup. State machine definitions stored in
// Postgres only ever reference these by name plus a params JSON blob — arbitrary
// code never comes from the database, only names and parameters do.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

var (
	ErrAlreadyRegistered = errors.New("already registered")
)

// Guard evaluates whether a transition is eligible. Implementations must be
// side-effect-free, fast, and free of network I/O: a guard runs while the engine
// holds a row lock on the state machine instance for the duration of Fire().
// Unlike Action, Guard deliberately has no database handle — that asymmetry is
// what makes the no-side-effects contract enforceable by the API shape, not just
// convention.
type Guard interface {
	Name() string
	Evaluate(ctx context.Context, ec model.ExecutionContext, params json.RawMessage) (bool, error)
}

// Action performs a side effect at a hook point (state entry/exit, or a specific
// transition). It may mutate ec.Context; the engine persists the mutated context
// back to the instance after a successful Fire(). Actions typically close over
// their own domain repository dependencies at registration time.
//
// db is the database handle to use for any of the action's own queries: for a
// SYNC action it is the live Fire() transaction (so the action's writes commit or
// roll back atomically with the state transition); for an ASYNC action, executed
// later by outbox.Publisher outside any Fire() transaction, it is the connection
// pool. It is never nil — an action that needs no database access can ignore it.
type Action interface {
	Name() string
	Execute(ctx context.Context, db sqlcgen.DBTX, ec *model.ExecutionContext, params json.RawMessage) error
}

// Registry is the process-wide lookup table of registered guards and actions,
// populated by explicit Go-code registration at startup.
type Registry struct {
	mu      sync.RWMutex
	guards  map[string]Guard
	actions map[string]Action
}

func New() *Registry {
	return &Registry{
		guards:  make(map[string]Guard),
		actions: make(map[string]Action),
	}
}

func (r *Registry) RegisterGuard(g Guard) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := g.Name()
	if _, exists := r.guards[name]; exists {
		return fmt.Errorf("register guard %q: %w", name, ErrAlreadyRegistered)
	}
	r.guards[name] = g
	return nil
}

func (r *Registry) RegisterAction(a Action) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := a.Name()
	if _, exists := r.actions[name]; exists {
		return fmt.Errorf("register action %q: %w", name, ErrAlreadyRegistered)
	}
	r.actions[name] = a
	return nil
}

func (r *Registry) LookupGuard(name string) (Guard, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.guards[name]
	return g, ok
}

func (r *Registry) LookupAction(name string) (Action, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.actions[name]
	return a, ok
}
