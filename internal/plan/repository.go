package plan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type Repository interface {
	Save(ctx context.Context, plan Plan) (Plan, error)
	GetByCodeAndVersion(ctx context.Context, code string, version int) (Plan, error)
	LatestVersion(ctx context.Context, code string) (int, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Save(ctx context.Context, plan Plan) (Plan, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Plan{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert into tariffs table
	tariffQuery := `
		INSERT INTO tariffs (
			tariff_code, version, name, tariff_type_code, amount, billing_interval_code, is_active, metadata
		) VALUES ($1, $2, $3, 'FLAT_FEE', $4, $5, TRUE, '{}'::jsonb)
		RETURNING tariff_id, created_at;
	`
	tariffCode := fmt.Sprintf("TRF_%s_V%d", plan.PlanCode, plan.Version)
	tariffName := fmt.Sprintf("Tariff for plan %s v%d", plan.PlanCode, plan.Version)

	var tariffID int64
	var tariffCreatedAt time.Time

	err = tx.QueryRowContext(
		ctx,
		tariffQuery,
		tariffCode,
		plan.Version,
		tariffName,
		plan.FlatFeeAmount.AmountMinor,
		plan.Interval,
	).Scan(&tariffID, &tariffCreatedAt)
	if err != nil {
		return Plan{}, fmt.Errorf("failed to insert tariff: %w", err)
	}

	// 2. Insert into plans table
	planQuery := `
		INSERT INTO plans (
			plan_code, version, tariff_id, effective_from, effective_until, 
			legacy_price_policy_code, migration_path, metadata
		) VALUES (
			$1, $2, $3, $4, $5, 
			COALESCE(NULLIF($6, ''), 'KEEP_FOREVER'), 
			COALESCE($7, '{}'::jsonb), 
			COALESCE($8, '{}'::jsonb)
		)
		RETURNING plan_id, created_at;
	`
	effectiveFrom := plan.EffectiveFrom
	if effectiveFrom.IsZero() {
		effectiveFrom = time.Now().UTC()
	}

	var planID int64
	var planCreatedAt time.Time

	err = tx.QueryRowContext(
		ctx,
		planQuery,
		plan.PlanCode,
		plan.Version,
		tariffID,
		effectiveFrom,
		plan.EffectiveUntil,
		string(plan.LegacyPricePolicyCode),
		plan.MigrationPath,
		plan.Metadata,
	).Scan(&planID, &planCreatedAt)
	if err != nil {
		return Plan{}, fmt.Errorf("failed to insert plan: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Plan{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	plan.ID = planID
	plan.TariffID = tariffID
	plan.EffectiveFrom = effectiveFrom
	plan.CreatedAt = planCreatedAt

	return plan, nil
}

func (r *postgresRepository) GetByCodeAndVersion(ctx context.Context, code string, version int) (Plan, error) {
	query := `
		SELECT 
			p.plan_id, p.plan_code, p.version, p.tariff_id, p.effective_from, p.effective_until,
			p.legacy_price_policy_code, p.migration_path, p.metadata, p.created_at,
			t.amount, t.billing_interval_code
		FROM plans p
		JOIN tariffs t ON p.tariff_id = t.tariff_id
		WHERE p.plan_code = $1 AND p.version = $2;
	`

	var p Plan
	var rawAmount int64
	var interval string

	err := r.db.QueryRowContext(ctx, query, code, version).Scan(
		&p.ID,
		&p.PlanCode,
		&p.Version,
		&p.TariffID,
		&p.EffectiveFrom,
		&p.EffectiveUntil,
		&p.LegacyPricePolicyCode,
		&p.MigrationPath,
		&p.Metadata,
		&p.CreatedAt,
		&rawAmount,
		&interval,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Plan{}, fmt.Errorf("plan %s version %d not found", code, version)
		}
		return Plan{}, fmt.Errorf("failed to query plan: %w", err)
	}

	p.Interval = BillingInterval(interval)
	m, _ := money.New(rawAmount, string(p.FlatFeeAmount.Currency))
	p.FlatFeeAmount = m

	return p, nil
}

func (r *postgresRepository) LatestVersion(ctx context.Context, code string) (int, error) {
	query := `
		SELECT COALESCE(MAX(version), 0)
		FROM plans
		WHERE plan_code = $1;
	`

	var latest int
	err := r.db.QueryRowContext(ctx, query, code).Scan(&latest)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest version for plan %s: %w", code, err)
	}

	return latest, nil
}
