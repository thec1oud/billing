package scripting

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

func TestZZTimedOutEvaluationsRetainCapacityUntilTheyExit(t *testing.T) {
	const concurrency = 2
	pool := NewPool(concurrency, 30*time.Millisecond)
	start := make(chan struct{})
	errs := make(chan error, concurrency)

	for i := range concurrency {
		source := fmt.Sprintf(`package script
// unique source %d
func Guard(context, payload, params map[string]any) bool {
	for { }
}`, i)
		go func() {
			<-start
			_, err := pool.EvalGuard(context.Background(), source, model.ExecutionContext{}, nil)
			errs <- err
		}()
	}
	close(start)

	for range concurrency {
		if err := <-errs; !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected a script timeout, got %v", err)
		}
	}
	if got := len(pool.sem); got != concurrency {
		t.Fatalf("timed-out interpreters released their slots: got %d active, want %d", got, concurrency)
	}

	_, err := pool.EvalGuard(context.Background(), `package script
func Guard(context, payload, params map[string]any) bool { return true }`, model.ExecutionContext{}, nil)
	if !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("expected capacity rejection while interpreters remain active, got %v", err)
	}
}

func TestPoolCachesCompiledScriptByHash(t *testing.T) {
	pool := NewPool(2, time.Second)
	source := `package script
func Guard(context, payload, params map[string]any) bool { return true }`
	for range 2 {
		if _, err := pool.EvalGuard(context.Background(), source, model.ExecutionContext{}, nil); err != nil {
			t.Fatalf("evaluate script: %v", err)
		}
	}
	pool.cacheMu.Lock()
	got := len(pool.cache)
	pool.cacheMu.Unlock()
	if got != 1 {
		t.Fatalf("compiled-script cache has %d entries, want 1", got)
	}
}
