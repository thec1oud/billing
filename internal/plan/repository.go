package plan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

var (
	ErrPlanNotFound         = errors.New("plan not found")
	ErrPlanDurationNotFound = errors.New("plan duration not found")
)

type Repository interface {
	Create(
		ctx context.Context,
		tx pgx.Tx,
		plan Plan,
	) (Plan, error)

	CreateDuration(
		ctx context.Context,
		tx pgx.Tx,
		duration PlanDuration,
	) (PlanDuration, error)

	GetByCodeAndVersion(
		ctx context.Context,
		code string,
		version int,
	) (Plan, error)

	GetByIDAndVersion(
		ctx context.Context,
		id int64,
		version int,
	) (Plan, error)

	LatestVersion(
		ctx context.Context,
		code string,
	) (int, error)

	ListVersions(
		ctx context.Context,
		code string,
	) ([]Plan, error)

	GetDuration(
		ctx context.Context,
		id int64,
	) (PlanDuration, error)

	GetDurationByPlanAndDuration(
		ctx context.Context,
		planID int64,
		duration time.Duration,
	) (PlanDuration, error)

	ListDurations(
		ctx context.Context,
		planID int64,
	) ([]PlanDuration, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	plan Plan,
) (Plan, error) {
	if err := plan.Validate(); err != nil {
		return Plan{}, fmt.Errorf(
			"invalid plan: %w",
			err,
		)
	}

	migrationPath := plan.MigrationPath
	if len(migrationPath) == 0 {
		migrationPath = json.RawMessage(`{}`)
	}

	metadata := plan.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	var effectiveUntil pgtype.Timestamptz

	if plan.EffectiveUntil != nil {
		effectiveUntil = pgtype.Timestamptz{
			Time:  *plan.EffectiveUntil,
			Valid: true,
		}
	}

	q := sqlcgen.New(tx)

	row, err := q.CreatePlan(
		ctx,
		sqlcgen.CreatePlanParams{
			PlanCode:              plan.PlanCode,
			Version:               int32(plan.Version),
			EffectiveFrom:         plan.EffectiveFrom,
			EffectiveUntil:        effectiveUntil,
			LegacyPricePolicyCode: string(plan.LegacyPricePolicyCode),
			MigrationPath:         migrationPath,
			Metadata:              metadata,
		},
	)
	if err != nil {
		return Plan{}, fmt.Errorf(
			"create plan: %w",
			err,
		)
	}

	return toPlanModel(row)
}

func (r *PostgresRepository) CreateDuration(
	ctx context.Context,
	tx pgx.Tx,
	duration PlanDuration,
) (PlanDuration, error) {
	if err := duration.Validate(); err != nil {
		return PlanDuration{}, fmt.Errorf(
			"invalid plan duration: %w",
			err,
		)
	}

	q := sqlcgen.New(tx)

	row, err := q.CreatePlanDuration(
		ctx,
		sqlcgen.CreatePlanDurationParams{
			PlanID:   duration.PlanID,
			TariffID: duration.TariffID,
			Duration: int64(duration.Duration),
			IsActive: duration.IsActive,
		},
	)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"create plan duration: %w",
			err,
		)
	}

	return toPlanDurationModel(row)
}

func (r *PostgresRepository) GetByCodeAndVersion(
	ctx context.Context,
	code string,
	version int,
) (Plan, error) {
	if code == "" {
		return Plan{}, errors.New("plan code is required")
	}

	if version < 1 {
		return Plan{}, errors.New(
			"plan version must be greater than zero",
		)
	}

	q := sqlcgen.New(r.pool)

	row, err := q.GetPlanByCodeAndVersion(
		ctx,
		sqlcgen.GetPlanByCodeAndVersionParams{
			PlanCode: code,
			Version:  int32(version),
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Plan{}, ErrPlanNotFound
		}

		return Plan{}, fmt.Errorf(
			"get plan by code and version: %w",
			err,
		)
	}

	return toPlanModel(row)
}

func (r *PostgresRepository) GetByIDAndVersion(
	ctx context.Context,
	id int64,
	version int,
) (Plan, error) {
	if id <= 0 {
		return Plan{}, errors.New(
			"plan id must be greater than zero",
		)
	}

	if version < 1 {
		return Plan{}, errors.New(
			"plan version must be greater than zero",
		)
	}

	q := sqlcgen.New(r.pool)

	row, err := q.GetPlanByIDAndVersion(
		ctx,
		sqlcgen.GetPlanByIDAndVersionParams{
			PlanID:  id,
			Version: int32(version),
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Plan{}, ErrPlanNotFound
		}

		return Plan{}, fmt.Errorf(
			"get plan by id and version: %w",
			err,
		)
	}

	return toPlanModel(row)
}

