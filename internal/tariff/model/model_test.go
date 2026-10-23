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

func TestTieredVolumeCalculateCharge(t *testing.T) {
	amount, err := money.New(0, "USD")
	require.NoError(t, err)

	firstPrice, err := money.New(100, "USD")
	require.NoError(t, err)

	secondPrice, err := money.New(50, "USD")
	require.NoError(t, err)

	limit := int64(10)

	tariff := Tariff{
		TariffCode:     "USAGE",
		Version:        1,
		Name:           "Usage",
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyVolume,
		Amount:         amount,
		Tiers: []Tier{
			{
				UpToQuantity: &limit,
				UnitPrice:    firstPrice,
				FlatFee:      amount,
			},
			{
				UpToQuantity: nil,
				UnitPrice:    secondPrice,
				FlatFee:      amount,
			},
		},
	}

	result, err := tariff.CalculateCharge(Quantity{Value: 15, Unit: UnitCount})
	require.NoError(t, err)
	require.Equal(t, int64(750), result.AmountMinor)
	require.Equal(t, "USD", string(result.Currency))
}

func TestTieredPricingBoundaries(t *testing.T) {
	zeroUSD, err := money.New(0, "USD")
	require.NoError(t, err)

	price100, err := money.New(100, "USD")
	require.NoError(t, err)

	price50, err := money.New(50, "USD")
	require.NoError(t, err)

	limit10 := int64(10)

	baseTiers := []Tier{
		{
			UpToQuantity: &limit10,
			UnitPrice:    price100,
			FlatFee:      zeroUSD,
		},
		{
			UpToQuantity: nil,
			UnitPrice:    price50,
			FlatFee:      zeroUSD,
		},
	}

	tests := []struct {
		name          string
		qty           int64
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
			name:          "within first tier",
			qty:           5,
			wantVolume:    500, // 5 * 100
			wantGraduated: 500, // 5 * 100
		},
		{
			name:          "exactly on first tier boundary",
			qty:           10,
			wantVolume:    1000, // 10 * 100
			wantGraduated: 1000, // 10 * 100
		},
		{
			name:          "one unit into second tier",
			qty:           11,
			wantVolume:    550,  // 11 * 50
			wantGraduated: 1050, // 10 * 100 + 1 * 50
		},
		{
			name:          "unbounded second tier",
			qty:           15,
			wantVolume:    750,  // 15 * 50
			wantGraduated: 1250, // 10 * 100 + 5 * 50
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumeTariff := Tariff{
				TariffCode:     "USAGE_VOL",
				Version:        1,
				Name:           "Usage Volume",
				TariffTypeCode: TariffTypeTieredUsage,
				TierStrategy:   TierStrategyVolume,
				Amount:         zeroUSD,
				Tiers:          baseTiers,
			}
			volRes, err := volumeTariff.CalculateCharge(Quantity{Value: tt.qty, Unit: UnitCount})
			require.NoError(t, err)
			require.Equal(t, tt.wantVolume, volRes.AmountMinor)

			gradTariff := Tariff{
				TariffCode:     "USAGE_GRAD",
				Version:        1,
				Name:           "Usage Graduated",
				TariffTypeCode: TariffTypeTieredUsage,
				TierStrategy:   TierStrategyGraduated,
				Amount:         zeroUSD,
				Tiers:          baseTiers,
			}
			gradRes, err := gradTariff.CalculateCharge(Quantity{Value: tt.qty, Unit: UnitCount})
			require.NoError(t, err)
			require.Equal(t, tt.wantGraduated, gradRes.AmountMinor)
		})
	}
}

