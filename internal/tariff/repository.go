package tariff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

var ErrTariffNotFound = errors.New("tariff not found")

type Repository interface {
	Create(ctx context.Context, tariff Tariff) (Tariff, error)
	GetByCodeAndVersion(
		ctx context.Context,
		code string,
		version int,
	) (Tariff, error)
	GetByID(ctx context.Context, id int64) (Tariff, error)
	LatestVersion(ctx context.Context, code string) (int, error)
	ListVersions(ctx context.Context, code string) ([]Tariff, error)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (Tariff, error) {
	if id <= 0 {
		return Tariff{}, errors.New("tariff id must be greater than zero")
	}
	row, err := sqlcgen.New(r.pool).GetTariffByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tariff{}, ErrTariffNotFound
	}
	if err != nil {
		return Tariff{}, fmt.Errorf("get tariff by id: %w", err)
	}
	return toModel(row)
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
	tariff Tariff,
) (Tariff, error) {
	if err := tariff.Validate(); err != nil {
		return Tariff{}, fmt.Errorf("validate tariff: %w", err)
	}

	tierBrackets, err := json.Marshal(tariff.Tiers)
	if err != nil {
		return Tariff{}, fmt.Errorf(
			"marshal tier brackets: %w",
			err,
		)
	}

	description := pgtype.Text{
		String: tariff.Description,
		Valid:  tariff.Description != "",
	}

	metadata := tariff.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	q := sqlcgen.New(r.pool)

	row, err := q.CreateTariff(ctx, sqlcgen.CreateTariffParams{
		TariffCode:     tariff.TariffCode,
		Version:        int32(tariff.Version),
		Name:           tariff.Name,
		Description:    description,
		TariffTypeCode: string(tariff.TariffTypeCode),
		Amount: pgtype.Int8{
			Int64: tariff.Amount.AmountMinor,
			Valid: true,
		},
		Currency:     string(tariff.Amount.Currency),
		TierBrackets: tierBrackets,
		IsActive:     tariff.IsActive,
		Metadata:     metadata,
	})
	if err != nil {
		return Tariff{}, fmt.Errorf("create tariff: %w", err)
	}

	return toModel(row)
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

	return toModel(row)
}

func (r *PostgresRepository) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	q := sqlcgen.New(r.pool)

	version, err := q.GetLatestTariffVersion(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrTariffNotFound
	}

	if err != nil {
		return 0, fmt.Errorf(
			"get latest tariff version: %w",
			err,
		)
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
		return nil, fmt.Errorf(
			"list tariff versions: %w",
			err,
		)
	}
	if len(rows) == 0 {
		return nil, ErrTariffNotFound
	}

	tariffs := make([]Tariff, 0, len(rows))

	for _, row := range rows {
		tariff, err := toModel(row)
		if err != nil {
			return nil, err
		}

		tariffs = append(tariffs, tariff)
	}

	return tariffs, nil
}

func toModel(row sqlcgen.Tariff) (Tariff, error) {
	amount, err := money.New(
		row.Amount.Int64,
		row.Currency,
	)
	if err != nil {
		return Tariff{}, fmt.Errorf(
			"create tariff money: %w",
			err,
		)
	}

	var tiers []Tier

	if len(row.TierBrackets) > 0 {
		if err := json.Unmarshal(
			row.TierBrackets,
			&tiers,
		); err != nil {
			return Tariff{}, fmt.Errorf(
				"decode tier brackets: %w",
				err,
			)
		}
	}

	description := ""
	if row.Description.Valid {
		description = row.Description.String
	}

	return Tariff{
		ID:             row.TariffID,
		TariffCode:     row.TariffCode,
		Version:        int(row.Version),
		Name:           row.Name,
		Description:    description,
		TariffTypeCode: TariffTypeCode(row.TariffTypeCode),
		Amount:         amount,
		Tiers:          tiers,
		Metadata:       row.Metadata,
		CreatedAt:      row.CreatedAt,
		IsActive:       row.IsActive,
	}, nil
}
