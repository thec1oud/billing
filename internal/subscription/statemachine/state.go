package statemachine

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/subscription/model"
	"github.com/thec1oud/billing/internal/subscription/repository"
)

const SubscriptionMachineType = "subscription_lifecycle"

const (
	EventPause  sm_model.EventName = "pause"
	EventResume sm_model.EventName = "resume"
	EventCancel sm_model.EventName = "cancel"
)

//go:embed state_machine.json
var subscriptionStateMachineJSON []byte

// BuildSubscriptionDefinitionSpec returns the Version 1 blueprint parsed from embedded JSON
func BuildSubscriptionDefinitionSpec() loader.DefinitionSpec {
	var spec loader.DefinitionSpec
	if err := json.Unmarshal(subscriptionStateMachineJSON, &spec); err != nil {
		panic(fmt.Errorf("failed to unmarshal embedded subscription state machine: %w", err))
	}
	return spec
}

// RegisterStateMachineActions registers the Subscription-related Database Actions into the global registry
func RegisterStateMachineActions(
	reg *registry.Registry,
	repo *repository.Repository,
) {
	_ = reg.RegisterAction(&PauseAction{repo: repo})
	_ = reg.RegisterAction(&ResumeAction{repo: repo})
	_ = reg.RegisterAction(&CancelAction{repo: repo})
}

// --- Action Implementation: Pause ---

type PauseAction struct {
	repo *repository.Repository
}

func (a *PauseAction) Name() string { return "subscription.action.pause" }

func (a *PauseAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse subscription ID %q: %w", ec.SubjectID, err)
	}

	err := a.repo.Transition(ctx, db, intID, model.StatusActive, model.StatusPaused, ec.Now)
	if err != nil {
		return err
	}

	newCtx, err := json.Marshal(map[string]any{
		"subscription_id": intID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}

// --- Action Implementation: Resume ---

type ResumeAction struct {
	repo *repository.Repository
}

func (a *ResumeAction) Name() string { return "subscription.action.resume" }

func (a *ResumeAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse subscription ID %q: %w", ec.SubjectID, err)
	}

	err := a.repo.Transition(ctx, db, intID, model.StatusPaused, model.StatusActive, ec.Now)
	if err != nil {
		return err
	}

	newCtx, err := json.Marshal(map[string]any{
		"subscription_id": intID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}

// --- Action Implementation: Cancel ---

type CancelAction struct {
	repo *repository.Repository
}

func (a *CancelAction) Name() string { return "subscription.action.cancel" }

func (a *CancelAction) Execute(
	ctx context.Context, db sqlcgen.DBTX, ec *sm_model.ExecutionContext, params json.RawMessage,
) error {
	var intID int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &intID); err != nil {
		return fmt.Errorf("failed to parse subscription ID %q: %w", ec.SubjectID, err)
	}

	// Default to immediate cancellation unless specified in payload
	var p struct {
		AtPeriodEnd bool `json:"at_period_end"`
	}
	if len(ec.EventPayload) > 0 {
		_ = json.Unmarshal(ec.EventPayload, &p)
	}

	err := a.repo.Cancel(ctx, db, intID, p.AtPeriodEnd, ec.Now)
	if err != nil {
		return err
	}

	newCtx, err := json.Marshal(map[string]any{
		"subscription_id": intID,
		"at_period_end":   p.AtPeriodEnd,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal new context: %w", err)
	}
	ec.Context = newCtx
	return nil
}
