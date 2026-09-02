package plan

import (
	"context"
	"errors"
	"fmt"
	"time"
	"github.com/jackc/pgx/v5/pgxpool"
	purchasableitem "github.com/thec1oud/billing/internal/purchasable_item"
)

type Service struct {
	db         *pgxpool.Pool
	repository Repository
	itemSvc    *purchasableitem.Service
}

func NewService(db *pgxpool.Pool, repository Repository, itemSvc *purchasableitem.Service) *Service {
	return &Service{
		db:         db,
		repository: repository,
		itemSvc:    itemSvc,
	}
}

func (s *Service) CreatePlan(
	ctx context.Context,
	plan Plan,
) (Plan, error) {
	if plan.PlanCode == "" {
		return Plan{}, errors.New("plan code is required")
	}

	if !plan.LegacyPricePolicyCode.Valid() {
		return Plan{}, fmt.Errorf(
			"unsupported legacy price policy %q",
			plan.LegacyPricePolicyCode,
		)
	}

	latest, err := s.repository.LatestVersion(
		ctx,
		plan.PlanCode,
	)
	if err != nil {
		return Plan{}, fmt.Errorf(
			"get latest plan version: %w",
			err,
		)
	}

	plan.Version = latest + 1

	if plan.EffectiveFrom.IsZero() {
		plan.EffectiveFrom = time.Now().UTC()
	}

	if err := plan.Validate(); err != nil {
		return Plan{}, fmt.Errorf(
			"validate plan: %w",
			err,
		)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := s.repository.Create(
		ctx,
		tx,
		plan,
	)
	if err != nil {
		return Plan{}, fmt.Errorf(
			"create plan: %w",
			err,
		)
	}

	_, err = s.itemSvc.Create(ctx, tx, purchasableitem.PurchasableItem{
		ItemCode:     fmt.Sprintf("%s_v%d", plan.PlanCode, created.Version),
		ItemTypeCode: purchasableitem.ItemTypePlan,
		Name:         fmt.Sprintf("%s Plan (v%d)", plan.PlanCode, created.Version),
		PlanID:       &created.ID,
		IsActive:     true,
	})
	if err != nil {
		return Plan{}, fmt.Errorf("create purchasable item for plan: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Plan{}, fmt.Errorf("commit transaction: %w", err)
	}

	return created, nil
}

func (s *Service) CreatePlanDuration(
	ctx context.Context,
	duration PlanDuration,
) (PlanDuration, error) {
	if err := duration.Validate(); err != nil {
		return PlanDuration{}, fmt.Errorf(
			"validate plan duration: %w",
			err,
		)
	}

	duration.IsActive = true

	created, err := s.repository.CreateDuration(
		ctx,
		duration,
	)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"create plan duration: %w",
			err,
		)
	}

	return created, nil
}

func (s *Service) GetPlanVersion(
	ctx context.Context,
	code string,
	version int,
) (Plan, error) {
	if code == "" {
		return Plan{}, errors.New(
			"plan code is required",
		)
	}

	if version < 1 {
		return Plan{}, errors.New(
			"plan version must be greater than zero",
		)
	}

	plan, err := s.repository.GetByCodeAndVersion(
		ctx,
		code,
		version,
	)
	if err != nil {
		return Plan{}, fmt.Errorf(
			"get plan version: %w",
			err,
		)
	}

	return plan, nil
}

func (s *Service) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	if code == "" {
		return 0, errors.New(
			"plan code is required",
		)
	}

	version, err := s.repository.LatestVersion(
		ctx,
		code,
	)
	if err != nil {
		return 0, fmt.Errorf(
			"get latest plan version: %w",
			err,
		)
	}

	return version, nil
}

func (s *Service) ListVersions(
	ctx context.Context,
	code string,
) ([]Plan, error) {
	if code == "" {
		return nil, errors.New(
			"plan code is required",
		)
	}

	plans, err := s.repository.ListVersions(
		ctx,
		code,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list plan versions: %w",
			err,
		)
	}

	return plans, nil
}

func (s *Service) GetDuration(
	ctx context.Context,
	id int64,
) (PlanDuration, error) {
	if id <= 0 {
		return PlanDuration{}, errors.New(
			"plan duration id must be greater than zero",
		)
	}

	duration, err := s.repository.GetDuration(
		ctx,
		id,
	)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"get plan duration: %w",
			err,
		)
	}

	return duration, nil
}

func (s *Service) GetDurationByPlanAndCode(
	ctx context.Context,
	planID int64,
	duration time.Duration,
) (PlanDuration, error) {
	if planID <= 0 {
		return PlanDuration{}, errors.New(
			"plan id must be greater than zero",
		)
	}

	result, err := s.repository.GetDurationByPlanAndDuration(
		ctx,
		planID,
		duration,
	)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"get plan duration by plan and duration: %w",
			err,
		)
	}

	return result, nil
}

func (s *Service) ListDurations(
	ctx context.Context,
	planID int64,
) ([]PlanDuration, error) {
	if planID <= 0 {
		return nil, errors.New(
			"plan id must be greater than zero",
		)
	}

	durations, err := s.repository.ListDurations(
		ctx,
		planID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list plan durations: %w",
			err,
		)
	}

	return durations, nil
}

func (s *Service) UpdateDurationTariff(
	ctx context.Context,
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

	duration, err := s.repository.UpdateDurationTariff(
		ctx,
		durationID,
		tariffID,
		isActive,
	)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"update plan duration tariff: %w",
			err,
		)
	}

	return duration, nil
}
