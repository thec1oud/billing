package tariff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

var (
	ErrTariffNotFound = errors.New("tariff not found")
)

type Repository interface {
	Create(ctx context.Context, tx pgx.Tx, tariff Tariff) (Tariff, error)
	GetByCodeAndVersion(ctx context.Context, code string, version int) (Tariff, error)
	LatestVersion(ctx context.Context, code string) (int, error)
	ListVersions(ctx context.Context, code string) ([]Tariff, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	tx pgx.Tx,
	tariff Tariff,
) (Tariff, error) {
	q := sqlcgen.New(tx)

	tierBrackets, err := json.Marshal(tariff.Tiers)
	if err != nil {
		return Tariff{}, fmt.Errorf("marshal tier brackets: %w", err)
	}

	var amount *int64
	var currency *string

	if tariff.Amount != nil {
		amountValue := tariff.Amount.AmountMinor
		currencyValue := string(tariff.Amount.Currency)

		amount = &amountValue
		currency = &currencyValue
	}

	var billingInterval *string
	var intervalCount *int32

	if tariff.Duration != nil {
		interval := string(tariff.Duration.Interval)
		count := int32(tariff.Duration.Count)

		billingInterval = &interval
		intervalCount = &count
	}

	row, err := q.CreateTariff(ctx, sqlcgen.CreateTariffParams{
		TariffCode:          tariff.TariffCode,
		Version:             int32(tariff.Version),
		Name:                tariff.Name,
		Description:         tariff.Description,
		TariffTypeCode:      string(tariff.TariffTypeCode),
		Amount:              amount,
		Currency:            currency,
		BillingIntervalCode: billingInterval,
		IntervalCount:       intervalCount,
		TierBrackets:        tierBrackets,
		IsActive:            tariff.IsActive,
		Metadata:            tariff.Metadata,
	})
	if err != nil {
		return Tariff{}, fmt.Errorf("create tariff: %w", err)
	}

	tariff.ID = row.TariffID
	tariff.Version = int(row.Version)
	tariff.CreatedAt = row.CreatedAt

	return tariff, nil
}

func (r *PostgresRepository) GetByCodeAndVersion(
	ctx context.Context,
	code string,
	version int,
) (Tariff, error) {
	q := sqlcgen.New(r.pool)

	row, err := q.GetTariffByCodeAndVersion(
		ctx,
		sqlcgen.GetTariffByCodeAndVersionParams{
			TariffCode: code,
			Version:    int32(version),
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tariff{}, ErrTariffNotFound
	}
	if err != nil {
		return Tariff{}, fmt.Errorf("get tariff: %w", err)
	}

	var amount *money.Money

	if row.Amount.Valid {
		if !row.Currency.Valid {
			return Tariff{}, fmt.Errorf(
				"tariff %s v%d has amount without currency",
				row.TariffCode,
				row.Version,
			)
		}

		m, err := money.New(
			row.Amount.Int64,
			row.Currency.String,
		)
		if err != nil {
			return Tariff{}, fmt.Errorf("create tariff money: %w", err)
		}

		amount = &m
	}

	var duration *BillingDuration

	if row.BillingIntervalCode.Valid {
		if !row.IntervalCount.Valid || row.IntervalCount.Int32 <= 0 {
			return Tariff{}, fmt.Errorf(
				"tariff %s v%d has invalid billing duration",
				row.TariffCode,
				row.Version,
			)
		}

		duration = &BillingDuration{
			Count:    int(row.IntervalCount.Int32),
			Interval: BillingInterval(row.BillingIntervalCode.String),
		}
	}

	var tiers []Tier

	if len(row.TierBrackets) > 0 {
		if err := json.Unmarshal(row.TierBrackets, &tiers); err != nil {
			return Tariff{}, fmt.Errorf("decode tariff tiers: %w", err)
		}
	}

	return Tariff{
		ID:             row.TariffID,
		TariffCode:     row.TariffCode,
		Version:        int(row.Version),
		Name:           row.Name,
		Description:    row.Description,
		TariffTypeCode: TariffTypeCode(row.TariffTypeCode),
		Amount:         amount,
		Duration:       duration,
		IsActive:       row.IsActive,
		Tiers:          tiers,
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
	}, nil
}

func (r *PostgresRepository) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	q := sqlcgen.New(r.pool)

	version, err := q.GetLatestTariffVersion(ctx, code)
	if err != nil {
		return 0, fmt.Errorf("get latest tariff version: %w", err)
	}

	return int(version), nil
}

func (r *PostgresRepository) ListVersions(
	ctx context.Context,
	code string,
) ([]Tariff, error) {
	q := sqlcgen.New(r.pool)

	rows, err := q.ListTariffVersions(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("list tariff versions: %w", err)
	}

	tariffs := make([]Tariff, 0, len(rows))

	for _, row := range rows {
		var amount *money.Money

		if row.Amount.Valid {
			if !row.Currency.Valid {
				return nil, fmt.Errorf(
					"tariff %s v%d has amount without currency",
					row.TariffCode,
					row.Version,
				)
			}

			m, err := money.New(row.Amount.Int64, row.Currency.String)
			if err != nil {
				return nil, fmt.Errorf("create tariff money: %w", err)
			}

			amount = &m
		}

		var duration *BillingDuration

		if row.BillingIntervalCode.Valid {
			duration = &BillingDuration{
				Count:    int(row.IntervalCount.Int32),
				Interval: BillingInterval(row.BillingIntervalCode.String),
			}
		}

		var tiers []Tier

		if len(row.TierBrackets) > 0 {
			if err := json.Unmarshal(row.TierBrackets, &tiers); err != nil {
				return nil, fmt.Errorf("decode tariff tiers: %w", err)
			}
		}

		tariffs = append(tariffs, Tariff{
			ID:             row.TariffID,
			TariffCode:     row.TariffCode,
			Version:        int(row.Version),
			Name:           row.Name,
			Description:    row.Description,
			TariffTypeCode: TariffTypeCode(row.TariffTypeCode),
			Amount:         amount,
			Duration:       duration,
			IsActive:       row.IsActive,
			Tiers:          tiers,
			Metadata:       row.Metadata,
			CreatedAt:      row.CreatedAt,
		})
	}

	return tariffs, nil
}