func (r *PostgresRepository) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	if code == "" {
		return 0, errors.New("plan code is required")
	}

	q := sqlcgen.New(r.pool)

	version, err := q.GetLatestPlanVersion(
		ctx,
		code,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"get latest plan version: %w",
			err,
		)
	}

	return int(version), nil
}

func (r *PostgresRepository) ListVersions(
	ctx context.Context,
	code string,
) ([]Plan, error) {
	if code == "" {
		return nil, errors.New("plan code is required")
	}

	q := sqlcgen.New(r.pool)

	rows, err := q.ListPlanVersions(
		ctx,
		code,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list plan versions: %w",
			err,
		)
	}

	if len(rows) == 0 {
		return nil, ErrPlanNotFound
	}

	plans := make([]Plan, 0, len(rows))

	for _, row := range rows {
		plan, err := toPlanModel(row)
		if err != nil {
			return nil, fmt.Errorf(
				"map plan version: %w",
				err,
			)
		}

		plans = append(plans, plan)
	}

	return plans, nil
}

func (r *PostgresRepository) GetDuration(
	ctx context.Context,
	id int64,
) (PlanDuration, error) {
	if id <= 0 {
		return PlanDuration{}, errors.New(
			"plan duration id must be greater than zero",
		)
	}

	q := sqlcgen.New(r.pool)

	row, err := q.GetPlanDuration(
		ctx,
		id,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanDuration{}, ErrPlanDurationNotFound
		}

		return PlanDuration{}, fmt.Errorf(
			"get plan duration: %w",
			err,
		)
	}

	return toPlanDurationModel(row)
}

func (r *PostgresRepository) GetDurationByPlanAndDuration(
	ctx context.Context,
	planID int64,
	duration time.Duration,
) (PlanDuration, error) {
	if planID <= 0 {
		return PlanDuration{}, errors.New(
			"plan id must be greater than zero",
		)
	}

	if duration <= 0 {
		return PlanDuration{}, errors.New(
			"plan duration must be greater than zero",
		)
	}

	q := sqlcgen.New(r.pool)

	row, err := q.GetPlanDurationByPlanAndDuration(
		ctx,
		sqlcgen.GetPlanDurationByPlanAndDurationParams{
			PlanID:   planID,
			Duration: int64(duration),
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PlanDuration{}, ErrPlanDurationNotFound
		}

		return PlanDuration{}, fmt.Errorf(
			"get plan duration by plan and duration: %w",
			err,
		)
	}

	return toPlanDurationModel(row)
}

func (r *PostgresRepository) ListDurations(
	ctx context.Context,
	planID int64,
) ([]PlanDuration, error) {
	if planID <= 0 {
		return nil, errors.New(
			"plan id must be greater than zero",
		)
	}

	q := sqlcgen.New(r.pool)

	rows, err := q.ListPlanDurations(
		ctx,
		planID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list plan durations: %w",
			err,
		)
	}

	durations := make([]PlanDuration, 0, len(rows))

	for _, row := range rows {
		duration, err := toPlanDurationModel(row)
		if err != nil {
			return nil, fmt.Errorf(
				"map plan duration: %w",
				err,
			)
		}

		durations = append(durations, duration)
	}

	return durations, nil
}

func toPlanModel(row sqlcgen.Plan) (Plan, error) {
	var effectiveUntil *time.Time

	if row.EffectiveUntil.Valid {
		value := row.EffectiveUntil.Time
		effectiveUntil = &value
	}

	migrationPath := row.MigrationPath
	if len(migrationPath) == 0 {
		migrationPath = json.RawMessage(`{}`)
	}

	metadata := row.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	plan := Plan{
		ID:                    row.PlanID,
		PlanCode:              row.PlanCode,
		Version:               int(row.Version),
		EffectiveFrom:         row.EffectiveFrom,
		EffectiveUntil:        effectiveUntil,
		LegacyPricePolicyCode: LegacyPricePolicy(row.LegacyPricePolicyCode),
		MigrationPath:         migrationPath,
		Metadata:              metadata,
		CreatedAt:             row.CreatedAt,
	}

	if err := plan.Validate(); err != nil {
		return Plan{}, fmt.Errorf(
			"validate persisted plan: %w",
			err,
		)
	}

	return plan, nil
}

func toPlanDurationModel(row sqlcgen.PlanDuration) (PlanDuration, error) {
	duration := PlanDuration{
		ID:        row.PlanDurationID,
		PlanID:    row.PlanID,
		TariffID:  row.TariffID,
		Duration:  time.Duration(row.Duration),
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt,
	}

	if err := duration.Validate(); err != nil {
		return PlanDuration{}, fmt.Errorf(
			"validate persisted plan duration: %w",
			err,
		)
	}

	return duration, nil
}
