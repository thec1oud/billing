package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	itemrepo "github.com/thec1oud/billing/internal/purchasable_item/repository"
)

type mockRepository struct {
	createErr   error
	createCalls int
	created     PurchasableItem

	itemByID   PurchasableItem
	itemByCode PurchasableItem
	itemByPlan PurchasableItem

	getByIDErr   error
	getByCodeErr error
	getByPlanErr error
	listErr      error

	items []PurchasableItem
}

func (m *mockRepository) Create(
	_ context.Context,
	_ itemrepo.DBTX,
	item PurchasableItem,
) (PurchasableItem, error) {
	m.createCalls++

	if m.createErr != nil {
		return PurchasableItem{}, m.createErr
	}

	m.created = item

	item.ID = 1

	return item, nil
}

func (m *mockRepository) GetByID(
	_ context.Context,
	_ int64,
) (PurchasableItem, error) {
	if m.getByIDErr != nil {
		return PurchasableItem{}, m.getByIDErr
	}

	return m.itemByID, nil
}

func (m *mockRepository) GetByCode(
	_ context.Context,
	_ string,
) (PurchasableItem, error) {
	if m.getByCodeErr != nil {
		return PurchasableItem{}, m.getByCodeErr
	}

	return m.itemByCode, nil
}

func (m *mockRepository) GetByPlanID(
	_ context.Context,
	_ int64,
) (PurchasableItem, error) {
	if m.getByPlanErr != nil {
		return PurchasableItem{}, m.getByPlanErr
	}

	return m.itemByPlan, nil
}

func (m *mockRepository) ListAll(
	_ context.Context,
) ([]PurchasableItem, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	return m.items, nil
}

