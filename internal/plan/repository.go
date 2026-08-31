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

var ErrPlanNotFound = errors.New("plan not found")
var ErrPlanDurationNotFound = errors.New("plan duration not found")

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
	GetByID(ctx context.Context, id int64) (Plan, error)

	LatestVersion(
		ctx context.Context,
		code string,
	) (int, error)

	ListAll(
		ctx context.Context,
	) ([]Plan, error)

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

	UpdateDurationTariff(
		ctx context.Context,
		tx pgx.Tx,
		durationID int64,
		tariffID int64,
		isActive bool,
	) (PlanDuration, error)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (Plan, error) {
	if id <= 0 {
		return Plan{}, errors.New("plan id must be greater than zero")
	}
	row, err := sqlcgen.New(r.pool).GetPlanByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrPlanNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("get plan by id: %w", err)
	}
	return toPlanModel(row), nil
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
		return Plan{}, fmt.Errorf("create plan: %w", err)
	}

	return toPlanModel(row), nil
}

func (r *PostgresRepository) CreateDuration(
	ctx context.Context,
	tx pgx.Tx,
	duration PlanDuration,
) (PlanDuration, error) {
	if err := duration.Validate(); err != nil {
		return PlanDuration{}, fmt.Errorf(
			"validate plan duration: %w",
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

	return toPlanDurationModel(row), nil
}

func (r *PostgresRepository) GetByCodeAndVersion(
	ctx context.Context,
	code string,
	version int,
) (Plan, error) {
	q := sqlcgen.New(r.pool)

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

func (r *PostgresRepository) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	q := sqlcgen.New(r.pool)

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
	q := sqlcgen.New(r.pool)

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

func (r *PostgresRepository) ListAll(
	ctx context.Context,
) ([]Plan, error) {
	q := sqlcgen.New(r.pool)

	rows, err := q.ListPlans(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"list plans: %w",
			err,
		)
	}

	plans := make([]Plan, 0, len(rows))
	for _, row := range rows {
		plans = append(plans, toPlanModel(row))
	}

	return plans, nil
}

func (r *PostgresRepository) GetDuration(
	ctx context.Context,
	id int64,
) (PlanDuration, error) {
	q := sqlcgen.New(r.pool)

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
	q := sqlcgen.New(r.pool)

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
	q := sqlcgen.New(r.pool)

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
	tx pgx.Tx,
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

	q := sqlcgen.New(tx)

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
