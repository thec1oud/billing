package model

import (
	"errors"
	"time"
)

type Status string

const (
	StatusTrialing Status = "TRIALING"
	StatusActive   Status = "ACTIVE"
	StatusPaused   Status = "PAUSED"
	StatusPastDue  Status = "PAST_DUE"
	StatusCanceled Status = "CANCELED"
	StatusUnpaid   Status = "UNPAID"
)

var (
	ErrNotFound               = errors.New("subscription not found")
	ErrInvalidStateTransition = errors.New("invalid subscription state transition")
)

// Subscription is the database-backed billing agreement and its current period.
type Subscription struct {
	SubscriptionID     int64
	AccountID          int64
	PlanID             int64
	PlanVersion        int
	Status             Status
	Quantity           int32
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	BillingCycleAnchor time.Time
	CancelAtPeriodEnd  bool
	CanceledAt         *time.Time
	EndedAt            *time.Time
	PausedAt           *time.Time
	ResumesAt          *time.Time
}

type CreateInput struct {
	AccountID          int64
	PlanID             int64
	PlanVersion        int
	Quantity           int32
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	BillingCycleAnchor time.Time
}
