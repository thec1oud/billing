// Package repository is the Postgres persistence layer for the state machine
// engine: definitions/states/transitions/bindings, running instances, transition
// history, and the outbox/schedule tables backing async actions and timers.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/shared/statemachine/model"
)

type DBTX interface {
	sqlcgen.DBTX
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) getQuerier(db DBTX) *sqlcgen.Queries {
	if db != nil {
		return sqlcgen.New(db)
	}
	return sqlcgen.New(r.pool)
}

func (r *PostgresRepository) handleError(err error, op string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			switch pgErr.ConstraintName {
			case "sm_instances_subject_type_subject_id_machine_type_key":
				return fmt.Errorf("%s: %w", op, model.ErrInstanceAlreadyExists)
			}
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

func textOrNil(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textValue(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func nonEmptyJSON(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(`{}`)
	}
	return b
}

// --- Definitions ---

func (r *PostgresRepository) InsertDefinition(
	ctx context.Context, db DBTX,
	machineType model.MachineType, version int, initialState model.StateName, isActive bool,
) (model.DefinitionMeta, error) {
	q := r.getQuerier(db)

	row, err := q.InsertDefinition(ctx, sqlcgen.InsertDefinitionParams{
		MachineType:  string(machineType),
		Version:      int32(version),
		InitialState: string(initialState),
		IsActive:     isActive,
	})
	if err != nil {
		return model.DefinitionMeta{}, r.handleError(err, "insert definition")
	}

	return model.DefinitionMeta{
		DefinitionID: row.DefinitionID,
		MachineType:  machineType,
		Version:      version,
		InitialState: initialState,
		IsActive:     isActive,
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *PostgresRepository) DeactivateDefinitionsForMachineType(ctx context.Context, db DBTX, machineType model.MachineType) error {
	q := r.getQuerier(db)
	if err := q.DeactivateDefinitionsForMachineType(ctx, string(machineType)); err != nil {
		return fmt.Errorf("deactivate definitions for machine type: %w", err)
	}
	return nil
}

//nolint:lll // Kept together for readability.
func (r *PostgresRepository) GetActiveDefinition(ctx context.Context, db DBTX, machineType model.MachineType) (model.DefinitionMeta, error) {
	q := r.getQuerier(db)

	row, err := q.GetActiveDefinition(ctx, string(machineType))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DefinitionMeta{}, model.ErrDefinitionNotFound
	}
	if err != nil {
		return model.DefinitionMeta{}, fmt.Errorf("get active definition: %w", err)
	}

	return mapDefinitionMeta(row), nil
}

func (r *PostgresRepository) GetDefinitionByID(ctx context.Context, db DBTX, definitionID uuid.UUID) (model.DefinitionMeta, error) {
	q := r.getQuerier(db)

	row, err := q.GetDefinitionByID(ctx, definitionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DefinitionMeta{}, model.ErrDefinitionNotFound
	}
	if err != nil {
		return model.DefinitionMeta{}, fmt.Errorf("get definition by id: %w", err)
	}

	return mapDefinitionMeta(row), nil
}

func mapDefinitionMeta(row sqlcgen.SmDefinition) model.DefinitionMeta {
	return model.DefinitionMeta{
		DefinitionID: row.DefinitionID,
		MachineType:  model.MachineType(row.MachineType),
		Version:      int(row.Version),
		InitialState: model.StateName(row.InitialState),
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
	}
}

// --- States ---

func (r *PostgresRepository) InsertState(ctx context.Context, db DBTX, definitionID uuid.UUID, s model.State) error {
	q := r.getQuerier(db)

	err := q.InsertState(ctx, sqlcgen.InsertStateParams{
		DefinitionID: definitionID,
		StateName:    string(s.Name),
		IsFinal:      s.IsFinal,
		Metadata:     nonEmptyJSON(s.Metadata),
	})
	if err != nil {
		return r.handleError(err, "insert state")
	}
	return nil
}

func (r *PostgresRepository) ListStatesByDefinition(ctx context.Context, db DBTX, definitionID uuid.UUID) ([]model.State, error) {
	q := r.getQuerier(db)

	rows, err := q.ListStatesByDefinition(ctx, definitionID)
	if err != nil {
		return nil, fmt.Errorf("list states by definition: %w", err)
	}

	states := make([]model.State, 0, len(rows))
	for _, row := range rows {
		states = append(states, model.State{
			Name:     model.StateName(row.StateName),
			IsFinal:  row.IsFinal,
			Metadata: row.Metadata,
		})
	}
	return states, nil
}

// --- Transitions ---

func (r *PostgresRepository) InsertTransition(ctx context.Context, db DBTX, definitionID uuid.UUID, t model.Transition) (uuid.UUID, error) {
	q := r.getQuerier(db)

	transitionID, err := q.InsertTransition(ctx, sqlcgen.InsertTransitionParams{
		DefinitionID:  definitionID,
		FromState:     string(t.FromState),
		EventName:     string(t.EventName),
		ToState:       string(t.ToState),
		Priority:      int32(t.Priority),
		EmitEventType: textOrNil(t.EmitEventType),
	})
	if err != nil {
		return uuid.Nil, r.handleError(err, "insert transition")
	}
	return transitionID, nil
}

func (r *PostgresRepository) ListTransitionsByDefinition(ctx context.Context, db DBTX, definitionID uuid.UUID) ([]model.Transition, error) {
	q := r.getQuerier(db)

	rows, err := q.ListTransitionsByDefinition(ctx, definitionID)
	if err != nil {
		return nil, fmt.Errorf("list transitions by definition: %w", err)
	}

	transitions := make([]model.Transition, 0, len(rows))
	for _, row := range rows {
		transitions = append(transitions, model.Transition{
			TransitionID:  row.TransitionID,
			FromState:     model.StateName(row.FromState),
			EventName:     model.EventName(row.EventName),
			ToState:       model.StateName(row.ToState),
			Priority:      int(row.Priority),
			EmitEventType: textValue(row.EmitEventType),
		})
	}
	return transitions, nil
}

// --- Guard bindings ---

func (r *PostgresRepository) InsertGuardBinding(ctx context.Context, db DBTX, transitionID uuid.UUID, gb model.GuardBinding) error {
	q := r.getQuerier(db)

	err := q.InsertGuardBinding(ctx, sqlcgen.InsertGuardBindingParams{
		TransitionID:       transitionID,
		Seq:                int32(gb.Seq),
		ImplementationKind: string(gb.ImplementationKind),
		GuardName:          textOrNil(gb.GuardName),
		Script:             textOrNil(gb.Script),
		Params:             nonEmptyJSON(gb.Params),
	})
	if err != nil {
		return r.handleError(err, "insert guard binding")
	}
	return nil
}

//nolint:lll // Kept together for readability.
func (r *PostgresRepository) ListGuardBindingsByDefinition(ctx context.Context, db DBTX, definitionID uuid.UUID) (map[uuid.UUID][]model.GuardBinding, error) {
	q := r.getQuerier(db)

	rows, err := q.ListGuardBindingsByDefinition(ctx, definitionID)
	if err != nil {
		return nil, fmt.Errorf("list guard bindings by definition: %w", err)
	}

	byTransition := make(map[uuid.UUID][]model.GuardBinding)
	for _, row := range rows {
		byTransition[row.TransitionID] = append(byTransition[row.TransitionID], model.GuardBinding{
			Seq:                int(row.Seq),
			ImplementationKind: model.GuardImplementationKind(row.ImplementationKind),
			GuardName:          textValue(row.GuardName),
			Script:             textValue(row.Script),
			Params:             row.Params,
		})
	}
	return byTransition, nil
}

// --- Action bindings ---

//nolint:lll // Kept together for readability.
func (r *PostgresRepository) InsertActionBinding(ctx context.Context, db DBTX, definitionID uuid.UUID, ab model.ActionBinding) (uuid.UUID, error) {
	q := r.getQuerier(db)

	params := ab.Params
	if ab.ParamsKind == model.ParamsKindStatic {
		params = nonEmptyJSON(params)
	}

	var stateName pgtype.Text
	if ab.HookType != model.HookOnTransition {
		stateName = textOrNil(string(ab.StateName))
	}

	bindingID, err := q.InsertActionBinding(ctx, sqlcgen.InsertActionBindingParams{
		DefinitionID: definitionID,
		HookType:     string(ab.HookType),
		StateName:    stateName,
		TransitionID: ab.TransitionID,
		Seq:          int32(ab.Seq),
		ActionName:   ab.ActionName,
		ParamsKind:   string(ab.ParamsKind),
		Params:       params,
		ParamsScript: textOrNil(ab.ParamsScript),
		Mode:         string(ab.Mode),
		OnError:      string(ab.OnError),
	})
	if err != nil {
		return uuid.Nil, r.handleError(err, "insert action binding")
	}
	return bindingID, nil
}

//nolint:lll // Kept together for readability.
func (r *PostgresRepository) ListActionBindingsByDefinition(ctx context.Context, db DBTX, definitionID uuid.UUID) ([]model.ActionBinding, error) {
	q := r.getQuerier(db)

	rows, err := q.ListActionBindingsByDefinition(ctx, definitionID)
	if err != nil {
		return nil, fmt.Errorf("list action bindings by definition: %w", err)
	}

	actions := make([]model.ActionBinding, 0, len(rows))
	for _, row := range rows {
		actions = append(actions, model.ActionBinding{
			BindingID:    row.BindingID,
			HookType:     model.HookType(row.HookType),
			StateName:    model.StateName(textValue(row.StateName)),
			TransitionID: row.TransitionID,
			Seq:          int(row.Seq),
			ActionName:   row.ActionName,
			ParamsKind:   model.ParamsKind(row.ParamsKind),
			Params:       row.Params,
			ParamsScript: textValue(row.ParamsScript),
			Mode:         model.ActionMode(row.Mode),
			OnError:      model.OnErrorPolicy(row.OnError),
		})
	}
	return actions, nil
}

// --- Definition bundle (composite read for the engine's definition cache) ---

func (r *PostgresRepository) GetDefinitionBundle(ctx context.Context, db DBTX, definitionID uuid.UUID) (model.DefinitionBundle, error) {
	meta, err := r.GetDefinitionByID(ctx, db, definitionID)
	if err != nil {
		return model.DefinitionBundle{}, err
	}

	states, err := r.ListStatesByDefinition(ctx, db, definitionID)
	if err != nil {
		return model.DefinitionBundle{}, err
	}

	transitions, err := r.ListTransitionsByDefinition(ctx, db, definitionID)
	if err != nil {
		return model.DefinitionBundle{}, err
	}

	guardsByTransition, err := r.ListGuardBindingsByDefinition(ctx, db, definitionID)
	if err != nil {
		return model.DefinitionBundle{}, err
	}
	for i := range transitions {
		transitions[i].Guards = guardsByTransition[transitions[i].TransitionID]
	}

	actions, err := r.ListActionBindingsByDefinition(ctx, db, definitionID)
	if err != nil {
		return model.DefinitionBundle{}, err
	}

	return model.DefinitionBundle{
		Meta:        meta,
		States:      states,
		Transitions: transitions,
		Actions:     actions,
	}, nil
}
