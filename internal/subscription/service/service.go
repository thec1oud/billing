package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	accountrepository "github.com/thec1oud/billing/internal/account/repository"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	sm_engine "github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
	"github.com/thec1oud/billing/internal/shared/timeutil"
	subscription "github.com/thec1oud/billing/internal/subscription"
	"github.com/thec1oud/billing/internal/subscription/model"
	subscriptionstatemachine "github.com/thec1oud/billing/internal/subscription/statemachine"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
)

type SubscriptionRepository interface {
	Pool() *pgxpool.Pool
	Create(ctx context.Context, db sqlcgen.DBTX, in model.CreateInput) (model.Subscription, error)
	Get(ctx context.Context, db sqlcgen.DBTX, id int64) (model.Subscription, error)
	ListAccountSubscriptions(ctx context.Context, db sqlcgen.DBTX, accountID int64) ([]model.Subscription, error)
}

type Service struct {
	repository SubscriptionRepository
	accounts   *accountrepository.Repository
	plans      planrepo.Repository
	tariffs    tariffrepo.Repository
	smEngine   *sm_engine.Engine
	events     *eventservice.Service
}

func New(
	repository SubscriptionRepository,
	accounts *accountrepository.Repository,
	plans planrepo.Repository,
	tariffs tariffrepo.Repository,
	smEngine *sm_engine.Engine,
	events *eventservice.Service,
) *Service {
	return &Service{
		repository: repository,
		accounts:   accounts,
		plans:      plans,
		tariffs:    tariffs,
		smEngine:   smEngine,
		events:     events,
	}
}

