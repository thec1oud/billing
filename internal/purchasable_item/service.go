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
		// A PLAN item may have a nil PlanID.
	case ItemTypeOneTimeService,
		ItemTypeProduct:
		if item.PlanID != nil {
			return PurchasableItem{}, fmt.Errorf(
				"%s item cannot reference a plan",
				item.ItemTypeCode,
			)
		}
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

func (s *Service) ListAll(
	ctx context.Context,
) ([]PurchasableItem, error) {
	return s.repository.ListAll(ctx)
}
