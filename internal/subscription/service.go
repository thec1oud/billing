package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/thec1oud/billing/internal/account"
	"github.com/thec1oud/billing/internal/plan"
	sharedUUID "github.com/thec1oud/billing/internal/shared/uuid"
	"github.com/thec1oud/billing/internal/substrate/events"
	"github.com/thec1oud/billing/internal/substrate/idempotency"
	"github.com/thec1oud/billing/internal/substrate/timeutil"
)

type Service struct {
	repository        *Repository
	accountRepository *account.Repository
	planRepository    *plan.Repository
	idem              *idempotency.Store
}

func NewService(
	repository *Repository,
	accountRepository *account.Repository,
	planRepository *plan.Repository,
	idem *idempotency.Store,
) *Service {

	return &Service{
		repository:        repository,
		accountRepository: accountRepository,
		planRepository:    planRepository,
		idem:              idem,
	}
}

// CreateSubscription creates a new ACTIVE subscription.
//
// Milestone 1 intentionally omits:
//
//   - Trial subscriptions
//   - Discounts
//   - Add-ons
//   - Pause / Resume
//   - Cancellation
//
func (s *Service) CreateSubscription(
	ctx context.Context,
	accountID uuid.UUID,
	planID string,
	idempotencyKey string,
) (*Subscription, error) {

	//------------------------------------------------------------------
	// Idempotency
	//------------------------------------------------------------------

	requestHash := idempotency.HashRequest(
		[]byte(fmt.Sprintf("%s:%s", accountID.String(), planID)),
	)

	decision, err := s.idem.CheckOrReserve(
		ctx,
		idempotencyKey,
		"subscription.create",
		requestHash,
	)
	if err != nil {
		return nil, err
	}

	// Return cached response if this request already succeeded.
	if !decision.ShouldProceed() {

		var cached Subscription

		if err := json.Unmarshal(
			decision.CachedResponse,
			&cached,
		); err != nil {
			return nil, err
		}

		return &cached, nil
	}

	//------------------------------------------------------------------
	// Verify Account Exists
	//------------------------------------------------------------------

	acc, err := s.accountRepository.Get(
		ctx,
		accountID,
	)
	if err != nil {
		return nil, err
	}

	if acc == nil {
		return nil, fmt.Errorf("account %s not found", accountID)
	}

	//------------------------------------------------------------------
	// Load Latest Immutable Plan Version
	//------------------------------------------------------------------

	latestVersion, err := s.planRepository.LatestVersion(
		ctx,
		planID,
	)
	if err != nil {
		return nil, err
	}

	selectedPlan, err := s.planRepository.Get(
		ctx,
		planID,
		latestVersion,
	)
	if err != nil {
		return nil, err
	}

	//------------------------------------------------------------------
	// Compute Billing Period
	//------------------------------------------------------------------

	now := timeutil.NowUTC()

	var currentPeriodEnd time.Time

	switch selectedPlan.BillingPeriod {

	case plan.BillingPeriodMonthly:

		currentPeriodEnd, err = timeutil.MonthlyPeriodEnd(
			now,
			now.Day(),
		)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf(
			"unsupported billing period %s",
			selectedPlan.BillingPeriod,
		)
	}

	//------------------------------------------------------------------
	// Create Aggregate ID
	//------------------------------------------------------------------

	subscriptionID, err := sharedUUID.New()
	if err != nil {
		return nil, err
	}

	//------------------------------------------------------------------
	// Build Event Payload
	//------------------------------------------------------------------

	event := SubscriptionCreated{
		SubscriptionID:     subscriptionID,
		AccountID:          accountID,
		PlanID:             selectedPlan.ID,
		PlanVersion:        selectedPlan.Version,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   currentPeriodEnd,
		BillingCycleAnchor: now.Day(),
	}

	//------------------------------------------------------------------
	// Append Event
	//------------------------------------------------------------------

	err = s.repository.Append(
		ctx,
		events.AppendRequest{
			AggregateType: events.AggregateSubscription,
			AggregateID:   subscriptionID,
			Sequence:      1,
			EventType:     events.SubscriptionCreated,
			EventVersion:  1,
			Actor:         "subscription.service",
			Payload:       event,
		},
	)
	if err != nil {
		return nil, err
	}

	//------------------------------------------------------------------
	// Rebuild Aggregate
	//------------------------------------------------------------------

	subscription, err := s.repository.Get(
		ctx,
		subscriptionID,
	)
	if err != nil {
		return nil, err
	}

	//------------------------------------------------------------------
	// Cache Response
	//------------------------------------------------------------------

	if err := s.idem.StoreResponse(
		ctx,
		*decision.ProceedToken,
		subscription,
	); err != nil {
		return nil, err
	}

	return subscription, nil
}