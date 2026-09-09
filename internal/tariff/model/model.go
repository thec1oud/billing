package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type TariffTypeCode string

const (
	TariffTypeFlatFee     TariffTypeCode = "FLAT_FEE"
	TariffTypePerUnit     TariffTypeCode = "PER_UNIT"
	TariffTypeTieredUsage TariffTypeCode = "TIERED_USAGE"
	TariffTypeStairstep   TariffTypeCode = "STAIRSTEP"
	TariffTypePackage     TariffTypeCode = "PACKAGE"
	TariffTypeMatrix      TariffTypeCode = "MATRIX"
	TariffTypeComposite   TariffTypeCode = "COMPOSITE"
)

func (t TariffTypeCode) Valid() bool {
	switch t {
	case TariffTypeFlatFee,
		TariffTypePerUnit,
		TariffTypeTieredUsage,
		TariffTypeStairstep,
		TariffTypePackage,
		TariffTypeMatrix,
		TariffTypeComposite:
		return true
	default:
		return false
	}
}

type QuantityUnit string

const (
	UnitCount    QuantityUnit = "COUNT"
	UnitSeat     QuantityUnit = "SEAT"
	UnitGigabyte QuantityUnit = "GIGABYTE"
	UnitHour     QuantityUnit = "HOUR"
	UnitAPICall  QuantityUnit = "API_CALL"
)

type Quantity struct {
	Value int64        `json:"value"`
	Unit  QuantityUnit `json:"unit"`
}

// Tier represents one tier in a TIERED_USAGE tariff.

type Tier struct {
	UpToQuantity *int64      `json:"up_to,omitempty"`
	UnitPrice    money.Money `json:"unit_price"`
	FlatFee      money.Money `json:"flat_fee"`
}

// TierStrategy defines how tiered pricing is evaluated.
type TierStrategy string

const (
	TierStrategyVolume    TierStrategy = "VOLUME"
	TierStrategyGraduated TierStrategy = "GRADUATED"
)

func (t TierStrategy) Valid() bool {
	switch t {
	case TierStrategyVolume, TierStrategyGraduated:
		return true
	default:
		return false
	}
}

// Tariffs are versioned. Once a tariff has been used in billing,
// changes should result in a new tariff version rather than mutation.
type Tariff struct {
	ID             int64           `json:"tariff_id"`
	TariffCode     string          `json:"tariff_code"`
	Version        int             `json:"version"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	TariffTypeCode TariffTypeCode  `json:"tariff_type_code"`
	TierStrategy   TierStrategy    `json:"tier_strategy,omitempty"`
	Amount         money.Money     `json:"amount"`
	Tiers          []Tier          `json:"tiers,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
	IsActive       bool            `json:"is_active"`
}

func (t Tariff) Validate() error {
	if t.TariffCode == "" {
		return errors.New("tariff code is required")
	}

	if t.Version < 1 {
		return errors.New("tariff version must be greater than zero")
	}

	if t.Name == "" {
		return errors.New("tariff name is required")
	}

	if !t.TariffTypeCode.Valid() {
		return fmt.Errorf(
			"unsupported tariff type %q",
			t.TariffTypeCode,
		)
	}

	if t.Amount.Currency == "" {
		return errors.New("tariff amount currency is required")
	}

	switch t.TariffTypeCode {
	case TariffTypeTieredUsage:
		if len(t.Tiers) == 0 {
			return errors.New("tiered usage tariff requires tiers")
		}
		if !t.TierStrategy.Valid() {
			return fmt.Errorf("unsupported tier strategy %q", t.TierStrategy)
		}
		if err := validateTiers(t.Tiers, t.Amount.Currency); err != nil {
			return err
		}
	}

	return nil
}

func validateTiers(
	tiers []Tier,
	currency money.Currency,
) error {
	var previous *int64

	for i, tier := range tiers {
		if tier.UnitPrice.Currency != currency {
			return fmt.Errorf(
				"tier %d unit price currency %s does not match tariff currency %s",
				i,
				tier.UnitPrice.Currency,
				currency,
			)
		}

		if tier.FlatFee.Currency != currency {
			return fmt.Errorf(
				"tier %d flat fee currency %s does not match tariff currency %s",
				i,
				tier.FlatFee.Currency,
				currency,
			)
		}

		if tier.UpToQuantity != nil {
			if *tier.UpToQuantity <= 0 {
				return fmt.Errorf(
					"tier %d up_to must be greater than zero",
					i,
				)
			}

			if previous != nil && *tier.UpToQuantity <= *previous {
				return fmt.Errorf(
					"tier %d up_to must be greater than previous tier",
					i,
				)
			}

			previous = tier.UpToQuantity
		}
	}

	return nil
}

// CalculateCharge supports the tariff pricing models represented directly by
// the current domain model. For tiered charging, the current implementation
// models the graduated pricing behavior only.
func (t Tariff) CalculateCharge(qty Quantity) (money.Money, error) {
	switch t.TariffTypeCode {
	case TariffTypeFlatFee:
		return t.Amount, nil

	case TariffTypePerUnit:
		return money.New(
			t.Amount.AmountMinor*qty.Value,
			string(t.Amount.Currency),
		)

	case TariffTypeTieredUsage:
		if t.TierStrategy != TierStrategyGraduated {
			return money.Money{}, fmt.Errorf(
				"charge calculation for tier strategy %s is not implemented",
				t.TierStrategy,
			)
		}
		return t.calculateGraduatedCharge(qty)

	default:
		return money.Money{}, fmt.Errorf(
			"charge calculation for tariff type %s is not implemented",
			t.TariffTypeCode,
		)
	}
}

func (t Tariff) calculateGraduatedCharge(
	qty Quantity,
) (money.Money, error) {
	if len(t.Tiers) == 0 {
		return money.Money{}, errors.New(
			"tiered tariff has no tiers",
		)
	}

	var total int64
	remaining := qty.Value
	var previous int64

	for _, tier := range t.Tiers {
		if remaining <= 0 {
			break
		}

		capacity := remaining

		if tier.UpToQuantity != nil {
			capacity = *tier.UpToQuantity - previous

			if capacity < 0 {
				return money.Money{}, errors.New(
					"invalid tier boundaries",
				)
			}

			previous = *tier.UpToQuantity
		}

		usage := remaining
		if usage > capacity {
			usage = capacity
		}

		total += tier.FlatFee.AmountMinor
		total += usage * tier.UnitPrice.AmountMinor

		remaining -= usage
	}

	if remaining > 0 {
		last := t.Tiers[len(t.Tiers)-1]

		if last.UpToQuantity != nil {
			return money.Money{}, errors.New(
				"tier configuration does not contain an unbounded final tier",
			)
		}
	}

	return money.New(
		total,
		string(t.Amount.Currency),
	)
}
