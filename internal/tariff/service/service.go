package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/money"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
)

type Tariff = tariffmodel.Tariff
type Tier = tariffmodel.Tier
type TariffTypeCode = tariffmodel.TariffTypeCode
type Quantity = tariffmodel.Quantity
type QuantityUnit = tariffmodel.QuantityUnit
type TierStrategy = tariffmodel.TierStrategy

const (
	TariffTypeFlatFee     = tariffmodel.TariffTypeFlatFee
	TariffTypePerUnit     = tariffmodel.TariffTypePerUnit
	TariffTypeTieredUsage = tariffmodel.TariffTypeTieredUsage
)

const (
	TierStrategyVolume    = tariffmodel.TierStrategyVolume
	TierStrategyGraduated = tariffmodel.TierStrategyGraduated
)

const (
	UnitCount    = tariffmodel.UnitCount
	UnitSeat     = tariffmodel.UnitSeat
	UnitGigabyte = tariffmodel.UnitGigabyte
	UnitHour     = tariffmodel.UnitHour
	UnitAPICall  = tariffmodel.UnitAPICall
)

type Service struct {
	db         *pgxpool.Pool
	repository tariffrepo.Repository
}

func NewService(
	db *pgxpool.Pool,
	repository tariffrepo.Repository,
) *Service {
	return &Service{
		db:         db,
		repository: repository,
	}
}

func (s *Service) CreateTariff(
	ctx context.Context,
	code string,
	name string,
	description string,
	tariffType TariffTypeCode,
	tierStrategy TierStrategy,
	quantityUnit QuantityUnit,
	amount money.Money,
	tiers []Tier,
	metadata []byte,
) (Tariff, error) {
	if code == "" {
		return Tariff{}, errors.New("tariff code is required")
	}

	if name == "" {
		return Tariff{}, errors.New("tariff name is required")
	}

	if !tariffType.Valid() {
		return Tariff{}, fmt.Errorf(
			"unsupported tariff type %q",
			tariffType,
		)
	}

	tariff := Tariff{
		TariffCode:     code,
		Name:           name,
		Description:    description,
		TariffTypeCode: tariffType,
		TierStrategy:   tierStrategy,
		QuantityUnit:   quantityUnit,
		Amount:         amount,
		Tiers:          tiers,
		Metadata:       metadata,
		IsActive:       true,
	}

	if err := tariff.Validate(); err != nil {
		return Tariff{}, fmt.Errorf(
			"validate tariff: %w",
			err,
		)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Tariff{}, fmt.Errorf(
			"begin transaction: %w",
			err,
		)
	}
	defer tx.Rollback(ctx)

	created, err := s.repository.Create(ctx, tx, tariff)
	if err != nil {
		return Tariff{}, fmt.Errorf(
			"create tariff: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return Tariff{}, fmt.Errorf(
			"commit transaction: %w",
			err,
		)
	}

	return created, nil
}

func (s *Service) GetTariffVersion(
	ctx context.Context,
	code string,
	version int,
) (Tariff, error) {
	if code == "" {
		return Tariff{}, errors.New("tariff code is required")
	}

	if version < 1 {
		return Tariff{}, errors.New(
			"tariff version must be greater than zero",
		)
	}

	return s.repository.GetByCodeAndVersion(
		ctx,
		code,
		version,
	)
}

func (s *Service) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	if code == "" {
		return 0, errors.New("tariff code is required")
	}

	return s.repository.LatestVersion(ctx, code)
}

func (s *Service) ListVersions(
	ctx context.Context,
	code string,
) ([]Tariff, error) {
	if code == "" {
		return nil, errors.New("tariff code is required")
	}

	return s.repository.ListVersions(ctx, code)
}

func (s *Service) CalculateUsageCharge(
	ctx context.Context,
	code string,
	version int,
	qty Quantity,
) (money.Money, error) {
	tariff, err := s.GetTariffVersion(
		ctx,
		code,
		version,
	)
	if err != nil {
		return money.Money{}, fmt.Errorf(
			"load tariff: %w",
			err,
		)
	}

	charge, err := tariff.CalculateCharge(qty)
	if err != nil {
		return money.Money{}, fmt.Errorf(
			"calculate usage charge: %w",
			err,
		)
	}

	return charge, nil
}

var ErrTariffNotFound = tariffrepo.ErrTariffNotFound
