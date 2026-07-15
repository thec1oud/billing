package plan

import (
	"context"

	"github.com/thec1oud/billing/internal/substrate/money"
)

type Service struct {
	repository *Repository
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: repository,
	}
}

// CreatePlan creates a new immutable plan version.
//
// If this is the first time the plan ID is used,
// Version = 1.
//
// Otherwise the latest version is found and the new
// plan becomes LatestVersion + 1.
func (s *Service) CreatePlan(
	ctx context.Context,
	id string,
	amount money.Money,
	period BillingPeriod,
) (Plan, error) {

	version := 1

	if latest, err := s.repository.LatestVersion(ctx, id); err == nil {
		version = latest + 1
	}

	plan := Plan{
		ID:            id,
		Version:       version,
		FlatFeeAmount: amount,
		BillingPeriod: period,
	}

	return s.repository.Save(ctx, plan)
}
