package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thec1oud/billing/internal/shared/money"
)

func TestTariffValidate(t *testing.T) {
	amount, err := money.New(1000, "USD")
	require.NoError(t, err)

	tests := []struct {
		name      string
		tariff    Tariff
		wantError bool
	}{
		{
			name: "valid flat fee",
			tariff: Tariff{
				TariffCode:     "BASIC",
				Version:        1,
				Name:           "Basic",
				TariffTypeCode: TariffTypeFlatFee,
				Amount:         amount,
			},
		},
		{
			name: "missing code",
			tariff: Tariff{
				Version:        1,
				Name:           "Basic",
				TariffTypeCode: TariffTypeFlatFee,
				Amount:         amount,
			},
			wantError: true,
		},
		{
			name: "invalid type",
			tariff: Tariff{
				TariffCode:     "BASIC",
				Version:        1,
				Name:           "Basic",
				TariffTypeCode: "INVALID",
				Amount:         amount,
			},
			wantError: true,
		},
		{
			name: "tiered without tiers",
			tariff: Tariff{
				TariffCode:     "USAGE",
				Version:        1,
				Name:           "Usage",
				TariffTypeCode: TariffTypeTieredUsage,
				TierStrategy:   TierStrategyGraduated,
				Amount:         amount,
			},
			wantError: true,
		},
		{
			name: "tiered requires valid strategy",
			tariff: Tariff{
				TariffCode:     "USAGE",
				Version:        1,
				Name:           "Usage",
				TariffTypeCode: TariffTypeTieredUsage,
				Amount:         amount,
				Tiers: []Tier{{
					UpToQuantity: nil,
					UnitPrice:    amount,
					FlatFee:      amount,
				}},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tariff.Validate()

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestFlatFeeCalculateCharge(t *testing.T) {
	amount, err := money.New(1500, "USD")
	require.NoError(t, err)

	tariff := Tariff{
		TariffCode:     "BASIC",
		Version:        1,
		Name:           "Basic",
		TariffTypeCode: TariffTypeFlatFee,
		Amount:         amount,
	}

	result, err := tariff.CalculateCharge(Quantity{
		Value: 10,
		Unit:  UnitSeat,
	})

	require.NoError(t, err)
	require.Equal(t, int64(1500), result.AmountMinor)
	require.Equal(t, "USD", string(result.Currency))
}

func TestPerUnitCalculateCharge(t *testing.T) {
	amount, err := money.New(250, "USD")
	require.NoError(t, err)

	tariff := Tariff{
		TariffCode:     "API",
		Version:        1,
		Name:           "API Usage",
		TariffTypeCode: TariffTypePerUnit,
		Amount:         amount,
	}

	result, err := tariff.CalculateCharge(Quantity{
		Value: 10,
		Unit:  UnitAPICall,
	})

	require.NoError(t, err)
	require.Equal(t, int64(2500), result.AmountMinor)
	require.Equal(t, "USD", string(result.Currency))
}

func TestTieredGraduatedCalculateCharge(t *testing.T) {
	amount, err := money.New(0, "USD")
	require.NoError(t, err)

	unitPrice1, err := money.New(100, "USD")
	require.NoError(t, err)

	flatFee1, err := money.New(0, "USD")
	require.NoError(t, err)

	unitPrice2, err := money.New(50, "USD")
	require.NoError(t, err)

	flatFee2, err := money.New(0, "USD")
	require.NoError(t, err)

	firstLimit := int64(10)

	tariff := Tariff{
		TariffCode:     "USAGE",
		Version:        1,
		Name:           "Usage",
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyGraduated,
		Amount:         amount,
		Tiers: []Tier{
			{
				UpToQuantity: &firstLimit,
				UnitPrice:    unitPrice1,
				FlatFee:      flatFee1,
			},
			{
				UpToQuantity: nil,
				UnitPrice:    unitPrice2,
				FlatFee:      flatFee2,
			},
		},
	}

	result, err := tariff.CalculateCharge(Quantity{
		Value: 15,
		Unit:  UnitCount,
	})

	require.NoError(t, err)

	// First 10 × 100 = 1000
	// Next 5 × 50  = 250
	// Total         = 1250
	require.Equal(t, int64(1250), result.AmountMinor)
	require.Equal(t, "USD", string(result.Currency))
}

func newTieredTariff(
	t *testing.T,
	strategy TierStrategy,
	flatFee1, flatFee2 int64,
) Tariff {
	t.Helper()

	amount, err := money.New(0, "USD")
	require.NoError(t, err)

	unitPrice1, err := money.New(100, "USD")
	require.NoError(t, err)

	unitPrice2, err := money.New(50, "USD")
	require.NoError(t, err)

	fee1, err := money.New(flatFee1, "USD")
	require.NoError(t, err)

	fee2, err := money.New(flatFee2, "USD")
	require.NoError(t, err)

	limit := int64(10)

	return Tariff{
		TariffCode:     "USAGE",
		Version:        1,
		Name:           "Usage",
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   strategy,
		Amount:         amount,
		Tiers: []Tier{
			{
				UpToQuantity: &limit,
				UnitPrice:    unitPrice1,
				FlatFee:      fee1,
			},
			{
				UpToQuantity: nil,
				UnitPrice:    unitPrice2,
				FlatFee:      fee2,
			},
		},
	}
}

// Tiers: "<=10 @ 100" and "rest @ 50".
// Volume prices all units at the tier the total reaches;
// graduated prices each unit at the tier it falls into.
func TestTieredCalculateChargeStrategies(t *testing.T) {
	tests := []struct {
		name          string
		qty           int64
		flatFee1      int64
		flatFee2      int64
		wantVolume    int64
		wantGraduated int64
	}{
		{
			name:          "zero units",
			qty:           0,
			wantVolume:    0,
			wantGraduated: 0,
		},
		{
			name:          "inside first tier",
			qty:           5,
			wantVolume:    500, // 5 × 100
			wantGraduated: 500, // 5 × 100
		},
		{
			name:          "exactly on tier limit",
			qty:           10,
			wantVolume:    1000, // 10 × 100, still first tier
			wantGraduated: 1000, // 10 × 100
		},
		{
			name:          "one unit past tier limit",
			qty:           11,
			wantVolume:    550,  // 11 × 50
			wantGraduated: 1050, // 10 × 100 + 1 × 50
		},
		{
			name:          "unbounded tier",
			qty:           15,
			wantVolume:    750,  // 15 × 50
			wantGraduated: 1250, // 10 × 100 + 5 × 50
		},
		{
			name:          "large quantity in unbounded tier",
			qty:           1000,
			wantVolume:    50000, // 1000 × 50
			wantGraduated: 50500, // 10 × 100 + 990 × 50
		},
		{
			name:          "flat fee of the reached tier only (volume)",
			qty:           15,
			flatFee1:      30,
			flatFee2:      7,
			wantVolume:    757,  // 15 × 50 + 7
			wantGraduated: 1287, // (10 × 100 + 30) + (5 × 50 + 7)
		},
		{
			name:          "flat fee on first tier at the limit",
			qty:           10,
			flatFee1:      30,
			flatFee2:      7,
			wantVolume:    1030, // 10 × 100 + 30
			wantGraduated: 1030, // 10 × 100 + 30
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qty := Quantity{Value: tt.qty, Unit: UnitCount}

			volume := newTieredTariff(
				t, TierStrategyVolume, tt.flatFee1, tt.flatFee2,
			)
			result, err := volume.CalculateCharge(qty)
			require.NoError(t, err)
			require.Equal(t, tt.wantVolume, result.AmountMinor)
			require.Equal(t, "USD", string(result.Currency))

			graduated := newTieredTariff(
				t, TierStrategyGraduated, tt.flatFee1, tt.flatFee2,
			)
			result, err = graduated.CalculateCharge(qty)
			require.NoError(t, err)
			require.Equal(t, tt.wantGraduated, result.AmountMinor)
			require.Equal(t, "USD", string(result.Currency))
		})
	}
}

func TestTieredVolumeCalculateChargeErrors(t *testing.T) {
	t.Run("no tiers", func(t *testing.T) {
		tariff := newTieredTariff(t, TierStrategyVolume, 0, 0)
		tariff.Tiers = nil

		_, err := tariff.CalculateCharge(Quantity{Value: 5, Unit: UnitCount})
		require.Error(t, err)
	})

	t.Run("quantity beyond last bounded tier", func(t *testing.T) {
		tariff := newTieredTariff(t, TierStrategyVolume, 0, 0)
		tariff.Tiers = tariff.Tiers[:1]

		_, err := tariff.CalculateCharge(Quantity{Value: 11, Unit: UnitCount})
		require.Error(t, err)
	})
}

func TestTieredCurrencyMismatch(t *testing.T) {
	amount, err := money.New(0, "USD")
	require.NoError(t, err)

	unitPrice, err := money.New(100, "EUR")
	require.NoError(t, err)

	flatFee, err := money.New(0, "USD")
	require.NoError(t, err)

	limit := int64(10)

	tariff := Tariff{
		TariffCode:     "USAGE",
		Version:        1,
		Name:           "Usage",
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyGraduated,
		Amount:         amount,
		Tiers: []Tier{
			{
				UpToQuantity: &limit,
				UnitPrice:    unitPrice,
				FlatFee:      flatFee,
			},
		},
	}

	require.Error(t, tariff.Validate())
}