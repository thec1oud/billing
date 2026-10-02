package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	itemmodel "github.com/thec1oud/billing/internal/purchasable_item/model"
	itemrepo "github.com/thec1oud/billing/internal/purchasable_item/repository"
)

type PurchasableItem = itemmodel.PurchasableItem
type ItemTypeCode = itemmodel.ItemTypeCode

var ErrPurchasableItemNotFound = itemrepo.ErrPurchasableItemNotFound

const ItemTypePlan = itemmodel.ItemTypePlan
const ItemTypeOneTimeService = itemmodel.ItemTypeOneTimeService
const ItemTypeProduct = itemmodel.ItemTypeProduct

type Service struct {
	db         *pgxpool.Pool
	repository itemrepo.Repository
}

func NewService(
	db *pgxpool.Pool,
	repository itemrepo.Repository,
) *Service {
	return &Service{
		db:         db,
		repository: repository,
	}
}

func (s *Service) Create(
	ctx context.Context,
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return PurchasableItem{}, fmt.Errorf(
			"begin transaction: %w",
			err,
		)
	}
	defer tx.Rollback(ctx)

	created, err := s.repository.Create(ctx, tx, item)
	if err != nil {
		return PurchasableItem{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PurchasableItem{}, fmt.Errorf(
			"commit transaction: %w",
			err,
		)
	}

	return created, nil
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

func (s *Service) GetByPlanID(
	ctx context.Context,
	planID int64,
) (PurchasableItem, error) {
	if planID <= 0 {
		return PurchasableItem{}, errors.New(
			"plan id must be greater than zero",
		)
	}

	return s.repository.GetByPlanID(ctx, planID)
}

func (s *Service) ListAll(
	ctx context.Context,
) ([]PurchasableItem, error) {
	return s.repository.ListAll(ctx)
}