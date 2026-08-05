package purchasable_item

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
)

var ErrPurchasableItemNotFound = errors.New("purchasable item not found")

type Repository interface {
	Create(
		ctx context.Context,
		tx pgx.Tx,
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

	q := sqlcgen.New(tx)

	row, err := q.CreatePurchasableItem(
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
	q := sqlcgen.New(r.pool)

	row, err := q.GetPurchasableItemByID(ctx, id)
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
	q := sqlcgen.New(r.pool)

	row, err := q.GetPurchasableItemByCode(ctx, code)
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
		ItemTypeCode: ItemTypeCode(row.ItemTypeCode),
		Name:         row.Name,
		Description:  description,
		PlanID:       planID,
		IsActive:     row.IsActive,
		Metadata:     metadata,
		CreatedAt:    row.CreatedAt,
	}, nil
}

