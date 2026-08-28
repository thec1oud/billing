package subscription

import (
	"time"

	"github.com/thec1oud/billing/internal/shared/money"
)

// BillingProjection is the read model exposed to the
// Invoice Engine. It contains only the data required
// to generate invoice line items.
type BillingProjection struct {
	// Subscription identity
	SubscriptionID int64
	AccountID      int64

	// Selected immutable plan
	PlanID      int64
	PlanVersion int

	// Pricing
	Amount   money.Money
	Currency money.Currency

	// Billing period
	PeriodStart time.Time
	PeriodEnd   time.Time
}
