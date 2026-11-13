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
// packages are simply never registered with the interpreter. Package variables,
// init functions, and script-spawned goroutines are rejected so cached programs
// cannot share mutable state or exceed the pool's goroutine bound.
package scripting

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting/env"
)

const (
	defaultConcurrency = 8
	defaultTimeout     = 100 * time.Millisecond
	maxCachedScripts   = 128
)

// ErrCapacityExceeded indicates every interpreter slot is occupied by work
// that already timed out and has not exited.
var ErrCapacityExceeded = errors.New("script evaluation capacity exceeded")

// Pool caches compiled scripts and bounds active evaluations. A timed-out
// interpreter keeps its slot until it exits; if every slot is occupied, new
// work is rejected instead of creating more interpreter goroutines.
type Pool struct {
	sem             chan struct{}
	timeout         time.Duration
	capacityMu      sync.Mutex
	runaways        int
	capacityChanged chan struct{}
	cacheMu         sync.Mutex
	cache           map[scriptKey]*list.Element
	lru             list.List
}

type scriptKey struct {
	hash [32]byte
}

type cachedScript struct {
	mu        sync.Mutex
	guard     func(map[string]any, map[string]any, map[string]any) bool
	guardErr  error
	params    func(map[string]any, map[string]any, map[string]any) map[string]any
	paramsErr error
	err       error
}

type cacheValue struct {
	key    scriptKey
	script *cachedScript
}

type evalJob struct {
	mu       sync.Mutex
	timedOut bool
	finished bool
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
	return &Pool{
		sem:             make(chan struct{}, concurrency),
		timeout:         timeout,
		capacityChanged: make(chan struct{}),
		cache:           make(map[scriptKey]*list.Element),
	}
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
	if err := ctx.Err(); err != nil {
		return nil, ctx.Err()
	}

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

	if err := p.acquire(ctx); err != nil {
		return nil, err
	}

	evalCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	done := make(chan evalOutcome, 1)
	job := &evalJob{}
	go func() {
		defer p.finishJob(job)
		done <- p.runScript(source, funcName, contextMap, payloadMap, paramsMap)
	}()

	select {
	case res := <-done:
		return res.val, res.err
	case <-evalCtx.Done():
		p.markTimedOut(job)
		if errors.Is(evalCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("script execution timed out after %s: %w", p.timeout, evalCtx.Err())
		}
		return nil, fmt.Errorf("script execution canceled: %w", evalCtx.Err())
	}
}

func (p *Pool) acquire(ctx context.Context) error {
	for {
		p.capacityMu.Lock()
		if p.runaways >= cap(p.sem) {
			p.capacityMu.Unlock()
			return ErrCapacityExceeded
		}
		changed := p.capacityChanged
		p.capacityMu.Unlock()

		select {
		case p.sem <- struct{}{}:
			return nil
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *Pool) markTimedOut(job *evalJob) {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.finished || job.timedOut {
		return
	}
	job.timedOut = true

	p.capacityMu.Lock()
	p.runaways++
	close(p.capacityChanged)
	p.capacityChanged = make(chan struct{})
	p.capacityMu.Unlock()
}

func (p *Pool) finishJob(job *evalJob) {
	job.mu.Lock()
	job.finished = true
	timedOut := job.timedOut
	job.mu.Unlock()

	if timedOut {
		p.capacityMu.Lock()
		p.runaways--
		close(p.capacityChanged)
		p.capacityChanged = make(chan struct{})
		p.capacityMu.Unlock()
	}
	<-p.sem
}

func (p *Pool) runScript(source, funcName string, contextMap, payloadMap, paramsMap map[string]any) evalOutcome {
	key := scriptKey{hash: sha256.Sum256([]byte(source))}
	p.cacheMu.Lock()
	element, ok := p.cache[key]
	if !ok {
		script := compileScript(source)
		element = p.lru.PushFront(cacheValue{key: key, script: script})
		p.cache[key] = element
		if p.lru.Len() > maxCachedScripts {
			oldest := p.lru.Back()
			delete(p.cache, oldest.Value.(cacheValue).key)
			p.lru.Remove(oldest)
		}
	} else {
		p.lru.MoveToFront(element)
	}
	entry := element.Value.(cacheValue).script
	p.cacheMu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.err != nil {
		return evalOutcome{nil, entry.err}
	}
	switch funcName {
	case "Guard":
		if entry.guardErr != nil {
			return evalOutcome{nil, entry.guardErr}
		}
		return evalOutcome{entry.guard(contextMap, payloadMap, paramsMap), nil}
	case "Params":
		if entry.paramsErr != nil {
			return evalOutcome{nil, entry.paramsErr}
		}
		return evalOutcome{entry.params(contextMap, payloadMap, paramsMap), nil}
	default:
		return evalOutcome{nil, fmt.Errorf("unknown script entrypoint %q", funcName)}
	}
}

func compileScript(source string) *cachedScript {
	if err := validateScriptShape(source); err != nil {
		return &cachedScript{err: err}
	}
	i := interp.New(interp.Options{})
	if err := i.Use(env.Exports); err != nil {
		return &cachedScript{err: fmt.Errorf("load script env: %w", err)}
	}

	if _, err := i.Eval(source); err != nil {
		return &cachedScript{err: fmt.Errorf("compile script: %w", err)}
	}

	entry := &cachedScript{}
	guardValue, err := i.Eval("script.Guard")
	if err != nil {
		entry.guardErr = fmt.Errorf("resolve script.Guard: %w", err)
	} else if fn, ok := guardValue.Interface().(func(map[string]any, map[string]any, map[string]any) bool); ok {
		entry.guard = fn
	} else {
		entry.guardErr = errors.New("script.Guard must have signature func(context, payload, params map[string]any) bool")
	}
	paramsValue, err := i.Eval("script.Params")
	if err != nil {
		entry.paramsErr = fmt.Errorf("resolve script.Params: %w", err)
	} else if fn, ok := paramsValue.Interface().(func(map[string]any, map[string]any, map[string]any) map[string]any); ok {
		entry.params = fn
	} else {
		entry.paramsErr = errors.New("script.Params must have signature func(context, payload, params map[string]any) map[string]any")
	}
	return entry
}

func validateScriptShape(source string) error {
	file, err := parser.ParseFile(token.NewFileSet(), "script.go", source, parser.AllErrors)
	if err != nil {
		return fmt.Errorf("parse script: %w", err)
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			if declaration.Tok == token.VAR {
				return errors.New("script package-level variables are not allowed")
			}
		case *ast.FuncDecl:
			if declaration.Name.Name == "init" {
				return errors.New("script init functions are not allowed")
			}
			var hasGoStatement bool
			ast.Inspect(declaration, func(node ast.Node) bool {
				if _, ok := node.(*ast.GoStmt); ok {
					hasGoStatement = true
					return false
				}
				return !hasGoStatement
			})
			if hasGoStatement {
				return errors.New("script-spawned goroutines are not allowed")
			}
		}
	}
	return nil
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