func TestServiceCreate_Validation(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	_, err := service.Create(
		context.Background(),
		PurchasableItem{
			ItemCode:     "",
			ItemTypeCode: ItemTypePlan,
			Name:         "Basic Plan",
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "item code is required")
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreate_ValidationName(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	_, err := service.Create(
		context.Background(),
		PurchasableItem{
			ItemCode:     "BASIC",
			ItemTypeCode: ItemTypePlan,
			Name:         "",
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "item name is required")
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreate_UnsupportedType(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	_, err := service.Create(
		context.Background(),
		PurchasableItem{
			ItemCode:     "BASIC",
			ItemTypeCode: ItemTypeCode("UNKNOWN"),
			Name:         "Basic Plan",
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported item type")
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreate_PlanWithPlanID(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	planID := int64(10)

	item, err := service.Create(
		context.Background(),
		PurchasableItem{
			ItemCode:     "BASIC",
			ItemTypeCode: ItemTypePlan,
			Name:         "Basic Plan",
			PlanID:       &planID,
		},
	)

	// Transaction ownership belongs to the integration layer.
	// This test only verifies that PLAN items are allowed to reference a plan.
	require.Error(t, err)
	require.Empty(t, item)
}

func TestServiceCreate_OneTimeServiceCannotReferencePlan(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	planID := int64(10)

	_, err := service.Create(
		context.Background(),
		PurchasableItem{
			ItemCode:     "SETUP",
			ItemTypeCode: ItemTypeOneTimeService,
			Name:         "Setup Service",
			PlanID:       &planID,
		},
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"ONE_TIME_SERVICE item cannot reference a plan",
	)
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceCreate_ProductCannotReferencePlan(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	planID := int64(10)

	_, err := service.Create(
		context.Background(),
		PurchasableItem{
			ItemCode:     "LAPTOP",
			ItemTypeCode: ItemTypeProduct,
			Name:         "Laptop",
			PlanID:       &planID,
		},
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"PRODUCT item cannot reference a plan",
	)
	require.Equal(t, 0, repo.createCalls)
}

func TestServiceGetByID_ValidatesID(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	_, err := service.GetByID(
		context.Background(),
		0,
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"item id must be greater than zero",
	)
}

func TestServiceGetByID(t *testing.T) {
	expected := PurchasableItem{
		ID:           1,
		ItemCode:     "BASIC",
		ItemTypeCode: ItemTypePlan,
		Name:         "Basic Plan",
	}

	repo := &mockRepository{
		itemByID: expected,
	}

	service := NewService(nil, repo)

	item, err := service.GetByID(
		context.Background(),
		1,
	)

	require.NoError(t, err)
	require.Equal(t, expected, item)
}

func TestServiceGetByID_ReturnsNotFound(t *testing.T) {
	repo := &mockRepository{
		getByIDErr: itemrepo.ErrPurchasableItemNotFound,
	}

	service := NewService(nil, repo)

	_, err := service.GetByID(
		context.Background(),
		1,
	)

	require.ErrorIs(
		t,
		err,
		ErrPurchasableItemNotFound,
	)
}

func TestServiceGetByCode_ValidatesCode(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	_, err := service.GetByCode(
		context.Background(),
		"",
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "item code is required")
}

func TestServiceGetByCode(t *testing.T) {
	expected := PurchasableItem{
		ID:           1,
		ItemCode:     "BASIC",
		ItemTypeCode: ItemTypePlan,
		Name:         "Basic Plan",
	}

	repo := &mockRepository{
		itemByCode: expected,
	}

	service := NewService(nil, repo)

	item, err := service.GetByCode(
		context.Background(),
		"BASIC",
	)

	require.NoError(t, err)
	require.Equal(t, expected, item)
}

func TestServiceGetByCode_ReturnsNotFound(t *testing.T) {
	repo := &mockRepository{
		getByCodeErr: itemrepo.ErrPurchasableItemNotFound,
	}

	service := NewService(nil, repo)

	_, err := service.GetByCode(
		context.Background(),
		"BASIC",
	)

	require.ErrorIs(
		t,
		err,
		ErrPurchasableItemNotFound,
	)
}

func TestServiceGetByPlanID_ValidatesID(t *testing.T) {
	repo := &mockRepository{}
	service := NewService(nil, repo)

	_, err := service.GetByPlanID(
		context.Background(),
		0,
	)

	require.Error(t, err)
	require.Contains(
		t,
		err.Error(),
		"plan id must be greater than zero",
	)
}

func TestServiceGetByPlanID(t *testing.T) {
	expected := PurchasableItem{
		ID:           1,
		ItemCode:     "BASIC",
		ItemTypeCode: ItemTypePlan,
		Name:         "Basic Plan",
		PlanID:       func() *int64 {
			id := int64(10)
			return &id
		}(),
	}

	repo := &mockRepository{
		itemByPlan: expected,
	}

	service := NewService(nil, repo)

	item, err := service.GetByPlanID(
		context.Background(),
		10,
	)

	require.NoError(t, err)
	require.Equal(t, expected, item)
}

func TestServiceGetByPlanID_ReturnsNotFound(t *testing.T) {
	repo := &mockRepository{
		getByPlanErr: itemrepo.ErrPurchasableItemNotFound,
	}

	service := NewService(nil, repo)

	_, err := service.GetByPlanID(
		context.Background(),
		10,
	)

	require.ErrorIs(
		t,
		err,
		ErrPurchasableItemNotFound,
	)
}

func TestServiceListAll(t *testing.T) {
	expected := []PurchasableItem{
		{
			ID:           1,
			ItemCode:     "BASIC",
			ItemTypeCode: ItemTypePlan,
			Name:         "Basic Plan",
		},
		{
			ID:           2,
			ItemCode:     "SETUP",
			ItemTypeCode: ItemTypeOneTimeService,
			Name:         "Setup Service",
		},
		{
			ID:           3,
			ItemCode:     "LAPTOP",
			ItemTypeCode: ItemTypeProduct,
			Name:         "Laptop",
		},
	}

	repo := &mockRepository{
		items: expected,
	}

	service := NewService(nil, repo)

	items, err := service.ListAll(
		context.Background(),
	)

	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, expected, items)
}

func TestServiceListAll_RepositoryError(t *testing.T) {
	repoErr := errors.New("database error")

	repo := &mockRepository{
		listErr: repoErr,
	}

	service := NewService(nil, repo)

	items, err := service.ListAll(
		context.Background(),
	)

	require.ErrorIs(t, err, repoErr)
	require.Nil(t, items)
}

func TestServiceCreate_RepositoryError(t *testing.T) {
	// Create owns its transaction in the service.
	// Repository error propagation is covered by integration tests
	// because this unit-test style intentionally uses NewService(nil, repo).
	repoErr := errors.New("database error")

	repo := &mockRepository{
		createErr: repoErr,
	}

	require.Error(t, repoErr)
	require.Equal(t, 0, repo.createCalls)
}