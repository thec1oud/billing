package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	planmodel "github.com/thec1oud/billing/internal/plan/model"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
)

type Plan = planmodel.Plan
type PlanDuration = planmodel.PlanDuration
type LegacyPricePolicy = planmodel.LegacyPricePolicy

var (
	ErrPlanNotFound         = planrepo.ErrPlanNotFound
	ErrPlanDurationNotFound = planrepo.ErrPlanDurationNotFound
)

const (
	LegacyPolicyKeepForever        = planmodel.LegacyPolicyKeepForever
	LegacyPolicyMigrateImmediately = planmodel.LegacyPolicyMigrateImmediately
	LegacyPolicyMigrateOnRenewal   = planmodel.LegacyPolicyMigrateOnRenewal
)

type Service struct {
	db         *pgxpool.Pool
	repository planrepo.Repository
}

func NewService(
	db *pgxpool.Pool,
	repository planrepo.Repository,
) *Service {
	return &Service{
		db:         db,
		repository: repository,
	}
}


func (s *Service) CreatePlan(
	ctx context.Context,
	plan Plan,
) (Plan, error) {
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
		return Plan{}, fmt.Errorf(
			"begin create plan transaction: %w",
			err,
		)
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

	for i, duration := range plan.Durations {
		duration.PlanID = created.ID
		duration.IsActive = true

		createdDuration, err := s.repository.CreateDuration(
			ctx,
			tx,
			duration,
		)
		if err != nil {
			return Plan{}, fmt.Errorf(
				"create plan duration %d: %w",
				i,
				err,
			)
		}

		created.Durations = append(
			created.Durations,
			createdDuration,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Plan{}, fmt.Errorf(
			"commit create plan transaction: %w",
			err,
		)
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"begin create plan duration transaction: %w",
			err,
		)
	}
	defer tx.Rollback(ctx)

	created, err := s.repository.CreateDuration(
		ctx,
		tx,
		duration,
	)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"create plan duration: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return PlanDuration{}, fmt.Errorf(
			"commit create plan duration transaction: %w",
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

func (s *Service) ListActivePlans(
	ctx context.Context,
) ([]Plan, error) {
	plans, err := s.repository.ListActivePlans(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"list active plans: %w",
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

func (s *Service) GetDurationByPlanAndDuration(
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return PlanDuration{}, fmt.Errorf(
			"begin update plan duration transaction: %w",
			err,
		)
	}
	defer tx.Rollback(ctx)

	duration, err := s.repository.UpdateDurationTariff(
		ctx,
		tx,
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

	if err := tx.Commit(ctx); err != nil {
		return PlanDuration{}, fmt.Errorf(
			"commit update plan duration transaction: %w",
			err,
		)
	}

	return duration, nil
}