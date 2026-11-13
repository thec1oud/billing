package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thec1oud/billing/internal/shared/apperr"
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
	ID        int64         `json:"plan_duration_id"`
	PlanID    int64         `json:"plan_id"`
	TariffID  int64         `json:"tariff_id"`
	Duration  time.Duration `json:"duration"`
	IsActive  bool          `json:"is_active"`
	CreatedAt time.Time     `json:"created_at"`
}

func (p Plan) Validate() error {
	if p.PlanCode == "" {
		return apperr.Validation("required_field", "plan code is required")
	}

	if p.Version < 1 {
		return apperr.Validation("invalid_version", "plan version must be greater than zero")
	}

	if !p.LegacyPricePolicyCode.Valid() {
		return apperr.Validation(
			"invalid_legacy_policy",
			fmt.Sprintf("unsupported legacy price policy %q", p.LegacyPricePolicyCode),
		)
	}

	if p.EffectiveFrom.IsZero() {
		return apperr.Validation("required_field", "effective from is required")
	}

	if p.EffectiveUntil != nil &&
		!p.EffectiveUntil.After(p.EffectiveFrom) {
		return apperr.Validation(
			"invalid_effective_period",
			"effective until must be after effective from",
		)
	}
	return nil
}

func (d PlanDuration) Validate() error {
	if d.PlanID <= 0 {
		return apperr.Validation("invalid_id", "plan id must be greater than zero")
	}

	if d.TariffID <= 0 {
		return apperr.Validation("invalid_id", "tariff id must be greater than zero")
	}

	if d.Duration <= 0 {
		return apperr.Validation("invalid_duration", "duration must be greater than zero")
	}

	return nil
}
