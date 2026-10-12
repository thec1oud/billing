package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	planmodel "github.com/thec1oud/billing/internal/plan/model"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

type Plan = planmodel.Plan
type PlanDuration = planmodel.PlanDuration
type LegacyPricePolicy = planmodel.LegacyPricePolicy

var (
	ErrPlanNotFound         = errors.New("plan not found")
	ErrPlanDurationNotFound = errors.New("plan duration not found")
)

type DBTX interface {
	sqlcgen.DBTX
}

type Repository interface {
	Pool() *pgxpool.Pool

	Create(
		ctx context.Context,
		db DBTX,
		plan Plan,
	) (Plan, error)

	CreateDuration(
		ctx context.Context,
		db DBTX,
		duration PlanDuration,
	) (PlanDuration, error)

	GetByCodeAndVersion(
		ctx context.Context,
		code string,
		version int,
	) (Plan, error)

	GetByID(
		ctx context.Context,
		id int64,
	) (Plan, error)

	LatestVersion(
		ctx context.Context,
		code string,
	) (int, error)

	ListVersions(
		ctx context.Context,
		code string,
	) ([]Plan, error)

	ListActivePlans(
		ctx context.Context,
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

	UpdateDurationTariff(
		ctx context.Context,
		db DBTX,
		durationID int64,
		tariffID int64,
		isActive bool,
	) (PlanDuration, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *PostgresRepository) getQuerier(db DBTX) *sqlcgen.Queries {
	if db != nil {
		return sqlcgen.New(db)
	}

	return sqlcgen.New(r.pool)
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Create(
	ctx context.Context,
	db DBTX,
	plan Plan,
) (Plan, error) {
	if err := plan.Validate(); err != nil {
		return Plan{}, fmt.Errorf("validate plan: %w", err)
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

	q := r.getQuerier(db)

	row, err := q.CreatePlan(
		ctx,
		sqlcgen.CreatePlanParams{
			PlanCode:              plan.PlanCode,
			EffectiveFrom:         plan.EffectiveFrom,
			EffectiveUntil:        effectiveUntil,
			LegacyPricePolicyCode: string(plan.LegacyPricePolicyCode),
			MigrationPath:         migrationPath,
			Metadata:              metadata,
		},
	)
	if err != nil {
		return Plan{}, fmt.Errorf("create plan: %w", err)
	}

	return toPlanModel(row), nil
}

func (r *PostgresRepository) CreateDuration(
	ctx context.Context,
	db DBTX,
	duration PlanDuration,
) (PlanDuration, error) {
	if err := duration.Validate(); err != nil {
		return PlanDuration{}, fmt.Errorf(
			"validate plan duration: %w",
			err,
		)
	}

	q := r.getQuerier(db)

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

	return toPlanDurationModel(row), nil
}

func (r *PostgresRepository) GetByCodeAndVersion(
	ctx context.Context,
	code string,
	version int,
) (Plan, error) {
	q := r.getQuerier(nil)

	row, err := q.GetPlanByCodeAndVersion(
		ctx,
		sqlcgen.GetPlanByCodeAndVersionParams{
			PlanCode: code,
			Version:  int32(version),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrPlanNotFound
	}

	if err != nil {
		return Plan{}, fmt.Errorf("get plan: %w", err)
	}

	return toPlanModel(row), nil
}

func (r *PostgresRepository) GetByID(
	ctx context.Context,
	id int64,
) (Plan, error) {
	if id <= 0 {
		return Plan{}, errors.New(
			"plan id must be greater than zero",
		)
	}

	q := r.getQuerier(nil)

	row, err := q.GetPlanByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrPlanNotFound
	}

	if err != nil {
		return Plan{}, fmt.Errorf(
			"get plan by id: %w",
			err,
		)
	}

	return toPlanModel(row), nil
}

func (r *PostgresRepository) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	q := r.getQuerier(nil)

	version, err := q.GetLatestPlanVersion(ctx, code)
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
	q := r.getQuerier(nil)

	rows, err := q.ListPlanVersions(ctx, code)
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
		plans = append(plans, toPlanModel(row))
	}

	return plans, nil
}

func (r *PostgresRepository) ListActivePlans(
	ctx context.Context,
) ([]Plan, error) {
	q := r.getQuerier(nil)

	rows, err := q.ListActivePlans(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"list active plans: %w",
			err,
		)
	}

	plans := make([]Plan, 0, len(rows))

	for _, row := range rows {
		plans = append(
			plans,
			toPlanModel(sqlcgen.Plan{
				PlanID:                row.PlanID,
				PlanCode:              row.PlanCode,
				Version:               row.Version,
				EffectiveFrom:         row.EffectiveFrom,
				EffectiveUntil:        row.EffectiveUntil,
				LegacyPricePolicyCode: row.LegacyPricePolicyCode,
				MigrationPath:         row.MigrationPath,
				Metadata:              row.Metadata,
				CreatedAt:             row.CreatedAt,
			}),
		)
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

	q := r.getQuerier(nil)

	row, err := q.GetPlanDuration(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanDuration{}, ErrPlanDurationNotFound
	}

	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"get plan duration: %w",
			err,
		)
	}

	return toPlanDurationModel(row), nil
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
			"duration must be greater than zero",
		)
	}

	q := r.getQuerier(nil)

	row, err := q.GetPlanDurationByPlanAndDuration(
		ctx,
		sqlcgen.GetPlanDurationByPlanAndDurationParams{
			PlanID:   planID,
			Duration: int64(duration),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanDuration{}, ErrPlanDurationNotFound
	}

	if err != nil {
		return PlanDuration{}, fmt.Errorf(
		"get plan duration by plan and duration: %w",
		err,
	)
	}

	return toPlanDurationModel(row), nil
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

	q := r.getQuerier(nil)

	rows, err := q.ListPlanDurations(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf(
			"list plan durations: %w",
			err,
		)
	}

	durations := make([]PlanDuration, 0, len(rows))

	for _, row := range rows {
		durations = append(
			durations,
			toPlanDurationModel(row),
		)
	}

	return durations, nil
}

func (r *PostgresRepository) UpdateDurationTariff(
	ctx context.Context,
	db DBTX,
	durationID int64,
	tariffID int64,
	isActive bool,
) (PlanDuration, error) {
	if durationID <= 0 {
		return PlanDuration{}, errors.New(
			"plan duration id must be greater than zero",
		)
	}

	if tariffID <= 0 {
		return PlanDuration{}, errors.New(
			"tariff id must be greater than zero",
		)
	}

	q := r.getQuerier(db)

	row, err := q.UpdatePlanDurationTariff(
		ctx,
		sqlcgen.UpdatePlanDurationTariffParams{
			PlanDurationID: durationID,
			TariffID:       tariffID,
			IsActive:       isActive,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlanDuration{}, ErrPlanDurationNotFound
	}

	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"update plan duration tariff: %w",
			err,
		)
	}

	return toPlanDurationModel(row), nil
}

func toPlanModel(row sqlcgen.Plan) Plan {
	migrationPath := row.MigrationPath
	if len(migrationPath) == 0 {
		migrationPath = json.RawMessage(`{}`)
	}

	metadata := row.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	var effectiveUntil *time.Time

	if row.EffectiveUntil.Valid {
		value := row.EffectiveUntil.Time
		effectiveUntil = &value
	}

	return Plan{
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
}

func toPlanDurationModel(row sqlcgen.PlanDuration) PlanDuration {
	return PlanDuration{
		ID:        row.PlanDurationID,
		PlanID:    row.PlanID,
		TariffID:  row.TariffID,
		Duration:  time.Duration(row.Duration),
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt,
	}
}