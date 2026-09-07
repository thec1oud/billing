package plan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

// CreatePlan creates a new immutable version of a plan.
//
// The service owns version assignment. The caller owns the transaction.
// Existing plan versions are never updated or deleted.
func (s *Service) CreatePlan(
	ctx context.Context,
	tx pgx.Tx,
	plan Plan,
) (Plan, error) {
	plan.PlanCode = strings.TrimSpace(plan.PlanCode)

	if plan.LegacyPricePolicyCode == "" {
		plan.LegacyPricePolicyCode = LegacyPolicyKeepForever
	}

	if plan.EffectiveFrom.IsZero() {
		plan.EffectiveFrom = time.Now().UTC()
	}

	latestVersion, err := s.repository.LatestVersion(
		ctx,
		plan.PlanCode,
	)
	if err != nil {
		return Plan{}, fmt.Errorf(
			"get latest plan version: %w",
			err,
		)
	}

	// The service owns version assignment.
	// The caller-provided version is deliberately ignored.
	plan.Version = latestVersion + 1

	if plan.Version < 1 {
		return Plan{}, errors.New(
			"next plan version must be greater than zero",
		)
	}

	if err := plan.Validate(); err != nil {
		return Plan{}, fmt.Errorf(
			"validate plan: %w",
			err,
		)
	}

	createdPlan, err := s.repository.Create(
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

	return createdPlan, nil
}

// CreatePlanDuration creates a new duration/tariff association.
//
// Plan durations are append-only. Existing duration/tariff associations
// are not updated through the service.
func (s *Service) CreatePlanDuration(
	ctx context.Context,
	tx pgx.Tx,
	duration PlanDuration,
) (PlanDuration, error) {
	if duration.PlanID <= 0 {
		return PlanDuration{}, errors.New(
			"plan ID must be greater than zero",
		)
	}

	if duration.TariffID <= 0 {
		return PlanDuration{}, errors.New(
			"tariff ID must be greater than zero",
		)
	}

	if duration.Duration <= 0 {
		return PlanDuration{}, errors.New(
			"duration must be greater than zero",
		)
	}

	// The current database schema defaults new durations to active.
	// PlanDuration uses bool, so there is no separate "unset" state.
	duration.IsActive = true

	createdDuration, err := s.repository.CreateDuration(
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

	return createdDuration, nil
}

func (s *Service) GetPlanVersion(
	ctx context.Context,
	code string,
	version int,
) (Plan, error) {
	code = strings.TrimSpace(code)

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

func (s *Service) GetPlanByIDAndVersion(
	ctx context.Context,
	id int64,
	version int,
) (Plan, error) {
	if id <= 0 {
		return Plan{}, errors.New(
			"plan ID must be greater than zero",
		)
	}

	if version < 1 {
		return Plan{}, errors.New(
			"plan version must be greater than zero",
		)
	}

	plan, err := s.repository.GetByIDAndVersion(
		ctx,
		id,
		version,
	)
	if err != nil {
		return Plan{}, fmt.Errorf(
			"get plan by ID and version: %w",
			err,
		)
	}

	return plan, nil
}

func (s *Service) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	code = strings.TrimSpace(code)

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
	code = strings.TrimSpace(code)

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
			"plan duration ID must be greater than zero",
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
			"plan ID must be greater than zero",
		)
	}

	if duration <= 0 {
		return PlanDuration{}, errors.New(
			"duration must be greater than zero",
		)
	}

	planDuration, err := s.repository.GetDurationByPlanAndDuration(
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

	return planDuration, nil
}

func (s *Service) ListDurations(
	ctx context.Context,
	planID int64,
) ([]PlanDuration, error) {
	if planID <= 0 {
		return nil, errors.New(
			"plan ID must be greater than zero",
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
