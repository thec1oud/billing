package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
	"github.com/thec1oud/billing/internal/shared/testutil"
)

// seedDefinition creates a minimal two-state, one-transition definition
// (DRAFT --submit--> DONE) and returns its definitionID and transitionID.
func seedDefinition(t *testing.T, ctx context.Context, repo *repository.PostgresRepository, machineType model.MachineType) (definitionID, transitionID uuid.UUID) {
	t.Helper()

	def, err := repo.InsertDefinition(ctx, nil, machineType, 1, "DRAFT", true)
	if err != nil {
		t.Fatalf("insert definition: %v", err)
	}

	if err := repo.InsertState(ctx, nil, def.DefinitionID, model.State{Name: "DRAFT"}); err != nil {
		t.Fatalf("insert state DRAFT: %v", err)
	}
	if err := repo.InsertState(ctx, nil, def.DefinitionID, model.State{Name: "DONE", IsFinal: true}); err != nil {
		t.Fatalf("insert state DONE: %v", err)
	}

	tID, err := repo.InsertTransition(ctx, nil, def.DefinitionID, model.Transition{
		FromState: "DRAFT",
		EventName: "submit",
		ToState:   "DONE",
	})
	if err != nil {
		t.Fatalf("insert transition: %v", err)
	}

	return def.DefinitionID, tID
}

func TestCreateInstance_DuplicateSubject_ReturnsAlreadyExists(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	definitionID, _ := seedDefinition(t, ctx, repo, "dup_subject_machine")

	_, err := repo.CreateInstance(ctx, nil, definitionID, "dup_subject_machine", "invoice", "inv-1", "DRAFT", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	_, err = repo.CreateInstance(ctx, nil, definitionID, "dup_subject_machine", "invoice", "inv-1", "DRAFT", nil)
	if !errors.Is(err, model.ErrInstanceAlreadyExists) {
		t.Fatalf("expected ErrInstanceAlreadyExists, got %v", err)
	}
}

func TestActiveDefinition_OnlyOnePerMachineType(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	const machineType model.MachineType = "single_active_machine"

	if _, err := repo.InsertDefinition(ctx, nil, machineType, 1, "DRAFT", true); err != nil {
		t.Fatalf("insert definition v1: %v", err)
	}

	// Inserting a second active version for the same machine type must violate
	// the partial unique index unless the first is deactivated first.
	if _, err := repo.InsertDefinition(ctx, nil, machineType, 2, "DRAFT", true); err == nil {
		t.Fatal("expected unique-active-version violation, got nil error")
	}

	if err := repo.DeactivateDefinitionsForMachineType(ctx, nil, machineType); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if _, err := repo.InsertDefinition(ctx, nil, machineType, 2, "DRAFT", true); err != nil {
		t.Fatalf("insert definition v2 after deactivation: %v", err)
	}

	active, err := repo.GetActiveDefinition(ctx, nil, machineType)
	if err != nil {
		t.Fatalf("get active definition: %v", err)
	}
	if active.Version != 2 {
		t.Fatalf("expected active version 2, got %d", active.Version)
	}
}

func TestTransition_CompositeFK_RejectsCrossDefinitionState(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	defA, err := repo.InsertDefinition(ctx, nil, "cross_def_machine_a", 1, "DRAFT", false)
	if err != nil {
		t.Fatalf("insert definition A: %v", err)
	}
	if err := repo.InsertState(ctx, nil, defA.DefinitionID, model.State{Name: "DRAFT"}); err != nil {
		t.Fatalf("insert state for A: %v", err)
	}

	defB, err := repo.InsertDefinition(ctx, nil, "cross_def_machine_b", 1, "DRAFT", false)
	if err != nil {
		t.Fatalf("insert definition B: %v", err)
	}
	if err := repo.InsertState(ctx, nil, defB.DefinitionID, model.State{Name: "OTHER"}); err != nil {
		t.Fatalf("insert state for B: %v", err)
	}

	// from_state "OTHER" does not exist under definition A, so the composite FK
	// (definition_id, from_state) -> sm_states must reject this.
	_, err = repo.InsertTransition(ctx, nil, defA.DefinitionID, model.Transition{
		FromState: "OTHER",
		EventName: "submit",
		ToState:   "DRAFT",
	})
	if err == nil {
		t.Fatal("expected composite FK violation, got nil error")
	}
}

func TestActionBinding_CheckConstraint_RejectsInvalidHookShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	definitionID, transitionID := seedDefinition(t, ctx, repo, "action_binding_check_machine")

	// ON_ENTER requires state_name set and transition_id NULL; providing both
	// (or neither) must violate the CHECK constraint.
	_, err := repo.InsertActionBinding(ctx, nil, definitionID, model.ActionBinding{
		HookType:     model.HookOnEnter,
		StateName:    "DRAFT",
		TransitionID: &transitionID,
		Seq:          1,
		ActionName:   "noop",
		ParamsKind:   model.ParamsKindStatic,
		Mode:         model.ActionModeSync,
		OnError:      model.OnErrorAbort,
	})
	if err == nil {
		t.Fatal("expected CHECK constraint violation for ON_ENTER with transition_id set, got nil error")
	}
}

