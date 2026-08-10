package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type LegacyPricePolicy string

const (
	LegacyPolicyKeepForever        LegacyPricePolicy = "KEEP_FOREVER"
	LegacyPolicyMigrateImmediately LegacyPricePolicy = "MIGRATE_IMMEDIATELY"
	LegacyPolicyMigrateOnRenewal   LegacyPricePolicy = "MIGRATE_ON_RENEWAL"
)

func (p LegacyPricePolicy) Valid() bool {
	switch p {
	case LegacyPolicyKeepForever,
		LegacyPolicyMigrateImmediately,
		LegacyPolicyMigrateOnRenewal:
		return true
	default:
		return false
	}
}


// Plan represents an immutable, versioned pricing entity.
type Plan struct {
	ID                    int64             `json:"plan_id"`
	PlanCode              string            `json:"plan_code"`
	Version               int               `json:"version"`
	EffectiveFrom         time.Time         `json:"effective_from"`
	EffectiveUntil        *time.Time        `json:"effective_until,omitempty"`
	LegacyPricePolicyCode LegacyPricePolicy `json:"legacy_price_policy_code"`
	MigrationPath         json.RawMessage   `json:"migration_path"`
	Metadata              json.RawMessage   `json:"metadata"`
	CreatedAt             time.Time         `json:"created_at"`

	Durations []PlanDuration `json:"durations,omitempty"`
}

type PlanDuration struct {
	ID        int64            `json:"plan_duration_id"`
	PlanID    int64            `json:"plan_id"`
	TariffID  int64            `json:"tariff_id"`
	Duration  time.Duration `json:"duration"`
	IsActive  bool             `json:"is_active"`
	CreatedAt time.Time        `json:"created_at"`
}

func (p Plan) Validate() error {
	if p.PlanCode == "" {
		return errors.New("plan code is required")
	}

	if p.Version < 1 {
		return errors.New("plan version must be greater than zero")
	}

	if !p.LegacyPricePolicyCode.Valid() {
		return fmt.Errorf(
			"unsupported legacy price policy %q",
			p.LegacyPricePolicyCode,
		)
	}

	if p.EffectiveUntil != nil &&
		!p.EffectiveUntil.After(p.EffectiveFrom) {
		return errors.New(
			"effective until must be after effective from",
		)
	}

	for i, duration := range p.Durations {
		if err := duration.Validate(); err != nil {
			return fmt.Errorf(
				"invalid duration at index %d: %w",
				i,
				err,
			)
		}
	}

	return nil
}

func (d PlanDuration) Validate() error {
	if d.TariffID <= 0 {
		return errors.New("tariff id must be greater than zero")
	}

	if d.Duration <= 0 {
		return errors.New("duration must be greater than zero")
	}

	return nil
}


