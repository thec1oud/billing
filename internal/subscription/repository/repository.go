package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

func (r *Repository) Create(
	ctx context.Context,
	db sqlcgen.DBTX,
	in model.CreateInput,
) (model.Subscription, error) {
	quantity := in.Quantity
	if quantity == 0 {
		quantity = 1
	}

	row, err := sqlcgen.New(db).CreateSubscription(ctx, sqlcgen.CreateSubscriptionParams{
		AccountID:            in.AccountID,
		PlanID:               in.PlanID,
		PlanVersion:          int32(in.PlanVersion),
		Quantity:             quantity,
		CurrentPeriodStartAt: in.CurrentPeriodStart,
		CurrentPeriodEndAt:   in.CurrentPeriodEnd,
		BillingCycleAnchor:   in.BillingCycleAnchor,
	})
	if err != nil {
		return model.Subscription{}, fmt.Errorf("create subscription: %w", err)
	}

	return model.Subscription{
		SubscriptionID:     row.SubscriptionID,
		Version:            row.Version,
		AccountID:          row.AccountID,
		PlanID:             row.PlanID,
		PlanVersion:        int(row.PlanVersion),
		Status:             model.Status(row.SubscriptionStatusCode),
		Quantity:           row.Quantity,
		CurrentPeriodStart: row.CurrentPeriodStartAt,
		CurrentPeriodEnd:   row.CurrentPeriodEndAt,
		BillingCycleAnchor: row.BillingCycleAnchor,
		CancelAtPeriodEnd:  row.CancelAtPeriodEnd,
		CanceledAt:         timestampPtr(row.CanceledAt),
		EndedAt:            timestampPtr(row.EndedAt),
		PausedAt:           timestampPtr(row.PausedAt),
		ResumesAt:          timestampPtr(row.ResumesAt),
	}, nil
}

func (r *Repository) Get(
	ctx context.Context,
	db sqlcgen.DBTX,
	id int64,
) (model.Subscription, error) {
	if id <= 0 {
		return model.Subscription{}, errors.New("subscription id must be greater than zero")
	}

	row, err := sqlcgen.New(db).GetSubscription(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Subscription{}, model.ErrNotFound
	}
	if err != nil {
		return model.Subscription{}, fmt.Errorf("get subscription: %w", err)
	}

	return model.Subscription{
		SubscriptionID:     row.SubscriptionID,
		Version:            row.Version,
		AccountID:          row.AccountID,
		PlanID:             row.PlanID,
		PlanVersion:        int(row.PlanVersion),
		Status:             model.Status(row.SubscriptionStatusCode),
		Quantity:           row.Quantity,
		CurrentPeriodStart: row.CurrentPeriodStartAt,
		CurrentPeriodEnd:   row.CurrentPeriodEndAt,
		BillingCycleAnchor: row.BillingCycleAnchor,
		CancelAtPeriodEnd:  row.CancelAtPeriodEnd,
		CanceledAt:         timestampPtr(row.CanceledAt),
		EndedAt:            timestampPtr(row.EndedAt),
		PausedAt:           timestampPtr(row.PausedAt),
		ResumesAt:          timestampPtr(row.ResumesAt),
	}, nil
}

func (r *Repository) Transition(
	ctx context.Context,
	db sqlcgen.DBTX,
	id int64,
	from,
	to model.Status,
	now time.Time,
) error {
	if id <= 0 {
		return errors.New("subscription id must be greater than zero")
	}

	rows, err := sqlcgen.New(db).TransitionSubscription(ctx, sqlcgen.TransitionSubscriptionParams{
		SubscriptionID:           id,
		SubscriptionStatusCode:   string(from),
		SubscriptionStatusCode_2: string(to),
		CanceledAt:               pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("transition subscription: %w", err)
	}

	if rows != 1 {
		return model.ErrInvalidStateTransition
	}

	return nil
}

func (r *Repository) Cancel(
	ctx context.Context,
	db sqlcgen.DBTX,
	id int64,
	atPeriodEnd bool,
	now time.Time,
) error {
	if atPeriodEnd {
		rows, err := sqlcgen.New(db).ScheduleSubscriptionCancellation(ctx, id)
		if err != nil {
			return fmt.Errorf("schedule cancellation: %w", err)
		}
		if rows != 1 {
			return model.ErrInvalidStateTransition
		}
		return nil
	}

	rows, err := sqlcgen.New(db).CancelSubscriptionNow(ctx, sqlcgen.CancelSubscriptionNowParams{
		SubscriptionID: id,
		CanceledAt:     pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}
	if rows != 1 {
		return model.ErrInvalidStateTransition
	}

	return nil
}

func (r *Repository) ListAccountSubscriptions(
	ctx context.Context,
	db sqlcgen.DBTX,
	accountID int64,
) ([]model.Subscription, error) {
	if accountID <= 0 {
		return nil, errors.New("account id must be greater than zero")
	}

	rows, err := sqlcgen.New(db).ListAccountSubscriptions(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("list account subscriptions: %w", err)
	}

	result := make([]model.Subscription, len(rows))
	for i, row := range rows {
		result[i] = model.Subscription{
			SubscriptionID:     row.SubscriptionID,
			Version:            row.Version,
			AccountID:          row.AccountID,
			PlanID:             row.PlanID,
			PlanVersion:        int(row.PlanVersion),
			Status:             model.Status(row.SubscriptionStatusCode),
			Quantity:           row.Quantity,
			CurrentPeriodStart: row.CurrentPeriodStartAt,
			CurrentPeriodEnd:   row.CurrentPeriodEndAt,
			BillingCycleAnchor: row.BillingCycleAnchor,
			CancelAtPeriodEnd:  row.CancelAtPeriodEnd,
			CanceledAt:         timestampPtr(row.CanceledAt),
			EndedAt:            timestampPtr(row.EndedAt),
			PausedAt:           timestampPtr(row.PausedAt),
			ResumesAt:          timestampPtr(row.ResumesAt),
		}
	}

	return result, nil
}

func timestampPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	result := value.Time
	return &result
}
