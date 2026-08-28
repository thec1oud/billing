package model

import (
	"errors"
	"time"
)

type Status string

const StatusActive Status = "ACTIVE"

var ErrNotFound = errors.New("subscription not found")

// Subscription uses database-generated int64 IDs for both subscription and account.
type Subscription struct {
	SubscriptionID     int64
	AccountID          int64
	PlanID             int64
	PlanVersion        int
	Status             Status
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	BillingCycleAnchor time.Time
}

type CreateInput struct {
	AccountID          int64
	PlanID             int64
	PlanVersion        int
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	BillingCycleAnchor time.Time
}
