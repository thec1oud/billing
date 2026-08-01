package purchasable_item

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Repository interface {
	Save(ctx context.Context, item PurchasableItem) (PurchasableItem, error)
	GetByCode(ctx context.Context, code string) (PurchasableItem, error)
	GetByID(ctx context.Context, id int64) (PurchasableItem, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Save(ctx context.Context, item PurchasableItem) (PurchasableItem, error) {
	query := `
		INSERT INTO purchasable_items (
			item_code, item_type_code, name, description, plan_id, is_active, metadata
		) VALUES (
			$1, $2, $3, $4, $5, COALESCE($6, TRUE), COALESCE($7, '{}'::jsonb)
		)
		RETURNING item_id, is_active, metadata, created_at, updated_at;
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		item.ItemCode,
		item.ItemTypeCode,
		item.Name,
		item.Description,
		item.PlanID,
		item.IsActive,
		item.Metadata,
	).Scan(
		&item.ID,
		&item.IsActive,
		&item.Metadata,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return PurchasableItem{}, fmt.Errorf("failed to insert purchasable item: %w", err)
	}

	return item, nil
}

func (r *postgresRepository) GetByCode(ctx context.Context, code string) (PurchasableItem, error) {
	query := `
		SELECT 
			item_id, item_code, item_type_code, name, description, plan_id, is_active, metadata, created_at, updated_at
		FROM purchasable_items
		WHERE item_code = $1;
	`

	var item PurchasableItem
	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&item.ID,
		&item.ItemCode,
		&item.ItemTypeCode,
		&item.Name,
		&item.Description,
		&item.PlanID,
		&item.IsActive,
		&item.Metadata,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PurchasableItem{}, fmt.Errorf("purchasable item %s not found", code)
		}
		return PurchasableItem{}, fmt.Errorf("failed to query purchasable item: %w", err)
	}

	return item, nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id int64) (PurchasableItem, error) {
	query := `
		SELECT 
			item_id, item_code, item_type_code, name, description, plan_id, is_active, metadata, created_at, updated_at
		FROM purchasable_items
		WHERE item_id = $1;
	`

	var item PurchasableItem
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.ItemCode,
		&item.ItemTypeCode,
		&item.Name,
		&item.Description,
		&item.PlanID,
		&item.IsActive,
		&item.Metadata,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PurchasableItem{}, fmt.Errorf("purchasable item with id %d not found", id)
		}
		return PurchasableItem{}, fmt.Errorf("failed to query purchasable item: %w", err)
	}

	return item, nil
}