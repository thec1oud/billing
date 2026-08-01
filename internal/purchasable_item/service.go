package purchasable_item

import (
	"context"
	"fmt"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

// CreateItem validates and registers a new purchasable item.
func (s *Service) CreateItem(
	ctx context.Context,
	code string,
	itemType ItemTypeCode,
	name string,
	description *string,
	planID *int64,
) (PurchasableItem, error) {
	if code == "" {
		return PurchasableItem{}, fmt.Errorf("item code cannot be empty")
	}
	if name == "" {
		return PurchasableItem{}, fmt.Errorf("item name cannot be empty")
	}

	item := PurchasableItem{
		ItemCode:     code,
		ItemTypeCode: itemType,
		Name:         name,
		Description:  description,
		PlanID:       planID,
		IsActive:     true,
	}

	return s.repository.Save(ctx, item)
}

func (s *Service) GetByCode(ctx context.Context, code string) (PurchasableItem, error) {
	return s.repository.GetByCode(ctx, code)
}