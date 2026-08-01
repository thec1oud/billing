package tariff

import (
	"context"
	"testing"
	 "fmt"

	"github.com/thec1oud/billing/internal/shared/money"
)

type mockRepository struct {
	tariffs map[string]map[int]Tariff
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		tariffs: make(map[string]map[int]Tariff),
	}
}

func (m *mockRepository) Save(_ context.Context, t Tariff) (Tariff, error) {
	if _, exists := m.tariffs[t.TariffCode]; !exists {
		m.tariffs[t.TariffCode] = make(map[int]Tariff)
	}
	t.ID = int64(len(m.tariffs[t.TariffCode]) + 1)
	m.tariffs[t.TariffCode][t.Version] = t
	return t, nil
}

func (m *mockRepository) GetByCodeAndVersion(_ context.Context, code string, version int) (Tariff, error) {
	vMap, exists := m.tariffs[code]
	if !exists {
		return Tariff{}, fmt.Errorf("not found")
	}
	t, exists := vMap[version]
	if !exists {
		return Tariff{}, fmt.Errorf("version not found")
	}
	return t, nil
}

func (m *mockRepository) LatestVersion(_ context.Context, code string) (int, error) {
	vMap, exists := m.tariffs[code]
	if !exists || len(vMap) == 0 {
		return 0, nil
	}
	latest := 0
	for v := range vMap {
		if v > latest {
			latest = v
		}
	}
	return latest, nil
}

// ==========================================
// TARIFF MATH & SERVICE UNIT TESTS
// ==========================================

func TestTariff_CalculateCharge_Tiered(t *testing.T) {
	usd10, _ := money.New(1000, "USD") // $10 flat fee base
	priceTier1, _ := money.New(100, "USD") // $1 per unit minor
	priceTier2, _ := money.New(50, "USD")  // $0.50 per unit minor
	zeroFee, _ := money.New(0, "USD")

	upTo100 := int64(100)

	tieredTariff := Tariff{
		TariffCode:     "API_USAGE",
		Version:        1,
		TariffTypeCode: TariffTypeTiered,
		Amount:         usd10,
		Tiers: []Tier{
			{UpToQuantity: &upTo100, UnitPrice: priceTier1, FlatFee: zeroFee},
			{UpToQuantity: nil, UnitPrice: priceTier2, FlatFee: zeroFee},
		},
	}

	t.Run("calculates usage correctly across tiers without floats", func(t *testing.T) {
		// 150 units -> 100 units @ $1.00 ($100) + 50 units @ $0.50 ($25) = $125 total (12500 cents)
		qty := Quantity{Value: 150, Unit: UnitAPICall}
		charge, err := tieredTariff.CalculateCharge(qty)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if charge.AmountMinor != 12500 {
			t.Errorf("expected 12500 minor units ($125.00), got %d", charge.AmountMinor)
		}
	})
}

func TestService_CreateTariff(t *testing.T) {
	repo := newMockRepository()
	svc := NewService(repo)
	basePrice, _ := money.New(2999, "USD")

	t.Run("creates version 1 tariff", func(t *testing.T) {
		trf, err := svc.CreateTariff(
			context.Background(),
			"BASE_SUBSCRIPTION",
			"Base Subscription Tariff",
			TariffTypeFlatFee,
			basePrice,
			BillingIntervalMonth,
			nil,
		)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if trf.Version != 1 {
			t.Errorf("expected version 1, got %d", trf.Version)
		}
	})
}