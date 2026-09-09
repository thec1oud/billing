package dev

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	planmodel "github.com/thec1oud/billing/internal/plan/model"
	planservice "github.com/thec1oud/billing/internal/plan/service"
	"github.com/thec1oud/billing/internal/shared/money"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
	tariffservice "github.com/thec1oud/billing/internal/tariff/service"
)

// Seed runs deterministic data seeding for development environments.
func Seed(
	ctx context.Context,
	pool *pgxpool.Pool,
	planSvc *planservice.Service,
	tariffSvc *tariffservice.Service,
	log *slog.Logger,
) error {
	log.Info("Running dev environment seeder...")

	// 1. Seed Fake Payment Provider
	_, err := pool.Exec(ctx, `
		INSERT INTO payment_provider (payment_provider_code)
		VALUES ('fake')
		ON CONFLICT DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("failed to seed fake payment provider: %w", err)
	}

	// 2. Seed Default Tariff
	defaultTariffCode := "dev_tariff_per_unit"
	var count int
	var tariffID int64
	err = pool.QueryRow(ctx, "SELECT count(*) FROM tariffs WHERE tariff_code = $1", defaultTariffCode).Scan(&count)
	if err == nil && count == 0 {
		tx, txErr := pool.Begin(ctx)
		if txErr != nil {
			return txErr
		}
		createdTariff, err := tariffSvc.CreateTariff(
			ctx,
			tx,
			defaultTariffCode,
			"Standard Per Unit 1000",
			"A dev mock tariff",
			tariffmodel.TariffTypePerUnit,
			money.Money{AmountMinor: 1000, Currency: "ETB"},
			nil, // tiers
			nil, // metadata
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			log.Warn("failed to seed dev tariff", "err", err)
		} else {
			err = tx.Commit(ctx)
			if err != nil {
				return fmt.Errorf("commit dev tariff: %w", err)
			}
			tariffID = createdTariff.ID
			log.Info("Seeded dev tariff", "tariff_code", defaultTariffCode)
		}
	} else {
		_ = pool.QueryRow(ctx, "SELECT tariff_id FROM tariffs WHERE tariff_code = $1", defaultTariffCode).Scan(&tariffID)
	}

	// 3. Seed Default Plan
	defaultPlanCode := "dev_premium_plan"
	err = pool.QueryRow(ctx, "SELECT count(*) FROM plans WHERE plan_code = $1", defaultPlanCode).Scan(&count)
	if err == nil && count == 0 && tariffID > 0 {
		tx, txErr := pool.Begin(ctx)
		if txErr != nil {
			return txErr
		}
		_, err = planSvc.CreatePlan(ctx, tx, planmodel.Plan{
			PlanCode:              defaultPlanCode,
			LegacyPricePolicyCode: planmodel.LegacyPolicyKeepForever,
			EffectiveFrom:         time.Now().UTC(),
			Durations: []planmodel.PlanDuration{
				{
					TariffID: tariffID,
					Duration: 30 * 24 * time.Hour,
					IsActive: true,
				},
			},
		})
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err != nil {
			log.Warn("failed to seed dev plan", "err", err)
		} else {
			log.Info("Seeded dev plan", "plan_code", defaultPlanCode)
		}
	}

	log.Info("Dev environment seeding completed.")
	return nil
}
