package subscription

import "time"

type SubscriptionCreated struct {
	SubscriptionID     int64     `json:"subscription_id"`
	AccountID          int64     `json:"account_id"`
	PlanID             string    `json:"plan_id"`
	PlanVersion        int       `json:"plan_version"`
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
	BillingCycleAnchor int       `json:"billing_cycle_anchor"`
}
