package plan

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

var (
	ErrPlanNotFound = errors.New("plan not found")
)

type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, plan Plan) (Plan, error)
	CreateDuration(ctx context.Context, tx pgx.Tx, duration PlanDuration) (PlanDuration, error)
	GetByCodeAndVersion(ctx context.Context, code string, version int) (Plan, error)
	GetDurations(ctx context.Context, planID int64) ([]PlanDuration, error)
	LatestVersion(ctx context.Context, code string) (int, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	plan Plan,
) (Plan, error) {
	q := sqlcgen.New(tx)

	row, err := q.CreatePlan(ctx, sqlcgen.CreatePlanParams{
		PlanCode:              plan.PlanCode,
		Version:               int32(plan.Version),
		EffectiveFrom:         plan.EffectiveFrom,
		EffectiveUntil:        plan.EffectiveUntil,
		LegacyPricePolicyCode: string(plan.LegacyPricePolicyCode),
		MigrationPath:         plan.MigrationPath,
		Metadata:              plan.Metadata,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("create plan: %w", err)
	}

	return Plan{
		ID:                    row.PlanID,
		PlanCode:              row.PlanCode,
		Version:               int(row.Version),
		EffectiveFrom:         row.EffectiveFrom,
		EffectiveUntil:        row.EffectiveUntil,
		LegacyPricePolicyCode: LegacyPricePolicy(row.LegacyPricePolicyCode),
		MigrationPath:         row.MigrationPath,
		Metadata:              row.Metadata,
		CreatedAt:             row.CreatedAt,
	}, nil
}

func (r *PostgresRepository) CreateDuration(
	ctx context.Context,
	tx pgx.Tx,
	duration PlanDuration,
) (PlanDuration, error) {
	q := sqlcgen.New(tx)

	row, err := q.CreatePlanDuration(ctx, sqlcgen.CreatePlanDurationParams{
		PlanID:   duration.PlanID,
		TariffID: duration.TariffID,
		Duration: duration.Duration,
		IsActive: duration.IsActive,
	})
	if err != nil {
		return PlanDuration{}, fmt.Errorf("create plan duration: %w", err)
	}

	return PlanDuration{
		ID:        row.PlanDurationID,
		PlanID:    row.PlanID,
		TariffID:  row.TariffID,
		Duration:  row.Duration,
		IsActive:  row.IsActive,
		CreatedAt: row.CreatedAt,
	}, nil
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

	durations, err := r.GetDurations(ctx, row.PlanID)
	if err != nil {
		return Plan{}, fmt.Errorf("get plan durations: %w", err)
	}

	return Plan{
		ID:                    row.PlanID,
		PlanCode:              row.PlanCode,
		Version:               int(row.Version),
		EffectiveFrom:         row.EffectiveFrom,
		EffectiveUntil:        row.EffectiveUntil,
		LegacyPricePolicyCode: LegacyPricePolicy(row.LegacyPricePolicyCode),
		MigrationPath:         row.MigrationPath,
		Metadata:              row.Metadata,
		CreatedAt:             row.CreatedAt,
		Durations:             durations,
	}, nil
}

func (r *PostgresRepository) GetDurations(
	ctx context.Context,
	planID int64,
) ([]PlanDuration, error) {
	q := sqlcgen.New(r.pool)

	rows, err := q.GetPlanDurations(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("get plan durations: %w", err)
	}

	durations := make([]PlanDuration, 0, len(rows))

	for _, row := range rows {
		durations = append(durations, PlanDuration{
			ID:        row.PlanDurationID,
			PlanID:    row.PlanID,
			TariffID:  row.TariffID,
			Duration:  row.Duration,
			IsActive:  row.IsActive,
			CreatedAt: row.CreatedAt,
		})
	}

	return durations, nil
}

func (r *PostgresRepository) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	q := sqlcgen.New(r.pool)

	version, err := q.GetLatestPlanVersion(ctx, code)
	if err != nil {
		return 0, fmt.Errorf("get latest plan version: %w", err)
	}

	return int(version), nil
}
