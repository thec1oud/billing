package tariff

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type TariffTypeCode string

const (
	TariffTypeFlatFee TariffTypeCode = "FLAT_FEE"
	TariffTypeTiered  TariffTypeCode = "TIERED"
	TariffTypeVolume  TariffTypeCode = "VOLUME"
)

type BillingInterval string

const (
	BillingIntervalDay   BillingInterval = "DAY"
	BillingIntervalWeek  BillingInterval = "WEEK"
	BillingIntervalMonth BillingInterval = "MONTH"
	BillingIntervalYear  BillingInterval = "YEAR"
)

type QuantityUnit string

const (
	UnitCount    QuantityUnit = "COUNT"
	UnitSeat     QuantityUnit = "SEAT"
	UnitGigabyte QuantityUnit = "GIGABYTE"
	UnitHour     QuantityUnit = "HOUR"
	UnitAPICall  QuantityUnit = "API_CALL"
)

// Quantity represents a composite discrete measurement.
type Quantity struct {
	Value int64        `json:"value"` // Stored in smallest unit/count
	Unit  QuantityUnit `json:"unit"`
}

// Tier defines threshold pricing boundaries for TIERED or VOLUME tariffs.
type Tier struct {
	UpToQuantity *int64      `json:"up_to_quantity,omitempty"` // nil means infinite/unbounded
	UnitPrice    money.Money `json:"unit_price"`
	FlatFee      money.Money `json:"flat_fee"`
}

// Tariff represents an immutable pricing definition row in `tariffs`.
type Tariff struct {
	ID                  int64           `json:"tariff_id"`
	TariffCode          string          `json:"tariff_code"`
	Version             int             `json:"version"`
	Name                string          `json:"name"`
	TariffTypeCode      TariffTypeCode  `json:"tariff_type_code"`
	Amount              money.Money     `json:"amount"` // Base rate / flat rate minor units
	BillingIntervalCode BillingInterval `json:"billing_interval_code"`
	IsActive            bool            `json:"is_active"`
	Tiers               []Tier          `json:"tiers,omitempty"`
	Metadata            json.RawMessage `json:"metadata"`
	CreatedAt           time.Time       `json:"created_at"`
}

// CalculateCharge calculates cost for a given usage quantity without floating-point math.
func (t *Tariff) CalculateCharge(qty Quantity) (money.Money, error) {
	if t.TariffTypeCode == TariffTypeFlatFee {
		return t.Amount, nil
	}

	if len(t.Tiers) == 0 {
		return money.Money{}, fmt.Errorf(
			"no pricing tiers defined for tariff %s v%d",
			t.TariffCode,
			t.Version)
	}

	var totalAmount int64
	remainingUsage := qty.Value

	switch t.TariffTypeCode {
	case TariffTypeTiered:
		var previousThreshold int64 = 0

		for _, tier := range t.Tiers {
			if remainingUsage <= 0 {
				break
			}

			if tier.UnitPrice.Currency != t.Amount.Currency {
				return money.Money{}, fmt.Errorf(
					"currency mismatch between tier (%s) and base tariff (%s)",
					tier.UnitPrice.Currency,
					t.Amount.Currency)
			}

			var tierCapacity int64
			if tier.UpToQuantity != nil {
				tierCapacity = *tier.UpToQuantity - previousThreshold
				previousThreshold = *tier.UpToQuantity
			} else {
				tierCapacity = remainingUsage // Last unbounded tier takes all remaining
			}

			usageInTier := remainingUsage
			if usageInTier > tierCapacity {
				usageInTier = tierCapacity
			}

			// Math using minor units directly
			totalAmount += tier.FlatFee.AmountMinor
			totalAmount += usageInTier * tier.UnitPrice.AmountMinor

			remainingUsage -= usageInTier
		}

	case TariffTypeVolume:
		// Entire quantity priced based on the highest qualifying tier
		var selectedTier *Tier
		for i := range t.Tiers {
			tier := &t.Tiers[i]
			if tier.UpToQuantity == nil || qty.Value <= *tier.UpToQuantity {
				selectedTier = tier
				break
			}
		}

		if selectedTier == nil {
			selectedTier = &t.Tiers[len(t.Tiers)-1]
		}

		if selectedTier.UnitPrice.Currency != t.Amount.Currency {
			return money.Money{}, fmt.Errorf("currency mismatch in volume tier")
		}

		totalAmount = selectedTier.FlatFee.AmountMinor + (qty.Value * selectedTier.UnitPrice.AmountMinor)
	}

	return money.New(totalAmount, string(t.Amount.Currency))
}
