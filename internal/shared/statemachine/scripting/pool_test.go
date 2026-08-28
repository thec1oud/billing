package scripting_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
)

const guardScript = `
package script

func Guard(context, payload, params map[string]any) bool {
	amount, _ := context["amount"].(float64)
	limit, _ := params["limit"].(float64)
	return amount <= limit
}
`

const paramsScript = `
package script

func Params(context, payload, params map[string]any) map[string]any {
	return map[string]any{
		"invoice_id": context["invoice_id"],
		"amount":     context["amount"],
	}
}
`

const envHelperGuardScript = `
package script

import "statemachine/env"

func Guard(context, payload, params map[string]any) bool {
	name, _ := context["name"].(string)
	return env.HasPrefix(env.ToUpper(name), "VIP")
}
`

const dangerousScript = `
package script

import "os"

func Guard(context, payload, params map[string]any) bool {
	os.Exit(1)
	return true
}
`

const infiniteLoopScript = `
package script

func Guard(context, payload, params map[string]any) bool {
	for {
	}
}
`

func TestEvalGuard_ValidScript_EvaluatesCorrectly(t *testing.T) {
	pool := scripting.NewPool(4, 200*time.Millisecond)
	ec := model.ExecutionContext{Context: json.RawMessage(`{"amount": 50}`)}
	params := json.RawMessage(`{"limit": 100}`)

	pass, err := pool.EvalGuard(context.Background(), guardScript, ec, params)
	if err != nil {
		t.Fatalf("eval guard: %v", err)
	}
	if !pass {
		t.Fatal("expected guard to pass (amount 50 <= limit 100)")
	}

	ec.Context = json.RawMessage(`{"amount": 500}`)
	pass, err = pool.EvalGuard(context.Background(), guardScript, ec, params)
	if err != nil {
		t.Fatalf("eval guard: %v", err)
	}
	if pass {
		t.Fatal("expected guard to fail (amount 500 > limit 100)")
	}
}

func TestEvalGuard_UsesInjectedEnvHelpers(t *testing.T) {
	pool := scripting.NewPool(4, 200*time.Millisecond)
	ec := model.ExecutionContext{Context: json.RawMessage(`{"name": "vip_customer"}`)}

	pass, err := pool.EvalGuard(context.Background(), envHelperGuardScript, ec, nil)
	if err != nil {
		t.Fatalf("eval guard: %v", err)
	}
	if !pass {
		t.Fatal("expected guard to pass using env.HasPrefix/env.ToUpper")
	}
}

func TestEvalGuard_DangerousImport_FailsToCompile(t *testing.T) {
	pool := scripting.NewPool(4, 200*time.Millisecond)

	_, err := pool.EvalGuard(context.Background(), dangerousScript, model.ExecutionContext{}, nil)
	if err == nil {
		t.Fatal("expected a script importing \"os\" to fail to compile, since it's never exported to the interpreter")
	}
	if !strings.Contains(err.Error(), "compile script") {
		t.Fatalf("expected a compile error, got: %v", err)
	}
}

func TestEvalGuard_TimesOutWithoutHanging(t *testing.T) {
	pool := scripting.NewPool(4, 30*time.Millisecond)

	start := time.Now()
	_, err := pool.EvalGuard(context.Background(), infiniteLoopScript, model.ExecutionContext{}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected an infinite-loop script to return a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("EvalGuard did not return promptly on timeout, took %s", elapsed)
	}
}

func TestEvalActionParams_RoundTripsThroughJSON(t *testing.T) {
	pool := scripting.NewPool(4, 200*time.Millisecond)
	ec := model.ExecutionContext{Context: json.RawMessage(`{"invoice_id": "inv-1", "amount": 42}`)}

	result, err := pool.EvalActionParams(context.Background(), paramsScript, ec, nil)
	if err != nil {
		t.Fatalf("eval action params: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if decoded["invoice_id"] != "inv-1" {
		t.Fatalf("expected invoice_id=inv-1, got %v", decoded["invoice_id"])
	}
	if decoded["amount"] != float64(42) {
		t.Fatalf("expected amount=42, got %v", decoded["amount"])
	}
}

func TestEvalGuard_ConcurrentEvals_NoCrossContamination(t *testing.T) {
	pool := scripting.NewPool(4, 200*time.Millisecond)

	const workers = 20
	var wg sync.WaitGroup
	errs := make([]error, workers)
	results := make([]bool, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			amount := 10
			if i%2 == 0 {
				amount = 1000
			}
			ec := model.ExecutionContext{Context: json.RawMessage(fmt.Sprintf(`{"amount": %d}`, amount))}
			pass, err := pool.EvalGuard(context.Background(), guardScript, ec, json.RawMessage(`{"limit": 100}`))
			results[i] = pass
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: unexpected error: %v", i, err)
		}
		wantPass := i%2 != 0 // amount=10 -> pass; amount=1000 -> fail
		if results[i] != wantPass {
			t.Fatalf("worker %d: expected pass=%v, got %v (cross-contamination between concurrent evals)", i, wantPass, results[i])
		}
	}
}
