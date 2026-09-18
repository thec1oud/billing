package statemachine

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/thec1oud/billing/internal/account/model"
	"github.com/thec1oud/billing/internal/account/repository"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
)

const AccountMachineType = "account_lifecycle"

//go:embed state_machine.json
var accountStateMachineJSON []byte

// BuildAccountDefinitionSpec returns the Version 1 blueprint parsed from embedded JSON
func BuildAccountDefinitionSpec() loader.DefinitionSpec {
	var spec loader.DefinitionSpec
	if err := json.Unmarshal(accountStateMachineJSON, &spec); err != nil {
		panic(fmt.Errorf("failed to unmarshal embedded account state machine: %w", err))
	}
	return spec
}

// RegisterStateMachineActions registers the Account-related Database Actions into the global registry
func RegisterStateMachineActions(
	reg *registry.Registry,
	repo *repository.Repository,
) {
	_ = reg.RegisterAction(&ActivateAction{repo: repo})
	_ = reg.RegisterAction(&ReactivateAction{repo: repo})
	_ = reg.RegisterAction(&SuspendAction{repo: repo})
	_ = reg.RegisterAction(&CloseAction{repo: repo})
}

// --- Action Implementation: Activate ---

type ActivateAction struct {
	repo *repository.Repository
}

func (a *ActivateAction) Name() string { return "account.action.activate" }

func (a *ActivateAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse account ID %q: %w", ec.SubjectID, err)
	}

	// In the state machine transition, from state is PENDING_VERIFICATION and to is ACTIVE
	err := a.repo.UpdateStatus(ctx, db, intID, model.StatusPendingVerification, model.StatusActive)
	if err != nil {
		return err
	}

	newCtx, err := json.Marshal(map[string]any{
		"account_id": intID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}

// --- Action Implementation: Reactivate ---

type ReactivateAction struct {
	repo *repository.Repository
}

func (a *ReactivateAction) Name() string { return "account.action.reactivate" }

func (a *ReactivateAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse account ID %q: %w", ec.SubjectID, err)
	}

	err := a.repo.UpdateStatus(ctx, db, intID, model.StatusSuspended, model.StatusActive)
	if err != nil {
		return err
	}

	newCtx, err := json.Marshal(map[string]any{
		"account_id": intID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}

// --- Action Implementation: Suspend ---

type SuspendAction struct {
	repo *repository.Repository
}

func (a *SuspendAction) Name() string { return "account.action.suspend" }

func (a *SuspendAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var p struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(ec.EventPayload, &p); err != nil {
		return fmt.Errorf("failed to parse suspend payload: %w", err)
	}

	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse account ID %q: %w", ec.SubjectID, err)
	}

	err := a.repo.UpdateStatus(ctx, db, intID, model.StatusActive, model.StatusSuspended)
	if err != nil {
		return err
	}

	newCtx, err := json.Marshal(map[string]any{
		"account_id": intID,
		"reason":     p.Reason,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}

// --- Action Implementation: Close ---

type CloseAction struct {
	repo *repository.Repository
}

func (a *CloseAction) Name() string { return "account.action.close" }

func (a *CloseAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var p struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(ec.EventPayload, &p); err != nil {
		return fmt.Errorf("failed to parse close payload: %w", err)
	}

	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse account ID %q: %w", ec.SubjectID, err)
	}

	// Close action can happen from ACTIVE or SUSPENDED. We can fetch the current state to call UpdateStatusTx.
	// We can use GetTx to get current status.
	account, err := a.repo.Get(ctx, db, intID)
	if err != nil {
		return err
	}

	err = a.repo.UpdateStatus(ctx, db, intID, account.Status, model.StatusClosed)
	if err != nil {
		return err
	}

	newCtx, err := json.Marshal(map[string]any{
		"account_id": intID,
		"reason":     p.Reason,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}
