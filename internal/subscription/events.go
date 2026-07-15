package subscription

import (
	"time"

	"github.com/google/uuid"
)

type SubscriptionCreated struct {
	SubscriptionID     uuid.UUID `json:"subscription_id"`
	AccountID          uuid.UUID `json:"account_id"`
	PlanID             string    `json:"plan_id"`
	PlanVersion        int       `json:"plan_version"`
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
	BillingCycleAnchor int       `json:"billing_cycle_anchor"`
}
