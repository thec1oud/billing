// Package scripting evaluates guard and action-params bindings stored as data —
// real Go source, interpreted at runtime via github.com/traefik/yaegi — against a
// deliberately curated interop surface (see the env subpackage). This is what
// lets a state machine definition carry actual decision logic as database rows
// without a deploy, while keeping "scripts compute only": a script can decide a
// bool (Guard) or compute a params map (Params), but performing the resulting
// side effect always goes through a registered, reviewed Go registry.Action.
//
// A script is Go source declaring `package script` and one function:
//
//	func Guard(context, payload, params map[string]any) bool
//
// or
//
//	func Params(context, payload, params map[string]any) map[string]any
//
// It may additionally `import "statemachine/env"` to use the handful of pure
// helpers exported there. Nothing else is reachable — no "os", "net", "io": those
// packages are simply never registered with the interpreter.
package scripting

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/traefik/yaegi/interp"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting/env"
)

const (
	defaultConcurrency = 8
	defaultTimeout     = 100 * time.Millisecond
)

// Pool evaluates scripts with bounded concurrency (a semaphore, not a literal
// pool of long-lived interpreters — each eval gets a fresh *interp.Interpreter,
// which trades a small amount of per-call startup cost for the guarantee that
// two different scripts can never interfere with each other's symbol table) and
// a per-eval wall-clock timeout. Yaegi has no built-in step/resource limiter, so
// the timeout is enforced by racing the eval against context.WithTimeout — a
// runaway script's goroutine may continue running in the background after a
// timeout is returned; keep the default timeout short since scripts are meant to
// be simple, side-effect-free computations, not long-running logic.
type Pool struct {
	sem     chan struct{}
	timeout time.Duration
}

// NewPool creates a Pool. concurrency <= 0 defaults to 8 concurrent evaluations;
// timeout <= 0 defaults to 100ms.
func NewPool(concurrency int, timeout time.Duration) *Pool {
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Pool{sem: make(chan struct{}, concurrency), timeout: timeout}
}

// EvalGuard compiles and runs source's Guard function against ec and params,
// returning its bool result.
func (p *Pool) EvalGuard(ctx context.Context, source string, ec model.ExecutionContext, params json.RawMessage) (bool, error) {
	result, err := p.eval(ctx, source, "Guard", ec, params)
	if err != nil {
		return false, err
	}
	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("script.Guard must return bool, got %T", result)
	}
	return b, nil
}

// EvalActionParams compiles and runs source's Params function against ec and
// params, marshaling its map result back to JSON to become an action's params.
//
//nolint:lll // Kept together for readability.
func (p *Pool) EvalActionParams(ctx context.Context, source string, ec model.ExecutionContext, params json.RawMessage) (json.RawMessage, error) {
	result, err := p.eval(ctx, source, "Params", ec, params)
	if err != nil {
		return nil, err
	}
	m, ok := result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("script.Params must return map[string]any, got %T", result)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal script.Params result: %w", err)
	}
	return b, nil
}

type evalOutcome struct {
	val any
	err error
}

func (p *Pool) eval(ctx context.Context, source, funcName string, ec model.ExecutionContext, params json.RawMessage) (any, error) {
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	evalCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	contextMap, err := toMap(ec.Context)
	if err != nil {
		return nil, fmt.Errorf("decode context for script: %w", err)
	}
	payloadMap, err := toMap(ec.EventPayload)
	if err != nil {
		return nil, fmt.Errorf("decode event payload for script: %w", err)
	}
	paramsMap, err := toMap(params)
	if err != nil {
		return nil, fmt.Errorf("decode params for script: %w", err)
	}

	done := make(chan evalOutcome, 1)
	go func() {
		done <- runScript(source, funcName, contextMap, payloadMap, paramsMap)
	}()

	select {
	case res := <-done:
		return res.val, res.err
	case <-evalCtx.Done():
		return nil, fmt.Errorf("script execution timed out after %s: %w", p.timeout, evalCtx.Err())
	}
}

func runScript(source, funcName string, contextMap, payloadMap, paramsMap map[string]any) evalOutcome {
	i := interp.New(interp.Options{})
	if err := i.Use(env.Exports); err != nil {
		return evalOutcome{nil, fmt.Errorf("load script env: %w", err)}
	}

	if _, err := i.Eval(source); err != nil {
		return evalOutcome{nil, fmt.Errorf("compile script: %w", err)}
	}

	fnVal, err := i.Eval("script." + funcName)
	if err != nil {
		//nolint:lll // Kept together for readability.
		return evalOutcome{nil, fmt.Errorf("resolve script.%s (did the script declare 'package script' and define %s?): %w", funcName, funcName, err)}
	}

	switch funcName {
	case "Guard":
		fn, ok := fnVal.Interface().(func(map[string]any, map[string]any, map[string]any) bool)
		if !ok {
			return evalOutcome{nil, fmt.Errorf("script.Guard must have signature func(context, payload, params map[string]any) bool")}
		}
		return evalOutcome{fn(contextMap, payloadMap, paramsMap), nil}
	case "Params":
		fn, ok := fnVal.Interface().(func(map[string]any, map[string]any, map[string]any) map[string]any)
		if !ok {
			return evalOutcome{nil, fmt.Errorf("script.Params must have signature func(context, payload, params map[string]any) map[string]any")}
		}
		return evalOutcome{fn(contextMap, payloadMap, paramsMap), nil}
	default:
		return evalOutcome{nil, fmt.Errorf("unknown script entrypoint %q", funcName)}
	}
}

func toMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}
