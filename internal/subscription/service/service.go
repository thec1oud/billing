package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	accountrepository "github.com/thec1oud/billing/internal/account/repository"
	planrepo "github.com/thec1oud/billing/internal/plan/repository"
	eventmodel "github.com/thec1oud/billing/internal/shared/eventstore/model"
	sm_engine "github.com/thec1oud/billing/internal/shared/statemachine/engine"
	sm_model "github.com/thec1oud/billing/internal/shared/statemachine/model"
	subscription "github.com/thec1oud/billing/internal/subscription"
	"github.com/thec1oud/billing/internal/subscription/model"
	"github.com/thec1oud/billing/internal/subscription/repository"
	subscriptionstatemachine "github.com/thec1oud/billing/internal/subscription/statemachine"
	tariffrepo "github.com/thec1oud/billing/internal/tariff/repository"
	eventservice "github.com/thec1oud/billing/internal/shared/eventstore/service"
)

type Service struct {
	repository *repository.Repository
	accounts   *accountrepository.Repository
	plans      planrepo.Repository
	tariffs    tariffrepo.Repository
	smEngine   *sm_engine.Engine
	events     *eventservice.Service
}

func New(
	repository *repository.Repository,
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

	a, err := s.accounts.Get(ctx, s.accounts.Pool(), input.AccountID)
	if err != nil {
		return model.Subscription{}, err
	}

	if a.Status != accountmodel.StatusActive {
		return model.Subscription{}, fmt.Errorf(
			"account %d is not active",
			input.AccountID,
		)
	}

	// The plan repository only exposes immutable code/version lookup.
	// Resolve by ID is intentionally performed through the database relation
	// at insert time; callers supply the selected immutable ID.
	now := time.Now().UTC()
	end := now.AddDate(0, 1, 0)

	input.CurrentPeriodStart = now
	input.CurrentPeriodEnd = end
	input.BillingCycleAnchor = now

	tx, err := s.repository.Pool().Begin(ctx)
	if err != nil {
		return model.Subscription{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created, err := s.repository.Create(ctx, tx, input)
	if err != nil {
		return model.Subscription{}, err
	}
	if s.smEngine != nil {
		initialContext, _ := json.Marshal(map[string]any{"subscription_id": created.SubscriptionID, "status": created.Status})
		if _, err := s.smEngine.CreateInstance(ctx, sm_model.MachineType("subscription_lifecycle"), "subscription", fmt.Sprintf("%d", created.SubscriptionID), initialContext); err != nil {
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
			AccountID:   sub.AccountID,
			Amount:      t.Amount,
			PeriodStart: sub.CurrentPeriodStart,
			PeriodEnd:   sub.CurrentPeriodEnd,
		}, nil
	}

	return subscription.BillingProjection{}, errors.New(
		"active billing duration disappeared",
	)
}
