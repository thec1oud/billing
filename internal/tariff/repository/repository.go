package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/tariff/model"
)

var (
	ErrTariffNotFound = errors.New("tariff not found")
)

type DBTX interface {
	sqlcgen.DBTX
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

type Repository interface {
	Create(
		ctx context.Context,
		db DBTX,
		tariff model.Tariff,
	) (model.Tariff, error)

	GetByCodeAndVersion(
		ctx context.Context,
		code string,
		version int,
	) (model.Tariff, error)

	GetByID(
		ctx context.Context,
		id int64,
	) (model.Tariff, error)

	LatestVersion(
		ctx context.Context,
		code string,
	) (int, error)

	ListVersions(
		ctx context.Context,
		code string,
	) ([]model.Tariff, error)
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) getQuerier(db DBTX) *sqlcgen.Queries {
	if db != nil {
		return sqlcgen.New(db)
	}

	return sqlcgen.New(r.pool)
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	db DBTX,
	tariff model.Tariff,
) (model.Tariff, error) {
	if err := tariff.Validate(); err != nil {
		return model.Tariff{}, fmt.Errorf(
			"validate tariff: %w",
			err,
		)
	}

	q := r.getQuerier(db)

	tierBrackets, err := json.Marshal(tariff.Tiers)
	if err != nil {
		return model.Tariff{}, fmt.Errorf(
			"marshal tariff tiers: %w",
			err,
		)
	}

	description := pgtype.Text{
		String: tariff.Description,
		Valid:  tariff.Description != "",
	}

	tierStrategy := pgtype.Text{
		String: string(tariff.TierStrategy),
		Valid:  tariff.TierStrategy != "",
	}

	quantityUnit := pgtype.Text{
		String: string(tariff.QuantityUnit),
		Valid:  tariff.QuantityUnit != "",
	}

	metadata := tariff.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	row, err := q.CreateTariff(ctx, sqlcgen.CreateTariffParams{
		TariffCode:     tariff.TariffCode,
		Name:           tariff.Name,
		Description:    description,
		TariffTypeCode: string(tariff.TariffTypeCode),
		TierStrategy:   tierStrategy,
		QuantityUnit:   quantityUnit,
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
		return model.Tariff{}, fmt.Errorf(
			"create tariff: %w",
			err,
		)
	}

	return r.createRowToModel(row)
}

func (r *PostgresRepository) GetByCodeAndVersion(
	ctx context.Context,
	code string,
	version int,
) (model.Tariff, error) {
	q := r.getQuerier(nil)

	row, err := q.GetTariffByCodeAndVersion(
		ctx,
		sqlcgen.GetTariffByCodeAndVersionParams{
			TariffCode: code,
			Version:    int32(version),
		},
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Tariff{}, ErrTariffNotFound
		}

		return model.Tariff{}, fmt.Errorf(
			"get tariff by code and version: %w",
			err,
		)
	}

	return r.tariffRowToModel(
		row.TariffID,
		row.TariffCode,
		row.Version,
		row.Name,
		row.Description,
		row.TariffTypeCode,
		row.TierStrategy,
		row.QuantityUnit,
		row.Amount,
		row.Currency,
		row.TierBrackets,
		row.IsActive,
		row.Metadata,
		row.CreatedAt,
	)
}

func (r *PostgresRepository) GetByID(
	ctx context.Context,
	id int64,
) (model.Tariff, error) {
	q := r.getQuerier(nil)

	row, err := q.GetTariffByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Tariff{}, ErrTariffNotFound
		}

		return model.Tariff{}, fmt.Errorf(
			"get tariff by id: %w",
			err,
		)
	}

	return r.tariffRowToModel(
		row.TariffID,
		row.TariffCode,
		row.Version,
		row.Name,
		row.Description,
		row.TariffTypeCode,
		row.TierStrategy,
		row.QuantityUnit,
		row.Amount,
		row.Currency,
		row.TierBrackets,
		row.IsActive,
		row.Metadata,
		row.CreatedAt,
	)
}

func (r *PostgresRepository) LatestVersion(
	ctx context.Context,
	code string,
) (int, error) {
	q := r.getQuerier(nil)

	version, err := q.GetLatestTariffVersion(ctx, code)
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
) ([]model.Tariff, error) {
	q := r.getQuerier(nil)

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

	tariffs := make([]model.Tariff, 0, len(rows))

	for _, row := range rows {
		tariff, err := r.tariffRowToModel(
			row.TariffID,
			row.TariffCode,
			row.Version,
			row.Name,
			row.Description,
			row.TariffTypeCode,
			row.TierStrategy,
			row.QuantityUnit,
			row.Amount,
			row.Currency,
			row.TierBrackets,
			row.IsActive,
			row.Metadata,
			row.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"convert tariff version: %w",
				err,
			)
		}

		tariffs = append(tariffs, tariff)
	}

	return tariffs, nil
}

func (r *PostgresRepository) createRowToModel(
	row sqlcgen.CreateTariffRow,
) (model.Tariff, error) {
	return r.tariffRowToModel(
		row.TariffID,
		row.TariffCode,
		row.Version,
		row.Name,
		row.Description,
		row.TariffTypeCode,
		row.TierStrategy,
		row.QuantityUnit,
		row.Amount,
		row.Currency,
		row.TierBrackets,
		row.IsActive,
		row.Metadata,
		row.CreatedAt,
	)
}

func (r *PostgresRepository) tariffRowToModel(
	id int64,
	code string,
	version int32,
	name string,
	description pgtype.Text,
	tariffTypeCode string,
	tierStrategy pgtype.Text,
	quantityUnit pgtype.Text,
	amount pgtype.Int8,
	currency string,
	tierBrackets []byte,
	isActive bool,
	metadata []byte,
	createdAt time.Time,
) (model.Tariff, error) {
	if !amount.Valid {
		return model.Tariff{}, errors.New(
			"tariff amount is null",
		)
	}

	amountMoney, err := money.New(
		amount.Int64,
		currency,
	)
	if err != nil {
		return model.Tariff{}, fmt.Errorf(
			"create tariff amount: %w",
			err,
		)
	}

	var tiers []model.Tier

	if len(tierBrackets) > 0 {
		if err := json.Unmarshal(tierBrackets, &tiers); err != nil {
			return model.Tariff{}, fmt.Errorf(
				"unmarshal tariff tiers: %w",
				err,
			)
		}
	}

	desc := ""
	if description.Valid {
		desc = description.String
	}

	strategy := model.TierStrategy("")
	if tierStrategy.Valid {
		strategy = model.TierStrategy(tierStrategy.String)
	}

	unit := model.QuantityUnit("")
	if quantityUnit.Valid {
		unit = model.QuantityUnit(quantityUnit.String)
	}

	return model.Tariff{
		ID:             id,
		TariffCode:     code,
		Version:        int(version),
		Name:           name,
		Description:    desc,
		TariffTypeCode: model.TariffTypeCode(tariffTypeCode),
		TierStrategy:   strategy,
		QuantityUnit:   unit,
		Amount:         amountMoney,
		Tiers:          tiers,
		Metadata:       json.RawMessage(metadata),
		CreatedAt:      createdAt,
		IsActive:       isActive,
	}, nil
}
