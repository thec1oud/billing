package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

type PurchasableItem = itemmodel.PurchasableItem

var ErrPurchasableItemNotFound = errors.New("purchasable item not found")

type DBTX interface {
	sqlcgen.DBTX
}

type Repository interface {
	Pool() *pgxpool.Pool

	Create(
		ctx context.Context,
		db DBTX,
		item PurchasableItem,
	) (PurchasableItem, error)

	GetByID(
		ctx context.Context,
		id int64,
	) (PurchasableItem, error)

	GetByCode(
		ctx context.Context,
		code string,
	) (PurchasableItem, error)

	GetByPlanID(
		ctx context.Context,
		planID int64,
	) (PurchasableItem, error)

	ListAll(
		ctx context.Context,
	) ([]PurchasableItem, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool: pool,
	}
}

func (r *PostgresRepository) Pool() *pgxpool.Pool {
	return r.pool
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
	item PurchasableItem,
) (PurchasableItem, error) {
	if err := item.Validate(); err != nil {
		return PurchasableItem{}, fmt.Errorf(
			"validate purchasable item: %w",
			err,
		)
	}

	metadata := item.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	description := pgtype.Text{}
	if item.Description != nil {
		description = pgtype.Text{
			String: *item.Description,
			Valid:  true,
		}
	}

	var planID pgtype.Int8
	if item.PlanID != nil {
		planID = pgtype.Int8{
			Int64: *item.PlanID,
			Valid: true,
		}
	}

	row, err := r.getQuerier(db).CreatePurchasableItem(
		ctx,
		sqlcgen.CreatePurchasableItemParams{
			ItemCode:     item.ItemCode,
			ItemTypeCode: string(item.ItemTypeCode),
			Name:         item.Name,
			Description:  description,
			PlanID:       planID,
			IsActive:     item.IsActive,
			Metadata:     metadata,
		},
	)
	if err != nil {
		return PurchasableItem{}, fmt.Errorf(
			"create purchasable item: %w",
			err,
		)
	}

	return toModel(row)
}

func (r *PostgresRepository) GetByID(
	ctx context.Context,
	id int64,
) (PurchasableItem, error) {
	row, err := r.getQuerier(nil).GetPurchasableItemByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return PurchasableItem{}, ErrPurchasableItemNotFound
	}

	if err != nil {
		return PurchasableItem{}, fmt.Errorf(
			"get purchasable item: %w",
			err,
		)
	}

	return toModel(row)
}

func (r *PostgresRepository) GetByCode(
	ctx context.Context,
	code string,
) (PurchasableItem, error) {
	row, err := r.getQuerier(nil).GetPurchasableItemByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return PurchasableItem{}, ErrPurchasableItemNotFound
	}

	if err != nil {
		return PurchasableItem{}, fmt.Errorf(
			"get purchasable item by code: %w",
			err,
		)
	}

	return toModel(row)
}

func (r *PostgresRepository) GetByPlanID(
	ctx context.Context,
	planID int64,
) (PurchasableItem, error) {
	row, err := r.getQuerier(nil).GetPurchasableItemByPlanID(
		ctx,
		pgtype.Int8{
			Int64: planID,
			Valid: true,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PurchasableItem{}, ErrPurchasableItemNotFound
	}

	if err != nil {
		return PurchasableItem{}, fmt.Errorf(
			"get purchasable item by plan: %w",
			err,
		)
	}

	return toModel(row)
}

func (r *PostgresRepository) ListAll(
	ctx context.Context,
) ([]PurchasableItem, error) {
	rows, err := r.getQuerier(nil).ListPurchasableItems(ctx)
	if err != nil {
		return nil, fmt.Errorf(
			"list purchasable items: %w",
			err,
		)
	}

	items := make([]PurchasableItem, 0, len(rows))

	for _, row := range rows {
		item, err := toModel(row)
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	return items, nil
}

func toModel(row sqlcgen.PurchasableItem) (PurchasableItem, error) {
	var description *string

	if row.Description.Valid {
		value := row.Description.String
		description = &value
	}

	var planID *int64

	if row.PlanID.Valid {
		value := row.PlanID.Int64
		planID = &value
	}

	metadata := row.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}

	return PurchasableItem{
		ID:           row.ItemID,
		ItemCode:     row.ItemCode,
		ItemTypeCode: itemmodel.ItemTypeCode(row.ItemTypeCode),
		Name:         row.Name,
		Description:  description,
		PlanID:       planID,
		IsActive:     row.IsActive,
		Metadata:     metadata,
		CreatedAt:    row.CreatedAt,
	}, nil
}

var _ Repository = (*PostgresRepository)(nil)