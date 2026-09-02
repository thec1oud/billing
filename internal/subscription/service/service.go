package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	accountmodel "github.com/thec1oud/billing/internal/account/model"
	accountrepository "github.com/thec1oud/billing/internal/account/repository"
	"github.com/thec1oud/billing/internal/plan"
	"github.com/thec1oud/billing/internal/shared/money"
	"github.com/thec1oud/billing/internal/subscription/model"
	"github.com/thec1oud/billing/internal/subscription/repository"
	"github.com/thec1oud/billing/internal/tariff"
)

type Service struct {
	repository *repository.Repository
	accounts   *accountrepository.Repository
	plans      plan.Repository
	tariffs    tariff.Repository
}

func New(
	repository *repository.Repository,
	accounts *accountrepository.Repository,
	plans plan.Repository,
	tariffs tariff.Repository,
) *Service {
	return &Service{
		repository: repository,
		accounts:   accounts,
		plans:      plans,
		tariffs:    tariffs,
	}
}

// Create creates an ACTIVE subscription for an existing ACTIVE account
// and immutable plan version.
func (s *Service) Create(
	ctx context.Context,
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

	a, err := s.accounts.Get(ctx, input.AccountID)
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

	return s.repository.Create(ctx, input)
}

func (s *Service) Get(
	ctx context.Context,
	subscriptionID int64,
) (model.Subscription, error) {
	return s.repository.Get(ctx, subscriptionID)
}

func (s *Service) ListAccountSubscriptions(
	ctx context.Context,
	accountID int64,
) ([]model.Subscription, error) {
	return s.repository.ListAccountSubscriptions(ctx, accountID)
}

type BillingProjection struct {
	AccountID   int64
	Amount      money.Money
	PeriodStart time.Time
	PeriodEnd   time.Time
}

// BillingProjection resolves the current active tariff for the subscription's
// immutable plan.
//
// Plan versions and tariff versions are immutable, so invoices can retain
// this result as a snapshot.
func (s *Service) BillingProjection(
	ctx context.Context,
	subscriptionID int64,
) (BillingProjection, error) {
	sub, err := s.repository.Get(ctx, subscriptionID)
	if err != nil {
		return BillingProjection{}, err
	}

	if s.plans == nil {
		return BillingProjection{}, errors.New(
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
		return BillingProjection{}, fmt.Errorf(
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
		return BillingProjection{}, fmt.Errorf(
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
			return BillingProjection{}, errors.New(
				"tariff repository is not configured",
			)
		}

		t, err := s.tariffs.GetByID(ctx, d.TariffID)
		if err != nil {
			return BillingProjection{}, fmt.Errorf(
				"get billing tariff: %w",
				err,
			)
		}

		if !t.IsActive {
			return BillingProjection{}, fmt.Errorf(
				"billing tariff %d is inactive",
				d.TariffID,
			)
		}

		return BillingProjection{
			AccountID:   sub.AccountID,
			Amount:      t.Amount,
			PeriodStart: sub.CurrentPeriodStart,
			PeriodEnd:   sub.CurrentPeriodEnd,
		}, nil
	}

	return BillingProjection{}, errors.New(
		"active billing duration disappeared",
	)
}
