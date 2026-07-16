package subscription

import (
	"time"

	"github.com/thec1oud/billing/internal/substrate/money"
)

// BillingProjection is the read model exposed to the
// Invoice Engine. It contains only the data required
// to generate invoice line items.
type BillingProjection struct {
	// Subscription identity
	SubscriptionID string
	AccountID      string

	// Selected immutable plan
	PlanID      string
	PlanVersion int

	// Pricing
	Amount   money.Money
	Currency string

	// Billing period
	PeriodStart time.Time
	PeriodEnd   time.Time
}
