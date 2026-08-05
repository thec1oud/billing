package plan

import (
	"context"

	"github.com/thec1oud/billing/internal/shared/money"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

// CreatePlan calculates the next version and persists an immutable plan.
func (s *Service) CreatePlan(
	ctx context.Context,
	code string,
	amount money.Money,
	interval BillingInterval,
) (Plan, error) {

	version := 1

	if latest, err := s.repository.LatestVersion(ctx, code); err == nil && latest > 0 {
		version = latest + 1
	}

	plan := Plan{
		PlanCode:      code,
		Version:       version,
		FlatFeeAmount: amount,
		Interval:      interval,
	}

	return s.repository.Save(ctx, plan)
}

// GetPlanVersion retrieves a targeted version of a plan.
func (s *Service) GetPlanVersion(ctx context.Context, code string, version int) (Plan, error) {
	return s.repository.GetByCodeAndVersion(ctx, code, version)
}