func TestClaimOutboxRows_ConcurrentClaims_NoDoubleClaim(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	definitionID, _ := seedDefinition(t, ctx, repo, "outbox_claim_machine")
	instance, err := repo.CreateInstance(ctx, nil, definitionID, "outbox_claim_machine", "invoice", "inv-claim", "DRAFT", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	const totalRows = 50
	for i := 0; i < totalRows; i++ {
		err := repo.InsertOutboxRow(ctx, nil, instance.InstanceID, nil, nil, "noop", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("insert outbox row %d: %v", i, err)
		}
	}

	const workers = 8
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		claimedIDs  = make(map[int64]int)
		totalClaims int
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				rows, err := repo.ClaimOutboxRows(ctx, nil, 5, "worker")
				if err != nil {
					t.Errorf("claim outbox rows: %v", err)
					return
				}
				if len(rows) == 0 {
					return
				}
				mu.Lock()
				for _, row := range rows {
					claimedIDs[row.OutboxID]++
					totalClaims++
				}
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()

	if totalClaims != totalRows {
		t.Fatalf("expected %d total claims, got %d", totalRows, totalClaims)
	}
	for id, count := range claimedIDs {
		if count != 1 {
			t.Fatalf("outbox row %d claimed %d times, want exactly 1", id, count)
		}
	}
}

func TestReapStuckOutbox_ReclaimsStaleClaim(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()
	repo := repository.NewPostgresRepository(pool)

	definitionID, _ := seedDefinition(t, ctx, repo, "outbox_reap_machine")
	instance, err := repo.CreateInstance(ctx, nil, definitionID, "outbox_reap_machine", "invoice", "inv-reap", "DRAFT", nil)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if err := repo.InsertOutboxRow(ctx, nil, instance.InstanceID, nil, nil, "noop", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("insert outbox row: %v", err)
	}

	claimed, err := repo.ClaimOutboxRows(ctx, nil, 1, "stale-worker")
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim outbox row: rows=%d err=%v", len(claimed), err)
	}

	// Reaping with a cutoff in the future should reclaim the just-claimed row
	// (simulates a worker crash: claimed_at is older than "now").
	n, err := repo.ReapStuckOutbox(ctx, nil, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatalf("reap stuck outbox: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row reaped, got %d", n)
	}

	reclaimed, err := repo.ClaimOutboxRows(ctx, nil, 1, "second-worker")
	if err != nil {
		t.Fatalf("claim after reap: %v", err)
	}
	if len(reclaimed) != 1 || reclaimed[0].OutboxID != claimed[0].OutboxID {
		t.Fatalf("expected reaped row to be claimable again, got %+v", reclaimed)
	}
}