func TestTieredPricingWithFlatFees(t *testing.T) {
	zeroUSD, err := money.New(0, "USD")
	require.NoError(t, err)

	price100, err := money.New(100, "USD")
	require.NoError(t, err)

	fee200, err := money.New(200, "USD")
	require.NoError(t, err)

	price50, err := money.New(50, "USD")
	require.NoError(t, err)

	fee300, err := money.New(300, "USD")
	require.NoError(t, err)

	limit10 := int64(10)

	tiers := []Tier{
		{
			UpToQuantity: &limit10,
			UnitPrice:    price100,
			FlatFee:      fee200,
		},
		{
			UpToQuantity: nil,
			UnitPrice:    price50,
			FlatFee:      fee300,
		},
	}

	volumeTariff := Tariff{
		TariffCode:     "USAGE_VOL",
		Version:        1,
		Name:           "Usage Volume",
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyVolume,
		Amount:         zeroUSD,
		Tiers:          tiers,
	}

	gradTariff := Tariff{
		TariffCode:     "USAGE_GRAD",
		Version:        1,
		Name:           "Usage Graduated",
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyGraduated,
		Amount:         zeroUSD,
		Tiers:          tiers,
	}

	// 0 units: volume charges tier 1 flat fee; graduated doesn't enter any tier
	volRes, err := volumeTariff.CalculateCharge(Quantity{Value: 0, Unit: UnitCount})
	require.NoError(t, err)
	require.Equal(t, int64(200), volRes.AmountMinor)

	gradRes, err := gradTariff.CalculateCharge(Quantity{Value: 0, Unit: UnitCount})
	require.NoError(t, err)
	require.Equal(t, int64(0), gradRes.AmountMinor)

	// 10 units (boundary): both pay tier 1 unit price + tier 1 flat fee
	volRes, err = volumeTariff.CalculateCharge(Quantity{Value: 10, Unit: UnitCount})
	require.NoError(t, err)
	require.Equal(t, int64(1200), volRes.AmountMinor) // 10 * 100 + 200

	gradRes, err = gradTariff.CalculateCharge(Quantity{Value: 10, Unit: UnitCount})
	require.NoError(t, err)
	require.Equal(t, int64(1200), gradRes.AmountMinor) // 10 * 100 + 200

	// 15 units (unbounded):
	// volume pays tier 2 price + tier 2 flat fee: 15 * 50 + 300 = 1050
	volRes, err = volumeTariff.CalculateCharge(Quantity{Value: 15, Unit: UnitCount})
	require.NoError(t, err)
	require.Equal(t, int64(1050), volRes.AmountMinor)

	// graduated pays tier 1 (10 * 100 + 200 = 1200) + tier 2 (5 * 50 + 300 = 550) = 1750
	gradRes, err = gradTariff.CalculateCharge(Quantity{Value: 15, Unit: UnitCount})
	require.NoError(t, err)
	require.Equal(t, int64(1750), gradRes.AmountMinor)
}

func TestTieredVolumeValidationAndErrors(t *testing.T) {
	zeroUSD, err := money.New(0, "USD")
	require.NoError(t, err)

	price100, err := money.New(100, "USD")
	require.NoError(t, err)

	// No tiers
	tariffNoTiers := Tariff{
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyVolume,
		Amount:         zeroUSD,
		Tiers:          nil,
	}
	_, err = tariffNoTiers.CalculateCharge(Quantity{Value: 5, Unit: UnitCount})
	require.Error(t, err)
	require.Contains(t, err.Error(), "tiered tariff has no tiers")

	// Missing unbounded tier
	limit10 := int64(10)
	tariffBoundedOnly := Tariff{
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyVolume,
		Amount:         zeroUSD,
		Tiers: []Tier{
			{
				UpToQuantity: &limit10,
				UnitPrice:    price100,
				FlatFee:      zeroUSD,
			},
		},
	}
	_, err = tariffBoundedOnly.CalculateCharge(Quantity{Value: 15, Unit: UnitCount})
	require.Error(t, err)
	require.Contains(t, err.Error(), "tier configuration does not contain an unbounded final tier")

	// Invalid tier boundaries (non-increasing) are rejected by Validate()
	limit5 := int64(5)
	tariffInvalidBounds := Tariff{
		TariffCode:     "USAGE",
		Version:        1,
		Name:           "Usage",
		TariffTypeCode: TariffTypeTieredUsage,
		TierStrategy:   TierStrategyVolume,
		Amount:         zeroUSD,
		Tiers: []Tier{
			{
				UpToQuantity: &limit10,
				UnitPrice:    price100,
				FlatFee:      zeroUSD,
			},
			{
				UpToQuantity: &limit5,
				UnitPrice:    price100,
				FlatFee:      zeroUSD,
			},
		},
	}
	err = tariffInvalidBounds.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "upper bound must be greater than previous tier")
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
