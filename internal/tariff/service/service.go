package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"

	"github.com/thec1oud/billing/internal/shared/money"
	tariffmodel "github.com/thec1oud/billing/internal/tariff/model"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
)

type Tariff = tariffmodel.Tariff
type Tier = tariffmodel.Tier
type TariffTypeCode = tariffmodel.TariffTypeCode
type Quantity = tariffmodel.Quantity

const TariffTypeFlatFee = tariffmodel.TariffTypeFlatFee
const TariffTypePerUnit = tariffmodel.TariffTypePerUnit
const TariffTypeTieredUsage = tariffmodel.TariffTypeTieredUsage

type Service struct {
	repository tariffrepo.Repository
}

func NewService(repository tariffrepo.Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) CreateTariff(
	ctx context.Context,
	tx pgx.Tx,
	code string,
	name string,
	description string,
	tariffType TariffTypeCode,
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

	for attempt := 0; attempt < 3; attempt++ {
		latest, err := s.repository.LatestVersion(ctx, code)
		if err != nil {
			if errors.Is(err, tariffrepo.ErrTariffNotFound) {
				latest = 0
			} else {
				return Tariff{}, fmt.Errorf(
					"get latest tariff version: %w",
					err,
				)
			}
		}

		tariff := Tariff{
			TariffCode:     code,
			Version:        latest + 1,
			Name:           name,
			Description:    description,
			TariffTypeCode: tariffType,
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

		created, err := s.repository.Create(ctx, tx, tariff)
		if err == nil {
			return created, nil
		}

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == pgerrcode.UniqueViolation &&
			pgErr.ConstraintName == "uq_tariff_code_version" {
			continue
		}

		return Tariff{}, fmt.Errorf("create tariff: %w", err)
	}

	return Tariff{}, errors.New("create tariff: retry limit exceeded")
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

	return tariff.CalculateCharge(qty)
}

var ErrTariffNotFound = tariffrepo.ErrTariffNotFound
