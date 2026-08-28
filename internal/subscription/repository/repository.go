package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thec1oud/billing/internal/shared/sqlcgen"
	"github.com/thec1oud/billing/internal/subscription/model"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
	}
}

func (r *Repository) Create(
	ctx context.Context,
	in model.CreateInput,
) (model.Subscription, error) {
	if r == nil || r.pool == nil {
		return model.Subscription{}, errors.New(
			"subscription repository: database is not configured",
		)
	}

	row, err := sqlcgen.New(r.pool).CreateSubscription(
		ctx,
		sqlcgen.CreateSubscriptionParams{
			AccountID:            in.AccountID,
			PlanID:               in.PlanID,
			PlanVersion:          int32(in.PlanVersion),
			CurrentPeriodStartAt: in.CurrentPeriodStart,
			CurrentPeriodEndAt:   in.CurrentPeriodEnd,
			BillingCycleAnchor:   in.BillingCycleAnchor,
		},
	)
	if err != nil {
		return model.Subscription{}, fmt.Errorf(
			"create subscription: %w",
			err,
		)
	}

	return model.Subscription{
		SubscriptionID:     row.SubscriptionID,
		AccountID:          row.AccountID,
		PlanID:             row.PlanID,
		PlanVersion:        int(row.PlanVersion),
		Status:             model.Status(row.SubscriptionStatusCode),
		CurrentPeriodStart: row.CurrentPeriodStartAt,
		CurrentPeriodEnd:   row.CurrentPeriodEndAt,
		BillingCycleAnchor: row.BillingCycleAnchor,
	}, nil
}

func (r *Repository) Get(
	ctx context.Context,
	subscriptionID int64,
) (model.Subscription, error) {
	if subscriptionID <= 0 {
		return model.Subscription{}, errors.New(
			"subscription id must be greater than zero",
		)
	}

	if r == nil || r.pool == nil {
		return model.Subscription{}, errors.New(
			"subscription repository: database is not configured",
		)
	}

	row, err := sqlcgen.New(r.pool).GetSubscription(
		ctx,
		subscriptionID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Subscription{}, model.ErrNotFound
	}

	if err != nil {
		return model.Subscription{}, fmt.Errorf(
			"get subscription %d: %w",
			subscriptionID,
			err,
		)
	}

	return toModel(row), nil
}

func toModel(row sqlcgen.GetSubscriptionRow) model.Subscription {
	return model.Subscription{
		SubscriptionID:     row.SubscriptionID,
		AccountID:          row.AccountID,
		PlanID:             row.PlanID,
		PlanVersion:        int(row.PlanVersion),
		Status:             model.Status(row.SubscriptionStatusCode),
		CurrentPeriodStart: row.CurrentPeriodStartAt,
		CurrentPeriodEnd:   row.CurrentPeriodEndAt,
		BillingCycleAnchor: row.BillingCycleAnchor,
	}
}
