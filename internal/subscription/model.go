package subscription

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/substrate/events"
)

type Status string

const (
	StatusActive Status = "ACTIVE"
)

// Subscription is the aggregate root.
//
// Milestone 1 intentionally supports only the ACTIVE state.
// Future milestones will introduce PAUSED, CANCELLED,
// EXPIRED, TRIALING, etc.
type Subscription struct {
	SubscriptionID     uuid.UUID
	AccountID          uuid.UUID
	PlanID             string
	PlanVersion        int
	Status             Status
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	BillingCycleAnchor int

	Version int64
}

// Apply updates the aggregate state from an event.
func (s *Subscription) Apply(event events.Event) error {

	switch event.EventType {

	case events.SubscriptionCreated:

		var payload SubscriptionCreated

		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}

		s.SubscriptionID = payload.SubscriptionID
		s.AccountID = payload.AccountID
		s.PlanID = payload.PlanID
		s.PlanVersion = payload.PlanVersion

		s.CurrentPeriodStart = payload.CurrentPeriodStart
		s.CurrentPeriodEnd = payload.CurrentPeriodEnd
		s.BillingCycleAnchor = payload.BillingCycleAnchor

		// Milestone 1:
		// Every newly-created subscription immediately becomes ACTIVE.
		s.Status = StatusActive

		s.Version = event.Sequence
	}

	return nil
}

// Rebuild reconstructs a Subscription aggregate from its event stream.
func Rebuild(stream []events.Event) (*Subscription, error) {

	subscription := &Subscription{}

	for _, event := range stream {

		if err := subscription.Apply(event); err != nil {
			return nil, err
		}

	}

	return subscription, nil
}