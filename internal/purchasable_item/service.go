package purchasable_item

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Create(
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

	switch item.ItemTypeCode {
	case ItemTypePlan:
		// PLAN items are allowed to exist without an attached plan reference.
		// The database column is nullable, so the service should not reject
		// a valid plan item solely because the caller did not supply PlanID.

	case ItemTypeOneTimeService,
		ItemTypeProduct:
		if item.PlanID != nil {
			return PurchasableItem{}, fmt.Errorf(
				"%s item cannot reference a plan",
				item.ItemTypeCode,
			)
		}

	default:
		return PurchasableItem{}, fmt.Errorf(
			"unsupported item type %q",
			item.ItemTypeCode,
		)
	}

	return s.repository.Create(ctx, tx, item)
}

func (s *Service) GetByID(
	ctx context.Context,
	id int64,
) (PurchasableItem, error) {
	if id <= 0 {
		return PurchasableItem{}, errors.New(
			"item id must be greater than zero",
		)
	}

	return s.repository.GetByID(ctx, id)
}

func (s *Service) GetByCode(
	ctx context.Context,
	code string,
) (PurchasableItem, error) {
	if code == "" {
		return PurchasableItem{}, errors.New(
			"item code is required",
		)
	}

	return s.repository.GetByCode(ctx, code)
}

