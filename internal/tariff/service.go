package tariff

import (
	"context"
	"fmt"

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

// CreateTariff calculates versioning and enforces immutability.
func (s *Service) CreateTariff(
	ctx context.Context,
	code string,
	name string,
	tariffType TariffTypeCode,
	amount money.Money,
	interval BillingInterval,
	tiers []Tier,
) (Tariff, error) {
	if code == "" {
		return Tariff{}, fmt.Errorf("tariff code cannot be empty")
	}

	version := 1
	if latest, err := s.repository.LatestVersion(ctx, code); err == nil && latest > 0 {
		version = latest + 1
	}

	tariff := Tariff{
		TariffCode:          code,
		Version:             version,
		Name:                name,
		TariffTypeCode:      tariffType,
		Amount:              amount,
		BillingIntervalCode: interval,
		IsActive:            true,
		Tiers:               tiers,
	}

	return s.repository.Save(ctx, tariff)
}

func (s *Service) GetTariffVersion(ctx context.Context, code string, version int) (Tariff, error) {
	return s.repository.GetByCodeAndVersion(ctx, code, version)
}

// CalculateUsageCharge computes charge using targeted tariff logic.
func (s *Service) CalculateUsageCharge(
	ctx context.Context,
	code string,
	version int,
	qty Quantity,
) (money.Money, error) {
	t, err := s.GetTariffVersion(ctx, code, version)
	if err != nil {
		return money.Money{}, fmt.Errorf("failed to load tariff: %w", err)
	}

	return t.CalculateCharge(qty)
}