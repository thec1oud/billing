package loader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/statemachine/registry"
	"github.com/thec1oud/billing/internal/shared/statemachine/repository"
)

// Publish validates spec and, if valid, compiles it into a new immutable
// sm_definitions version in one transaction: (optionally) deactivate the previous
// active version for this MachineType, then insert the definition, states,
// transitions, and guard/action bindings. Returns the definition's ID, reusing
// the existing ID when the same version and content have already been published.
func Publish(
	ctx context.Context,
	pool *pgxpool.Pool,
	repo *repository.PostgresRepository,
	reg *registry.Registry,
	spec DefinitionSpec,
) (uuid.UUID, error) {
	if err := validate(spec, reg); err != nil {
		return uuid.Nil, err
	}
	hash, err := definitionHash(spec)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash definition: %w", err)
	}
	if id, exists, err := existingDefinition(ctx, repo, spec, hash); err != nil {
		return uuid.Nil, err
	} else if exists {
		return id, nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if spec.Activate {
		if err := repo.DeactivateDefinitionsForMachineType(ctx, tx, spec.MachineType); err != nil {
			return uuid.Nil, fmt.Errorf("deactivate previous active definitions: %w", err)
		}
	}

	def, err := repo.InsertDefinitionWithContentHash(ctx, tx, spec.MachineType, spec.Version, hash, spec.InitialState, spec.Activate)
	if err != nil {
		if isDefinitionPublishUniqueViolation(err) {
			if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				return uuid.Nil, fmt.Errorf("rollback conflicting definition publish: %w", rollbackErr)
			}
			if id, exists, existingErr := existingDefinition(ctx, repo, spec, hash); existingErr != nil {
				return uuid.Nil, existingErr
			} else if exists {
				return id, nil
			}
		}
		return uuid.Nil, fmt.Errorf("insert definition: %w", err)
	}

	for _, s := range spec.States {
		state := model.State{Name: s.Name, IsFinal: s.IsFinal, Metadata: s.Metadata}
		if err := repo.InsertState(ctx, tx, def.DefinitionID, state); err != nil {
			return uuid.Nil, fmt.Errorf("insert state %q: %w", s.Name, err)
		}

		for _, ab := range s.OnEnter {
			binding := ab.toModel(model.HookOnEnter, s.Name, nil)
			if _, err := repo.InsertActionBinding(ctx, tx, def.DefinitionID, binding); err != nil {
				return uuid.Nil, fmt.Errorf("insert ON_ENTER action %q for state %q: %w", ab.ActionName, s.Name, err)
			}
		}
		for _, ab := range s.OnExit {
			binding := ab.toModel(model.HookOnExit, s.Name, nil)
			if _, err := repo.InsertActionBinding(ctx, tx, def.DefinitionID, binding); err != nil {
				return uuid.Nil, fmt.Errorf("insert ON_EXIT action %q for state %q: %w", ab.ActionName, s.Name, err)
			}
		}
	}

	for _, t := range spec.Transitions {
		transitionID, err := repo.InsertTransition(ctx, tx, def.DefinitionID, model.Transition{
			FromState:     t.FromState,
			EventName:     t.EventName,
			ToState:       t.ToState,
			Priority:      t.Priority,
			EmitEventType: t.EmitEventType,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("insert transition %s/%s: %w", t.FromState, t.EventName, err)
		}

		for _, gb := range t.Guards {
			guard := model.GuardBinding{
				Seq:                gb.Seq,
				ImplementationKind: gb.ImplementationKind,
				GuardName:          gb.GuardName,
				Script:             gb.Script,
				Params:             gb.Params,
			}
			if err := repo.InsertGuardBinding(ctx, tx, transitionID, guard); err != nil {
				return uuid.Nil, fmt.Errorf("insert guard binding on %s/%s: %w", t.FromState, t.EventName, err)
			}
		}

		for _, ab := range t.Actions {
			binding := ab.toModel(model.HookOnTransition, "", &transitionID)
			if _, err := repo.InsertActionBinding(ctx, tx, def.DefinitionID, binding); err != nil {
				return uuid.Nil, fmt.Errorf("insert ON_TRANSITION action %q on %s/%s: %w", ab.ActionName, t.FromState, t.EventName, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit definition publish: %w", err)
	}

	return def.DefinitionID, nil
}

func existingDefinition(
	ctx context.Context,
	repo *repository.PostgresRepository,
	spec DefinitionSpec,
	hash string,
) (uuid.UUID, bool, error) {
	id, existingHash, err := repo.GetDefinitionByMachineTypeVersion(ctx, nil, spec.MachineType, spec.Version)
	if errors.Is(err, model.ErrDefinitionNotFound) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("check existing definition: %w", err)
	}
	if existingHash == "" {
		return uuid.Nil, true, fmt.Errorf(
			"%w: definition %s version %d exists without a content hash; bump its version to publish verifiable content",
			model.ErrDefinitionVersionConflict,
			spec.MachineType,
			spec.Version,
		)
	}
	if existingHash != hash {
		return uuid.Nil, true, fmt.Errorf(
			"%w: definition %s version %d has changed content; bump the version before publishing",
			model.ErrDefinitionVersionConflict,
			spec.MachineType,
			spec.Version,
		)
	}
	return id, true, nil
}

func isDefinitionPublishUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}
	switch pgErr.ConstraintName {
	case "sm_definitions_machine_type_version_key", "idx_sm_definitions_active_per_type":
		return true
	default:
		return false
	}
}

func definitionHash(spec DefinitionSpec) (string, error) {
	spec.Activate = false
	content, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}