// Create creates an ACTIVE subscription for an existing ACTIVE account
// and immutable plan version.
func (s *Service) Create(
	ctx context.Context,
	actor eventmodel.Actor,
	input model.CreateInput,
) (model.Subscription, error) {
	if input.AccountID <= 0 || input.PlanID <= 0 {
		return model.Subscription{}, errors.New(
			"account id and plan id must be greater than zero",
		)
	}

	if input.PlanVersion <= 0 {
		return model.Subscription{}, errors.New(
			"plan version must be greater than zero",
		)
	}

	if s.plans == nil {
		return model.Subscription{}, errors.New(
			"plan repository is not configured",
		)
	}

	if s.accounts == nil {
		return model.Subscription{}, errors.New(
			"account repository is not configured",
		)
	}

	if s.repository == nil {
		return model.Subscription{}, errors.New(
			"subscription repository is not configured",
		)
	}

	// The plan repository only exposes immutable code/version lookup.
	// Resolve by ID is intentionally performed through the database relation
	// at insert time; callers supply the selected immutable ID.
	periodStart, anchor, periodEnd, err := CalculatePeriod(
		input.CurrentPeriodStart,
		input.BillingCycleAnchor,
		input.CurrentPeriodEnd,
	)
	if err != nil {
		return model.Subscription{}, err
	}
	input.CurrentPeriodStart = periodStart
	input.BillingCycleAnchor = anchor
	input.CurrentPeriodEnd = periodEnd

	tx, err := s.repository.Pool().Begin(ctx)
	if err != nil {
		return model.Subscription{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	p, err := s.plans.GetByID(ctx, input.PlanID)
	if err != nil {
		return model.Subscription{}, fmt.Errorf(
			"get plan: %w",
			err,
		)
	}

	if p.Version != input.PlanVersion {
		return model.Subscription{}, fmt.Errorf(
			"plan %d version mismatch: requested=%d actual=%d",
			input.PlanID,
			input.PlanVersion,
			p.Version,
		)
	}

	status, err := s.accounts.LockAccountForUpdate(ctx, tx, input.AccountID)
	if err != nil {
		return model.Subscription{}, err
	}

	if status != accountmodel.StatusActive {
		return model.Subscription{}, fmt.Errorf(
			"account %d is not active",
			input.AccountID,
		)
	}

	created, err := s.repository.Create(ctx, tx, input)
	if err != nil {
		return model.Subscription{}, err
	}
	if s.smEngine != nil {
		initialContext, _ := json.Marshal(map[string]any{"subscription_id": created.SubscriptionID, "status": created.Status})
		if _, err := s.smEngine.CreateInstance(
			ctx,
			sm_model.MachineType("subscription_lifecycle"),
			"subscription",
			fmt.Sprintf("%d", created.SubscriptionID),
			initialContext,
			sm_engine.WithTx(tx), // share the outer tx: SM instance is committed atomically with the subscription row and event
		); err != nil {
			return model.Subscription{}, fmt.Errorf("create subscription state machine instance: %w", err)
		}
	}

	payload := map[string]any{
		"subscription_id": created.SubscriptionID,
		"account_id":      created.AccountID,
		"plan_id":         created.PlanID,
		"plan_version":    created.PlanVersion,
		"quantity":        created.Quantity,
	}
	if _, err := s.events.AppendEvent(ctx, tx, eventmodel.AppendRequest{
		AggregateType: eventmodel.AggregateSubscription,
		AggregateID:   fmt.Sprintf("%d", created.SubscriptionID),
		EventType:     eventmodel.SubscriptionCreated,
		EventVersion:  1,
		Actor:         actor,
		Payload:       payload,
	}); err != nil {
		return model.Subscription{}, fmt.Errorf("append SubscriptionCreated: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return model.Subscription{}, fmt.Errorf("commit transaction: %w", err)
	}

	return created, nil
}

func (s *Service) Get(
	ctx context.Context,
	subscriptionID int64,
) (model.Subscription, error) {
	return s.repository.Get(ctx, s.repository.Pool(), subscriptionID)
}

func (s *Service) ListAccountSubscriptions(
	ctx context.Context,
	accountID int64,
) ([]model.Subscription, error) {
	return s.repository.ListAccountSubscriptions(ctx, s.repository.Pool(), accountID)
}

func (s *Service) Pause(ctx context.Context, actor eventmodel.Actor, subscriptionID int64) (model.Subscription, error) {
	return s.fireTransition(ctx, actor, subscriptionID, subscriptionstatemachine.EventPause, nil)
}

func (s *Service) Resume(ctx context.Context, actor eventmodel.Actor, subscriptionID int64) (model.Subscription, error) {
	return s.fireTransition(ctx, actor, subscriptionID, subscriptionstatemachine.EventResume, nil)
}

func (s *Service) Cancel(ctx context.Context, actor eventmodel.Actor, subscriptionID int64, atPeriodEnd bool) (model.Subscription, error) {
	payload, _ := json.Marshal(map[string]any{"at_period_end": atPeriodEnd})
	return s.fireTransition(ctx, actor, subscriptionID, subscriptionstatemachine.EventCancel, payload)
}

func (s *Service) fireTransition(
	ctx context.Context,
	actor eventmodel.Actor,
	subscriptionID int64,
	event sm_model.EventName,
	payload []byte,
) (model.Subscription, error) {
	if s.smEngine == nil {
		return model.Subscription{}, errors.New("subscription state machine is not configured")
	}

	instance, err := s.smEngine.GetInstanceBySubject(
		ctx,
		"subscription",
		fmt.Sprintf("%d", subscriptionID),
		subscriptionstatemachine.SubscriptionMachineType,
	)
	if err != nil {
		return model.Subscription{}, fmt.Errorf("get subscription state machine: %w", err)
	}

	if _, err := s.smEngine.Fire(ctx, instance.InstanceID, event, payload, sm_engine.WithEventActor(actor)); err != nil {
		return model.Subscription{}, fmt.Errorf("fire subscription transition %q: %w", event, err)
	}

	return s.repository.Get(ctx, s.repository.Pool(), subscriptionID)
}

// BillingProjection resolves the current active tariff for the subscription's
// immutable plan.
//
// Plan versions and tariff versions are immutable, so invoices can retain
// this result as a snapshot.
func (s *Service) BillingProjection(
	ctx context.Context,
	subscriptionID int64,
) (subscription.BillingProjection, error) {
	if s.repository == nil {
		return subscription.BillingProjection{}, errors.New(
			"subscription repository is not configured",
		)
	}

	sub, err := s.repository.Get(ctx, s.repository.Pool(), subscriptionID)
	if err != nil {
		return subscription.BillingProjection{}, err
	}

	if s.plans == nil {
		return subscription.BillingProjection{}, errors.New(
			"plan repository is not configured",
		)
	}

	// The subscription retains the database plan ID; pricing is intentionally
	// resolved from its active duration.
	//
	// A richer catalog may select a duration explicitly, but an ambiguous plan
	// must never silently invoice.
	durations, err := s.plans.ListDurations(ctx, sub.PlanID)
	if err != nil {
		return subscription.BillingProjection{}, fmt.Errorf(
			"list plan durations: %w",
			err,
		)
	}

	active := 0

	for _, d := range durations {
		if d.IsActive {
			active++
		}
	}

	if active != 1 {
		return subscription.BillingProjection{}, fmt.Errorf(
			"plan %d must have exactly one active billing duration, got %d",
			sub.PlanID,
			active,
		)
	}

	for _, d := range durations {
		if !d.IsActive {
			continue
		}

		if s.tariffs == nil {
			return subscription.BillingProjection{}, errors.New(
				"tariff repository is not configured",
			)
		}

		t, err := s.tariffs.GetByID(ctx, d.TariffID)
		if err != nil {
			return subscription.BillingProjection{}, fmt.Errorf(
				"get billing tariff: %w",
				err,
			)
		}

		if !t.IsActive {
			return subscription.BillingProjection{}, fmt.Errorf(
				"billing tariff %d is inactive",
				d.TariffID,
			)
		}

		return subscription.BillingProjection{
			SubscriptionID: sub.SubscriptionID,
			AccountID:      sub.AccountID,
			PlanID:         sub.PlanID,
			PlanVersion:    sub.PlanVersion,
			Amount:         t.Amount,
			Currency:       t.Amount.Currency,
			PeriodStart:    sub.CurrentPeriodStart,
			PeriodEnd:      sub.CurrentPeriodEnd,
		}, nil
	}

	return subscription.BillingProjection{}, errors.New(
		"active billing duration disappeared",
	)
}

// CalculatePeriod computes the period boundaries and cycle anchor for a subscription.
// Defaulting rules:
//   - If start is zero, timeutil.NowUTC() is used.
//   - If anchor is zero, the start timestamp is used as the anchor.
//   - If end is zero, it is computed via timeutil.MonthlyPeriodEnd(start, anchor.Day()),
//     which properly clamps days (29, 30, 31) to target month ends without month overflow.
//
// All boundaries are strictly normalized to UTC. Account timezones are preserved for
// customer invoice display and communication, while period boundaries remain UTC-only
// to ensure deterministic, monotonic recurrence across DST changes.
func CalculatePeriod(start, anchor, end time.Time) (periodStart, billingAnchor, periodEnd time.Time, err error) {
	if start.IsZero() {
		periodStart = timeutil.NowUTC()
	} else {
		periodStart = timeutil.ToUTC(start)
	}

	if anchor.IsZero() {
		billingAnchor = periodStart
	} else {
		billingAnchor = timeutil.ToUTC(anchor)
	}

	if end.IsZero() {
		computedEnd, err := timeutil.MonthlyPeriodEnd(periodStart, billingAnchor.Day())
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("calculate period end: %w", err)
		}
		periodEnd = computedEnd
	} else {
		periodEnd = timeutil.ToUTC(end)
	}

	return periodStart, billingAnchor, periodEnd, nil
}
