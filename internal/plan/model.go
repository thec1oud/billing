package plan

import "github.com/thec1oud/billing/internal/shared/money"

type BillingPeriod string

const (
	BillingPeriodMonthly BillingPeriod = "MONTHLY"
)

type Plan struct {
	ID            string
	Version       int
	FlatFeeAmount money.Money
	BillingPeriod BillingPeriod
}
