package purchasable_item

import (
	"context"
	"errors"
	"testing"
)

type mockRepository struct {
	items map[string]PurchasableItem
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		items: make(map[string]PurchasableItem),
	}
}

func (m *mockRepository) Save(_ context.Context, item PurchasableItem) (PurchasableItem, error) {
	if _, exists := m.items[item.ItemCode]; exists {
		return PurchasableItem{}, errors.New("item code already exists")
	}
	item.ID = int64(len(m.items) + 1)
	m.items[item.ItemCode] = item
	return item, nil
}

func (m *mockRepository) GetByCode(_ context.Context, code string) (PurchasableItem, error) {
	item, exists := m.items[code]
	if !exists {
		return PurchasableItem{}, errors.New("item not found")
	}
	return item, nil
}

func (m *mockRepository) GetByID(_ context.Context, id int64) (PurchasableItem, error) {
	for _, item := range m.items {
		if item.ID == id {
			return item, nil
		}
	}
	return PurchasableItem{}, errors.New("item not found")
}

// ==========================================
// SERVICE UNIT TESTS
// ==========================================

func TestService_CreateItem(t *testing.T) {
	t.Run("successfully creates a purchasable item", func(t *testing.T) {
		repo := newMockRepository()
		svc := NewService(repo)

		desc := "Standard user license"
		item, err := svc.CreateItem(context.Background(), "SEAT_USER", ItemTypeProduct, "User Seat", &desc, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if item.ID != 1 {
			t.Errorf("expected ID 1, got %d", item.ID)
		}
		if item.ItemCode != "SEAT_USER" {
			t.Errorf("expected code SEAT_USER, got %s", item.ItemCode)
		}
	})

	t.Run("returns error when item code is empty", func(t *testing.T) {
		repo := newMockRepository()
		svc := NewService(repo)

		_, err := svc.CreateItem(context.Background(), "", ItemTypeProduct, "User Seat", nil, nil)
		if err == nil {
			t.Fatal("expected error for empty item code, got nil")
		}
	})
}
