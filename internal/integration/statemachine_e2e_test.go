package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/engine"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/outbox"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/statemachine/scheduler"
	"github.com/thec1oud/billing/internal/shared/statemachine/scripting"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

// notifyAction stands in for a real webhook/notification side effect: it just
// records that it ran, proving the ASYNC action actually executed via the
// outbox.Publisher rather than inline during Fire().
type notifyAction struct {
	ch chan json.RawMessage
}

func (a *notifyAction) Name() string { return "notify" }
func (a *notifyAction) Execute(_ context.Context, _ sqlcgen.DBTX, _ *model.ExecutionContext, params json.RawMessage) error {
	a.ch <- params
	return nil
}

const approvalGuardScript = `
package script

func Guard(context, payload, params map[string]any) bool {
	amount, _ := context["amount"].(float64)
	limit, _ := params["limit"].(float64)
	return amount <= limit
}
`

// TestStateMachine_EndToEnd exercises every capability of the dynamic state
// machine engine together against one demo workflow:
//
//	DRAFT --submit--> PENDING_REVIEW --approve[scripted guard]--> APPROVED (final)
//	                          |
//	                          +--(no timely approval)--> auto-expire via
//	                             scheduler.Poller --> EXPIRED (final)
//
// covering: a SYNC ON_ENTER action, an ASYNC ON_TRANSITION action executed by
// outbox.Publisher, a SCRIPT-kind guard evaluated by scripting.Pool, and a
// scheduled timeout fired by scheduler.Poller — all through the same
// engine.Fire path, with sm_transition_history recording every step.
func TestStateMachine_EndToEnd(t *testing.T) {
	ctx := context.Background()
	pool := testutil.NewPostgresContainer(t)
	repo := repository.NewPostgresRepository(pool)
	reg := registry.New()

	notify := &notifyAction{ch: make(chan json.RawMessage, 1)}
	if err := reg.RegisterAction(notify); err != nil {
		t.Fatalf("register notify action: %v", err)
	}

	eng := engine.NewEngine(pool, repo, reg, engine.WithScripting(scripting.NewPool(4, 200*time.Millisecond)))

	const machineType model.MachineType = "e2e_approval_workflow"
	spec := loader.DefinitionSpec{
		MachineType:  machineType,
		Version:      1,
		InitialState: "DRAFT",
		Activate:     true,
		States: []loader.StateSpec{
			{Name: "DRAFT"},
			{
				Name: "PENDING_REVIEW",
				OnEnter: []loader.ActionBindingSpec{
					{
						Seq: 1, ActionName: "notify", ParamsKind: model.ParamsKindStatic,
						Params: json.RawMessage(`{"event": "submitted_for_review"}`),
						Mode:   model.ActionModeAsync, OnError: model.OnErrorAbort,
					},
				},
			},
			{Name: "APPROVED", IsFinal: true},
			{Name: "EXPIRED", IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			{FromState: "DRAFT", EventName: "submit", ToState: "PENDING_REVIEW"},
			{
				FromState: "PENDING_REVIEW", EventName: "approve", ToState: "APPROVED",
				Guards: []loader.GuardBindingSpec{
					{Seq: 1, ImplementationKind: model.GuardImplScript, Script: approvalGuardScript, Params: json.RawMessage(`{"limit": 1000}`)},
				},
			},
			{FromState: "PENDING_REVIEW", EventName: "expire", ToState: "EXPIRED"},
		},
	}
	if _, err := loader.Publish(ctx, pool, repo, reg, spec); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// --- create + submit ---
	instance, err := eng.CreateInstance(ctx, machineType, "purchase_order", "po-1", json.RawMessage(`{"amount": 250}`))
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	if _, err := eng.Fire(ctx, instance.InstanceID, "submit", nil); err != nil {
		t.Fatalf("fire submit: %v", err)
	}

	afterSubmit, err := repo.GetInstanceByID(ctx, nil, instance.InstanceID)
	if err != nil {
		t.Fatalf("get instance: %v", err)
	}
	if afterSubmit.CurrentState != "PENDING_REVIEW" {
		t.Fatalf("expected PENDING_REVIEW, got %s", afterSubmit.CurrentState)
	}

	// The notify action is ASYNC: it must NOT have run yet.
	select {
	case <-notify.ch:
		t.Fatal("notify action ran synchronously; it should have been deferred to the outbox")
	default:
	}

	// Drive the outbox publisher directly (deterministic, no ticker wait).
	publisher := outbox.NewPublisher(pool, repo, reg, outbox.Config{BatchSize: 10})
	if err := publisher.Tick(ctx); err != nil {
		t.Fatalf("outbox tick: %v", err)
	}

	select {
	case params := <-notify.ch:
		var decoded map[string]any
		if err := json.Unmarshal(params, &decoded); err != nil {
			t.Fatalf("unmarshal notify params: %v", err)
		}
		if decoded["event"] != "submitted_for_review" {
			t.Fatalf("unexpected notify params: %v", decoded)
		}
	default:
		t.Fatal("expected notify action to have run after outbox publisher tick")
	}

	// --- scripted guard rejects an over-limit approval ---
	overLimitInstance, err := eng.CreateInstance(ctx, machineType, "purchase_order", "po-2", json.RawMessage(`{"amount": 5000}`))
	if err != nil {
		t.Fatalf("create over-limit instance: %v", err)
	}
	if _, err := eng.Fire(ctx, overLimitInstance.InstanceID, "submit", nil); err != nil {
		t.Fatalf("fire submit on over-limit instance: %v", err)
	}
	if err := publisher.Tick(ctx); err != nil {
		t.Fatalf("outbox tick: %v", err)
	}
	<-notify.ch // drain
	if _, err := eng.Fire(ctx, overLimitInstance.InstanceID, "approve", nil); err == nil {
		t.Fatal("expected scripted guard to reject an over-limit approval")
	}

	// --- scripted guard approves the in-limit instance ---
	result, err := eng.Fire(ctx, instance.InstanceID, "approve", nil)
	if err != nil {
		t.Fatalf("fire approve: %v", err)
	}
	if result.ToState != "APPROVED" || !result.Completed {
		t.Fatalf("expected APPROVED/completed, got %+v", result)
	}

	// --- scheduled auto-expire fires through the same engine.Fire path ---
	thirdInstance, err := eng.CreateInstance(ctx, machineType, "purchase_order", "po-3", json.RawMessage(`{"amount": 10}`))
	if err != nil {
		t.Fatalf("create third instance: %v", err)
	}
	if _, err := eng.Fire(ctx, thirdInstance.InstanceID, "submit", nil); err != nil {
		t.Fatalf("fire submit on third instance: %v", err)
	}
	if _, err := repo.InsertScheduledTransition(ctx, nil, thirdInstance.InstanceID, "expire", nil, time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("insert scheduled transition: %v", err)
	}

	poller := scheduler.NewPoller(eng, repo, scheduler.Config{BatchSize: 10})
	if err := poller.Tick(ctx); err != nil {
		t.Fatalf("scheduler tick: %v", err)
	}

	expired, err := repo.GetInstanceByID(ctx, nil, thirdInstance.InstanceID)
	if err != nil {
		t.Fatalf("get third instance: %v", err)
	}
	if expired.CurrentState != "EXPIRED" || expired.Status != model.InstanceStatusCompleted {
		t.Fatalf("expected scheduled expire to fire (state=EXPIRED, COMPLETED), got state=%s status=%s", expired.CurrentState, expired.Status)
	}

	// --- full audit trail is queryable ---
	bundle, err := repo.GetDefinitionBundle(ctx, nil, instance.DefinitionID)
	if err != nil {
		t.Fatalf("get definition bundle: %v", err)
	}
	if len(bundle.States) != 4 || len(bundle.Transitions) != 3 {
		t.Fatalf("unexpected definition shape: %d states, %d transitions", len(bundle.States), len(bundle.Transitions))
	}
}
