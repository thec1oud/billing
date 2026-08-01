package plan

import (
	"encoding/json"
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

type BillingInterval string

const (
	BillingIntervalDay   BillingInterval = "DAY"
	BillingIntervalWeek  BillingInterval = "WEEK"
	BillingIntervalMonth BillingInterval = "MONTH"
	BillingIntervalYear  BillingInterval = "YEAR"
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
	TariffID              int64             `json:"tariff_id"`
	EffectiveFrom         time.Time         `json:"effective_from"`          // Inclusive
	EffectiveUntil        *time.Time        `json:"effective_until,omitempty"` // Exclusive
	LegacyPricePolicyCode LegacyPricePolicy `json:"legacy_price_policy_code"`
	MigrationPath         json.RawMessage   `json:"migration_path"`
	Metadata              json.RawMessage   `json:"metadata"`
	CreatedAt             time.Time         `json:"created_at"`

	// Derived fields from linked Tariff
	FlatFeeAmount money.Money     `json:"flat_fee_amount"`
	Interval      BillingInterval `json:"billing_interval"`
}