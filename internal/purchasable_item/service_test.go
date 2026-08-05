package purchasable_item

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type fakePurchasableItemRepository struct {
	item         PurchasableItem
	createErr    error
	getByIDErr   error
	getByCodeErr error
}

func (f *fakePurchasableItemRepository) Create(
	_ context.Context,
	_ pgx.Tx,
	item PurchasableItem,
) (PurchasableItem, error) {
	if f.createErr != nil {
		return PurchasableItem{}, f.createErr
	}

	item.ID = 1
	item.CreatedAt = time.Now()

	f.item = item

	return item, nil
}

func (f *fakePurchasableItemRepository) GetByID(
	_ context.Context,
	id int64,
) (PurchasableItem, error) {
	if f.getByIDErr != nil {
		return PurchasableItem{}, f.getByIDErr
	}

	if f.item.ID != id {
		return PurchasableItem{}, ErrPurchasableItemNotFound
	}

	return f.item, nil
}

func (f *fakePurchasableItemRepository) GetByCode(
	_ context.Context,
	code string,
) (PurchasableItem, error) {
	if f.getByCodeErr != nil {
		return PurchasableItem{}, f.getByCodeErr
	}

	if f.item.ItemCode != code {
		return PurchasableItem{}, ErrPurchasableItemNotFound
	}

	return f.item, nil
}

func TestService_CreatePurchasableItem(t *testing.T) {
	t.Parallel()

	repo := &fakePurchasableItemRepository{}

	service := NewService(repo)

	result, err := service.Create(
		context.Background(),
		nil,
		PurchasableItem{
			ItemCode:     "PRO_PLAN",
			ItemTypeCode: ItemTypePlan,
			Name:         "Pro Plan",
			IsActive:     true,
		},
	)

	if err != nil {
		t.Fatalf(
			"Create() error = %v",
			err,
		)
	}

	if result.ID != 1 {
		t.Errorf(
			"ID = %d, want %d",
			result.ID,
			1,
		)
	}

	if result.ItemCode != "PRO_PLAN" {
		t.Errorf(
			"ItemCode = %q, want %q",
			result.ItemCode,
			"PRO_PLAN",
		)
	}

	if result.ItemTypeCode != ItemTypePlan {
		t.Errorf(
			"ItemTypeCode = %q, want %q",
			result.ItemTypeCode,
			ItemTypePlan,
		)
	}
}

func TestService_CreatePurchasableItem_ValidationError(t *testing.T) {
	t.Parallel()

	repo := &fakePurchasableItemRepository{}

	service := NewService(repo)

	_, err := service.Create(
		context.Background(),
		nil,
		PurchasableItem{
			ItemTypeCode: ItemTypePlan,
			Name:         "Pro Plan",
		},
	)

	if err == nil {
		t.Fatal("Create() expected error, got nil")
	}
}

func TestService_CreatePurchasableItem_EmptyName(t *testing.T) {
	t.Parallel()

	service := NewService(
		&fakePurchasableItemRepository{},
	)

	_, err := service.Create(
		context.Background(),
		nil,
		PurchasableItem{
			ItemCode:     "PRO_PLAN",
			ItemTypeCode: ItemTypePlan,
		},
	)

	if err == nil {
		t.Fatal("Create() expected error, got nil")
	}
}

func TestService_CreatePurchasableItem_RepositoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("database error")

	repo := &fakePurchasableItemRepository{
		createErr: expectedErr,
	}

	service := NewService(repo)

	_, err := service.Create(
		context.Background(),
		nil,
		PurchasableItem{
			ItemCode:     "PRO_PLAN",
			ItemTypeCode: ItemTypePlan,
			Name:         "Pro Plan",
		},
	)

	if err == nil {
		t.Fatal("Create() expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf(
			"Create() error = %v, want wrapped %v",
			err,
			expectedErr,
		)
	}
}

func TestService_GetByID(t *testing.T) {
	t.Parallel()

	repo := &fakePurchasableItemRepository{
		item: PurchasableItem{
			ID:           10,
			ItemCode:     "PRO_PLAN",
			ItemTypeCode: ItemTypePlan,
			Name:         "Pro Plan",
			IsActive:     true,
		},
	}

	service := NewService(repo)

	result, err := service.GetByID(
		context.Background(),
		10,
	)

	if err != nil {
		t.Fatalf(
			"GetByID() error = %v",
			err,
		)
	}

	if result.ID != 10 {
		t.Errorf(
			"ID = %d, want %d",
			result.ID,
			10,
		)
	}
}

func TestService_GetByID_NotFound(t *testing.T) {
	t.Parallel()

	repo := &fakePurchasableItemRepository{
		item: PurchasableItem{
			ID: 10,
		},
	}

	service := NewService(repo)

	_, err := service.GetByID(
		context.Background(),
		99,
	)

	if !errors.Is(err, ErrPurchasableItemNotFound) {
		t.Errorf(
			"GetByID() error = %v, want %v",
			err,
			ErrPurchasableItemNotFound,
		)
	}
}

func TestService_GetByCode(t *testing.T) {
	t.Parallel()

	repo := &fakePurchasableItemRepository{
		item: PurchasableItem{
			ID:           10,
			ItemCode:     "PRO_PLAN",
			ItemTypeCode: ItemTypePlan,
			Name:         "Pro Plan",
		},
	}

	service := NewService(repo)

	result, err := service.GetByCode(
		context.Background(),
		"PRO_PLAN",
	)

	if err != nil {
		t.Fatalf(
			"GetByCode() error = %v",
			err,
		)
	}

	if result.ItemCode != "PRO_PLAN" {
		t.Errorf(
			"ItemCode = %q, want %q",
			result.ItemCode,
			"PRO_PLAN",
		)
	}
}

func TestService_GetByCode_NotFound(t *testing.T) {
	t.Parallel()

	repo := &fakePurchasableItemRepository{
		item: PurchasableItem{
			ItemCode: "OTHER",
		},
	}

	service := NewService(repo)

	_, err := service.GetByCode(
		context.Background(),
		"PRO_PLAN",
	)

	if !errors.Is(err, ErrPurchasableItemNotFound) {
		t.Errorf(
			"GetByCode() error = %v, want %v",
			err,
			ErrPurchasableItemNotFound,
		)
	}
}

func TestPurchasableItem_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		item    PurchasableItem
		wantErr bool
	}{
		{
			name: "valid",
			item: PurchasableItem{
				ItemCode:     "PRO_PLAN",
				ItemTypeCode: ItemTypePlan,
				Name:         "Pro Plan",
			},
		},
		{
			name: "missing code",
			item: PurchasableItem{
				ItemTypeCode: ItemTypePlan,
				Name:         "Pro Plan",
			},
			wantErr: true,
		},
		{
			name: "missing type",
			item: PurchasableItem{
				ItemCode: "PRO_PLAN",
				Name:     "Pro Plan",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			item: PurchasableItem{
				ItemCode:     "PRO_PLAN",
				ItemTypeCode: ItemTypePlan,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Validate()

			if (err != nil) != tt.wantErr {
				t.Fatalf(
					"Validate() error = %v, wantErr = %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
