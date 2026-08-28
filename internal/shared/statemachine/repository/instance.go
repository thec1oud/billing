package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

func (r *PostgresRepository) CreateInstance(
	ctx context.Context, db DBTX,
	definitionID uuid.UUID, machineType model.MachineType, subjectType, subjectID string,
	initialState model.StateName, initialContext json.RawMessage,
) (model.Instance, error) {
	q := r.getQuerier(db)

	ctxBytes := nonEmptyJSON(initialContext)
	row, err := q.CreateInstance(ctx, sqlcgen.CreateInstanceParams{
		DefinitionID: definitionID,
		MachineType:  string(machineType),
		SubjectType:  subjectType,
		SubjectID:    subjectID,
		CurrentState: string(initialState),
		Context:      ctxBytes,
	})
	if err != nil {
		return model.Instance{}, r.handleError(err, "create instance")
	}

	return model.Instance{
		InstanceID:   row.InstanceID,
		DefinitionID: definitionID,
		MachineType:  machineType,
		SubjectType:  subjectType,
		SubjectID:    subjectID,
		CurrentState: initialState,
		Context:      ctxBytes,
		Version:      row.Version,
		Status:       model.InstanceStatus(row.Status),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}

// GetInstanceForUpdate takes SELECT ... FOR UPDATE. db must be an active
// transaction for the lock to have any effect beyond the single statement.
func (r *PostgresRepository) GetInstanceForUpdate(ctx context.Context, db DBTX, instanceID uuid.UUID) (model.Instance, error) {
	q := r.getQuerier(db)

	row, err := q.GetInstanceForUpdate(ctx, instanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Instance{}, model.ErrInstanceNotFound
	}
	if err != nil {
		return model.Instance{}, fmt.Errorf("get instance for update: %w", err)
	}
	return mapInstance(row), nil
}

func (r *PostgresRepository) GetInstanceByID(ctx context.Context, db DBTX, instanceID uuid.UUID) (model.Instance, error) {
	q := r.getQuerier(db)

	row, err := q.GetInstanceByID(ctx, instanceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Instance{}, model.ErrInstanceNotFound
	}
	if err != nil {
		return model.Instance{}, fmt.Errorf("get instance by id: %w", err)
	}
	return mapInstance(row), nil
}

//nolint:lll // Kept together for readability.
func (r *PostgresRepository) GetInstanceBySubject(ctx context.Context, db DBTX, subjectType, subjectID string, machineType model.MachineType) (model.Instance, error) {
	q := r.getQuerier(db)

	row, err := q.GetInstanceBySubject(ctx, sqlcgen.GetInstanceBySubjectParams{
		SubjectType: subjectType,
		SubjectID:   subjectID,
		MachineType: string(machineType),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Instance{}, model.ErrInstanceNotFound
	}
	if err != nil {
		return model.Instance{}, fmt.Errorf("get instance by subject: %w", err)
	}
	return mapInstance(row), nil
}

// UpdateInstanceState applies the atomic WHERE instance_id=$1 AND version=$2
// compare-and-swap. Returns rows affected; under the FOR UPDATE lock taken by
// GetInstanceForUpdate earlier in the same transaction, 0 should never happen in
// practice — callers should treat that as ErrOptimisticConflict, not a retry path.
func (r *PostgresRepository) UpdateInstanceState(
	ctx context.Context, db DBTX,
	instanceID uuid.UUID, expectedVersion int64,
	newState model.StateName, newContext json.RawMessage, newStatus model.InstanceStatus,
) (int64, error) {
	q := r.getQuerier(db)

	rowsAffected, err := q.UpdateInstanceState(ctx, sqlcgen.UpdateInstanceStateParams{
		InstanceID:   instanceID,
		Version:      expectedVersion,
		CurrentState: string(newState),
		Context:      nonEmptyJSON(newContext),
		Status:       string(newStatus),
	})
	if err != nil {
		return 0, fmt.Errorf("update instance state: %w", err)
	}
	return rowsAffected, nil
}

func mapInstance(row sqlcgen.SmInstance) model.Instance {
	return model.Instance{
		InstanceID:   row.InstanceID,
		DefinitionID: row.DefinitionID,
		MachineType:  model.MachineType(row.MachineType),
		SubjectType:  row.SubjectType,
		SubjectID:    row.SubjectID,
		CurrentState: model.StateName(row.CurrentState),
		Context:      row.Context,
		Version:      row.Version,
		Status:       model.InstanceStatus(row.Status),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

// TransitionHistoryPending is the row inserted before a transition's hooks run
// (see engine.Fire step 10); FinalizeTransitionHistory fills in ContextAfter once
// the hooks have completed.
type TransitionHistoryPending struct {
	InstanceID    uuid.UUID
	TransitionID  *uuid.UUID
	FromState     model.StateName // empty for instance-creation rows
	ToState       model.StateName
	EventName     model.EventName // empty for instance-creation rows
	EventPayload  json.RawMessage
	ContextBefore json.RawMessage
	TriggeredBy   string
}

//nolint:lll // Kept together for readability.
func (r *PostgresRepository) InsertTransitionHistoryPending(ctx context.Context, db DBTX, h TransitionHistoryPending) (historyID int64, occurredAt time.Time, err error) {
	q := r.getQuerier(db)

	row, err := q.InsertTransitionHistoryPending(ctx, sqlcgen.InsertTransitionHistoryPendingParams{
		InstanceID:    h.InstanceID,
		TransitionID:  h.TransitionID,
		FromState:     textOrNil(string(h.FromState)),
		ToState:       string(h.ToState),
		EventName:     textOrNil(string(h.EventName)),
		EventPayload:  h.EventPayload,
		ContextBefore: h.ContextBefore,
		TriggeredBy:   textOrNil(h.TriggeredBy),
	})
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("insert transition history pending: %w", err)
	}
	return row.HistoryID, row.OccurredAt, nil
}

func (r *PostgresRepository) FinalizeTransitionHistory(ctx context.Context, db DBTX, historyID int64, contextAfter json.RawMessage) error {
	q := r.getQuerier(db)

	err := q.FinalizeTransitionHistory(ctx, sqlcgen.FinalizeTransitionHistoryParams{
		HistoryID:    historyID,
		ContextAfter: contextAfter,
	})
	if err != nil {
		return fmt.Errorf("finalize transition history: %w", err)
	}
	return nil
}
