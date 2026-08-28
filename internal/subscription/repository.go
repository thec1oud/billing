package subscription

import (
	"context"
	"strconv"

	"github.com/thec1oud/billing/internal/plan"
	events "github.com/thec1oud/billing/internal/shared/eventstore"
)

// Repository is responsible for persisting and rebuilding
// Subscription aggregates from the event store.
type Repository struct {
	store events.EventStore
}

// NewRepository creates a new Subscription repository.
func NewRepository(store events.EventStore) *Repository {
	return &Repository{
		store: store,
	}
}

// Append stores a single Subscription event in the event store.
func (r *Repository) Append(
	ctx context.Context,
	req events.AppendRequest,
) error {

	_, err := r.store.Append(ctx, req)
	return err
}

// Get rebuilds a Subscription aggregate by replaying
// its event stream.
func (r *Repository) Get(
	ctx context.Context,
	subscriptionID int64,
) (*Subscription, error) {

	stream, err := r.store.ReadStream(
		ctx,
		events.AggregateSubscription,
		strconv.FormatInt(subscriptionID, 10),
	)
	if err != nil {
		return nil, err
	}

	return Rebuild(stream)
}

// BillingProjection returns the data required by the
// Invoice Engine to generate invoice line items.
func (r *Repository) BillingProjection(
	ctx context.Context,
	subscriptionID int64,
	planRepository plan.Repository,
) (*BillingProjection, error) {

	subscription, err := r.Get(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}

	selectedPlan, err := planRepository.GetByCodeAndVersion(
		ctx,
		subscription.PlanID,
		subscription.PlanVersion,
	)
	if err != nil {
		return nil, err
	}

	return &BillingProjection{
		SubscriptionID: subscription.SubscriptionID,
		AccountID:      subscription.AccountID,

		PlanID:      selectedPlan.PlanCode,
		PlanVersion: selectedPlan.Version,

		Amount:   selectedPlan.FlatFeeAmount,
		Currency: string(selectedPlan.FlatFeeAmount.Currency),

		PeriodStart: subscription.CurrentPeriodStart,
		PeriodEnd:   subscription.CurrentPeriodEnd,
	}, nil
}
