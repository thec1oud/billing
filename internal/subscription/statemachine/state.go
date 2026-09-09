package statemachine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/loader"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/subscription/model"
	"github.com/thec1oud/billing/internal/subscription/repository"
)

const SubscriptionMachineType sm_model.MachineType = "subscription_lifecycle"

const (
	EventPause  sm_model.EventName = "pause"
	EventResume sm_model.EventName = "resume"
	EventCancel sm_model.EventName = "cancel"
)

func BuildSubscriptionDefinitionSpec() loader.DefinitionSpec {
	return loader.DefinitionSpec{
		MachineType:  SubscriptionMachineType,
		Version:      1,
		Activate:     true,
		InitialState: sm_model.StateName(model.StatusActive),
		States: []loader.StateSpec{
			{Name: sm_model.StateName(model.StatusActive)},
			{Name: sm_model.StateName(model.StatusPaused)},
			{Name: sm_model.StateName(model.StatusCanceled), IsFinal: true},
		},
		Transitions: []loader.TransitionSpec{
			transition(model.StatusActive, EventPause, model.StatusPaused),
			transition(model.StatusPaused, EventResume, model.StatusActive),
			transition(model.StatusActive, EventCancel, model.StatusCanceled),
			transition(model.StatusPaused, EventCancel, model.StatusCanceled),
		},
	}
}

func transition(from model.Status, event sm_model.EventName, to model.Status) loader.TransitionSpec {
	params, _ := json.Marshal(map[string]string{"to": string(to)})
	return loader.TransitionSpec{
		FromState: sm_model.StateName(from),
		EventName: event,
		ToState:   sm_model.StateName(to),
		Actions: []loader.ActionBindingSpec{{
			Seq:        1,
			ActionName: "subscription.transition",
			ParamsKind: sm_model.ParamsKindStatic,
			Params:     params,
			Mode:       sm_model.ActionModeSync,
			OnError:    sm_model.OnErrorAbort,
		}},
	}
}

func RegisterStateMachineActions(reg *registry.Registry, repo *repository.Repository) {
	_ = reg.RegisterAction(&transitionAction{repo: repo})
}

type transitionAction struct {
	repo *repository.Repository
}

func (a *transitionAction) Name() string { return "subscription.transition" }

func (a *transitionAction) Execute(
	ctx context.Context,
	db sqlcgen.DBTX,
	ec *sm_model.ExecutionContext,
	params json.RawMessage,
) error {
	var input struct {
		To model.Status `json:"to"`
	}
	if err := json.Unmarshal(params, &input); err != nil {
		return fmt.Errorf("decode subscription transition params: %w", err)
	}

	var id int64
	if _, err := fmt.Sscanf(ec.SubjectID, "%d", &id); err != nil {
		return fmt.Errorf("parse subscription id %q: %w", ec.SubjectID, err)
	}

	if err := a.repo.TransitionTx(ctx, db, id, model.Status(ec.FromState), input.To, ec.Now); err != nil {
		return err
	}

	ec.Context, _ = json.Marshal(map[string]any{
		"subscription_id": id,
		"from_status":     ec.FromState,
		"status":          input.To,
		"transitioned_at": ec.Now,
	})
	return nil
}
