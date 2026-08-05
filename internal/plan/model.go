package plan

import (
	"encoding/json"
	"time"
)

type LegacyPricePolicy string

const (
	LegacyPolicyKeepForever        LegacyPricePolicy = "KEEP_FOREVER"
	LegacyPolicyMigrateImmediately LegacyPricePolicy = "MIGRATE_IMMEDIATELY"
	LegacyPolicyMigrateOnRenewal   LegacyPricePolicy = "MIGRATE_ON_RENEWAL"
)

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

// PlanDuration represents a billing duration available for a plan.
type PlanDuration struct {
	ID        int64     `json:"plan_duration_id"`
	PlanID    int64     `json:"plan_id"`
	TariffID  int64     `json:"tariff_id"`
	Duration  string    `json:"duration"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}