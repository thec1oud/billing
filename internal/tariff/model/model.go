package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

func (u QuantityUnit) Valid() bool {
	switch u {
	case UnitCount,
		UnitSeat,
		UnitGigabyte,
		UnitHour,
		UnitAPICall:
		return true
	default:
		return false
	}
}

type Quantity struct {
	Value int64        `json:"value"`
	Unit  QuantityUnit `json:"unit"`
}

func (q Quantity) Validate() error {
	if q.Value < 0 {
		return errors.New("quantity cannot be negative")
	}

	if !q.Unit.Valid() {
		return fmt.Errorf("invalid quantity unit %q", q.Unit)
	}

	return nil
}

// Tier represents one tier in a TIERED_USAGE tariff.
type Tier struct {
	UpToQuantity *int64      `json:"up_to,omitempty"`
	UnitPrice    money.Money `json:"unit_price"`
	FlatFee      money.Money `json:"flat_fee"`
}

type TierStrategy string

const (
	TierStrategyVolume    TierStrategy = "VOLUME"
	TierStrategyGraduated TierStrategy = "GRADUATED"
)

func (t TierStrategy) Valid() bool {
	switch t {
	case TierStrategyVolume,
		TierStrategyGraduated:
		return true
	default:
		return false
	}
}

type Tariff struct {
	ID             int64           `json:"tariff_id"`
	TariffCode     string          `json:"tariff_code"`
	Version        int             `json:"version"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	TariffTypeCode TariffTypeCode  `json:"tariff_type_code"`
	TierStrategy   TierStrategy    `json:"tier_strategy,omitempty"`
	QuantityUnit   QuantityUnit    `json:"quantity_unit,omitempty"`
	Amount         money.Money     `json:"amount"`
	Tiers          []Tier          `json:"tiers,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
	IsActive       bool            `json:"is_active"`
}

func (t Tariff) Validate() error {
	t.TariffCode = strings.TrimSpace(t.TariffCode)
	t.Name = strings.TrimSpace(t.Name)

	if t.TariffCode == "" {
		return errors.New("tariff code is required")
	}

	if t.Version < 0 {
		return errors.New("tariff version cannot be negative")
	}

	if t.Name == "" {
		return errors.New("tariff name is required")
	}

	if !t.TariffTypeCode.Valid() {
		return fmt.Errorf("invalid tariff type %q", t.TariffTypeCode)
	}

	// Money always carries its currency.
	if t.Amount.Currency == "" {
		return errors.New("tariff amount currency is required")
	}

	switch t.TariffTypeCode {
	case TariffTypePerUnit:
		if t.QuantityUnit != "" && !t.QuantityUnit.Valid() {
			return fmt.Errorf(
				"invalid quantity unit %q for per-unit tariff",
				t.QuantityUnit,
			)
		}

	case TariffTypeTieredUsage:
		if len(t.Tiers) == 0 {
			return errors.New("tiered usage tariff requires tiers")
		}

		if !t.TierStrategy.Valid() {
			return fmt.Errorf(
				"invalid tier strategy %q",
				t.TierStrategy,
			)
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
	var previous int64

	for i, tier := range tiers {
		// Every monetary value must carry the same currency
		// as the tariff.
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

		if tier.UpToQuantity == nil {
			// Only the final tier may be unbounded.
			if i != len(tiers)-1 {
				return fmt.Errorf(
					"tier %d is unbounded but is not the final tier",
					i,
				)
			}

			continue
		}

		if *tier.UpToQuantity <= previous {
			return fmt.Errorf(
				"tier %d upper bound must be greater than previous tier",
				i,
			)
		}

		previous = *tier.UpToQuantity
	}

	// Graduated pricing needs an unbounded final tier.
	if tiers[len(tiers)-1].UpToQuantity != nil {
		return errors.New(
			"final tier must be unbounded",
		)
	}

	return nil
}

func (t Tariff) CalculateCharge(qty Quantity) (money.Money, error) {
	if err := qty.Validate(); err != nil {
		return money.Money{}, fmt.Errorf(
			"validate quantity: %w",
			err,
		)
	}

	switch t.TariffTypeCode {
	case TariffTypeFlatFee:
		return t.Amount, nil

	case TariffTypePerUnit:
		if t.QuantityUnit != "" && qty.Unit != t.QuantityUnit {
			return money.Money{}, fmt.Errorf(
				"quantity unit %s does not match tariff unit %s",
				qty.Unit,
				t.QuantityUnit,
			)
		}

		return t.Amount.MultiplyByScalar(qty.Value)

	case TariffTypeTieredUsage:
		switch t.TierStrategy {
		case TierStrategyGraduated, TierStrategyVolume:
			return t.calculateTieredUsageCharge(qty)
		default:
			return money.Money{}, fmt.Errorf(
				"charge calculation for tier strategy %s is not implemented",
				t.TierStrategy,
			)
		}

	default:
		return money.Money{}, fmt.Errorf(
			"charge calculation for tariff type %s is not implemented",
			t.TariffTypeCode,
		)
	}
}

func (t Tariff) calculateTieredUsageCharge(
	qty Quantity,
) (money.Money, error) {
	if len(t.Tiers) == 0 {
		return money.Money{}, errors.New(
			"tiered tariff has no tiers",
		)
	}

	total, err := money.Zero(t.Amount.Currency)
	if err != nil {
		return money.Money{}, fmt.Errorf(
			"initialize charge: %w",
			err,
		)
	}

	remaining := qty.Value
	var previous int64

	for _, tier := range t.Tiers {
		if remaining <= 0 {
			break
		}

		capacity := remaining

		if tier.UpToQuantity != nil {
			capacity = *tier.UpToQuantity - previous

			if capacity <= 0 {
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

		tierCharge, err := tier.UnitPrice.MultiplyByScalar(usage)
		if err != nil {
			return money.Money{}, fmt.Errorf(
				"calculate tier usage charge: %w",
				err,
			)
		}

		tierCharge, err = tierCharge.Add(tier.FlatFee)
		if err != nil {
			return money.Money{}, fmt.Errorf(
				"add tier flat fee: %w",
				err,
			)
		}

		total, err = total.Add(tierCharge)
		if err != nil {
			return money.Money{}, fmt.Errorf(
				"add tier charge: %w",
				err,
			)
		}

		remaining -= usage
	}

	if remaining > 0 {
		return money.Money{}, errors.New(
			"tier configuration does not contain an unbounded final tier",
		)
	}

	return total, nil
}
