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
