package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

// CanFire reports whether eventName has at least one guard-passing candidate
// transition from the instance's current state. This is a best-effort,
// read-only introspection query: it reads the instance via the pool (no row
// lock), so the answer can be stale by the time a caller actually calls Fire().
func (e *Engine) CanFire(ctx context.Context, instanceID uuid.UUID, eventName model.EventName) (bool, error) {
	instance, err := e.repo.GetInstanceByID(ctx, nil, instanceID)
	if err != nil {
		return false, err
	}
	if instance.Status != model.InstanceStatusRunning {
		return false, nil
	}

	def, err := e.loadDefinition(ctx, instance.DefinitionID)
	if err != nil {
		return false, err
	}

	candidates := def.TransitionsFor(instance.CurrentState, eventName)
	if len(candidates) == 0 {
		return false, nil
	}

	ec := model.ExecutionContext{
		InstanceID:  instance.InstanceID,
		SubjectType: instance.SubjectType,
		SubjectID:   instance.SubjectID,
		FromState:   instance.CurrentState,
		EventName:   eventName,
		Context:     instance.Context,
		Now:         time.Now(),
	}

	selected, err := e.selectTransition(ctx, candidates, ec)
	if err != nil {
		return false, fmt.Errorf("evaluate guards for CanFire: %w", err)
	}
	return selected != nil, nil
}

// AvailableEvents returns the distinct event names with at least one transition
// defined from the instance's current state, regardless of whether their guards
// currently pass. Read-only, best-effort (see CanFire).
func (e *Engine) AvailableEvents(ctx context.Context, instanceID uuid.UUID) ([]model.EventName, error) {
	instance, err := e.repo.GetInstanceByID(ctx, nil, instanceID)
	if err != nil {
		return nil, err
	}

	def, err := e.loadDefinition(ctx, instance.DefinitionID)
	if err != nil {
		return nil, err
	}

	return def.EventsFrom(instance.CurrentState), nil
}

// GetInstanceBySubject resolves a running or completed instance by its subject mapping.
func (e *Engine) GetInstanceBySubject(ctx context.Context, subjectType, subjectID string, machineType model.MachineType) (model.Instance, error) {
	return e.repo.GetInstanceBySubject(ctx, nil, subjectType, subjectID, machineType)
}
