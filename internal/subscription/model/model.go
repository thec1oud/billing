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
	ErrNotFound              = errors.New("subscription not found")
	ErrInvalidStateTransition = errors.New("invalid subscription state transition")
)

// Subscription is the database-backed billing agreement and its current period.
type Subscription struct {
	SubscriptionID     int64      `json:"id"`
	AccountID          int64      `json:"account_id"`
	PlanID             int64      `json:"plan_id"`
	PlanVersion        int        `json:"plan_version"`
	Status             Status     `json:"status"`
	Quantity           int32      `json:"quantity"`
	CurrentPeriodStart time.Time  `json:"current_period_start"`
	CurrentPeriodEnd   time.Time  `json:"current_period_end"`
	BillingCycleAnchor time.Time  `json:"billing_cycle_anchor"`
	CancelAtPeriodEnd  bool       `json:"cancel_at_period_end"`
	CanceledAt         *time.Time `json:"canceled_at,omitempty"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	PausedAt           *time.Time `json:"paused_at,omitempty"`
	ResumesAt          *time.Time `json:"resumes_at,omitempty"`
}

type CreateInput struct {
	AccountID          int64     `json:"account_id"`
	PlanID             int64     `json:"plan_id"`
	PlanVersion        int       `json:"plan_version,omitempty"`
	Quantity           int32     `json:"quantity,omitempty"`
	CurrentPeriodStart time.Time `json:"current_period_start,omitempty"`
	CurrentPeriodEnd   time.Time `json:"current_period_end,omitempty"`
	BillingCycleAnchor time.Time `json:"billing_cycle_anchor,omitempty"`
}