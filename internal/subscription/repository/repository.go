package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

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

const subscriptionColumns = `
	subscription_id,
	account_id,
	plan_id,
	plan_version,
	subscription_status_code,
	quantity,
	current_period_start_at,
	current_period_end_at,
	billing_cycle_anchor,
	cancel_at_period_end,
	canceled_at,
	ended_at,
	paused_at,
	resumes_at
`

func (r *Repository) Create(
	ctx context.Context,
	in model.CreateInput,
) (model.Subscription, error) {
	if r == nil || r.pool == nil {
		return model.Subscription{}, errors.New(
			"subscription repository: database is not configured",
		)
	}

	quantity := in.Quantity
	if quantity == 0 {
		quantity = 1
	}

	row := r.pool.QueryRow(
		ctx,
		`INSERT INTO subscriptions (
			account_id,
			plan_id,
			plan_version,
			subscription_status_code,
			quantity,
			current_period_start_at,
			current_period_end_at,
			billing_cycle_anchor
		) VALUES ($1, $2, $3, 'ACTIVE', $4, $5, $6, $7)
		RETURNING `+subscriptionColumns,
		in.AccountID,
		in.PlanID,
		in.PlanVersion,
		quantity,
		in.CurrentPeriodStart,
		in.CurrentPeriodEnd,
		in.BillingCycleAnchor,
	)

	return scanSubscription(row)
}

func (r *Repository) Get(
	ctx context.Context,
	id int64,
) (model.Subscription, error) {
	if id <= 0 {
		return model.Subscription{}, errors.New(
			"subscription id must be greater than zero",
		)
	}

	if r == nil || r.pool == nil {
		return model.Subscription{}, errors.New(
			"subscription repository: database is not configured",
		)
	}

	row := r.pool.QueryRow(
		ctx,
		`SELECT `+subscriptionColumns+`
		 FROM subscriptions
		 WHERE subscription_id = $1`,
		id,
	)

	return scanSubscription(row)
}

func (r *Repository) Transition(
	ctx context.Context,
	id int64,
	from,
	to model.Status,
	now time.Time,
) error {
	if id <= 0 {
		return errors.New(
			"subscription id must be greater than zero",
		)
	}

	if r == nil || r.pool == nil {
		return errors.New(
			"subscription repository: database is not configured",
		)
	}

	tag, err := r.pool.Exec(
		ctx,
		`UPDATE subscriptions
		 SET subscription_status_code = $3,
		     paused_at = CASE
		         WHEN $3 = 'PAUSED' THEN $4
		         ELSE paused_at
		     END,
		     resumes_at = CASE
		         WHEN $3 = 'ACTIVE' THEN $4
		         ELSE resumes_at
		     END
		 WHERE subscription_id = $1
		   AND subscription_status_code = $2`,
		id,
		string(from),
		string(to),
		now,
	)
	if err != nil {
		return fmt.Errorf(
			"transition subscription: %w",
			err,
		)
	}

	if tag.RowsAffected() != 1 {
		return model.ErrInvalidStateTransition
	}

	return nil
}

func (r *Repository) Cancel(
	ctx context.Context,
	id int64,
	atPeriodEnd bool,
	now time.Time,
) error {
	if r == nil || r.pool == nil {
		return errors.New(
			"subscription repository: database is not configured",
		)
	}

	if atPeriodEnd {
		tag, err := r.pool.Exec(
			ctx,
			`UPDATE subscriptions
			 SET cancel_at_period_end = TRUE
			 WHERE subscription_id = $1
			   AND subscription_status_code IN ('ACTIVE', 'PAUSED')`,
			id,
		)
		if err != nil {
			return fmt.Errorf(
				"schedule cancellation: %w",
				err,
			)
		}

		if tag.RowsAffected() != 1 {
			return model.ErrInvalidStateTransition
		}

		return nil
	}

	tag, err := r.pool.Exec(
		ctx,
		`UPDATE subscriptions
		 SET subscription_status_code = 'CANCELED',
		     canceled_at = $2,
		     ended_at = $2,
		     cancel_at_period_end = FALSE
		 WHERE subscription_id = $1
		   AND subscription_status_code IN ('ACTIVE', 'PAUSED')`,
		id,
		now,
	)
	if err != nil {
		return fmt.Errorf(
			"cancel subscription: %w",
			err,
		)
	}

	if tag.RowsAffected() != 1 {
		return model.ErrInvalidStateTransition
	}

	return nil
}

func scanSubscription(
	row pgx.Row,
) (model.Subscription, error) {
	var value model.Subscription
	var canceledAt pgtype.Timestamptz
	var endedAt pgtype.Timestamptz
	var pausedAt pgtype.Timestamptz
	var resumesAt pgtype.Timestamptz

	err := row.Scan(
		&value.SubscriptionID,
		&value.AccountID,
		&value.PlanID,
		&value.PlanVersion,
		&value.Status,
		&value.Quantity,
		&value.CurrentPeriodStart,
		&value.CurrentPeriodEnd,
		&value.BillingCycleAnchor,
		&value.CancelAtPeriodEnd,
		&canceledAt,
		&endedAt,
		&pausedAt,
		&resumesAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Subscription{}, model.ErrNotFound
	}

	if err != nil {
		return model.Subscription{}, fmt.Errorf(
			"get subscription: %w",
			err,
		)
	}

	value.CanceledAt = timestampPtr(canceledAt)
	value.EndedAt = timestampPtr(endedAt)
	value.PausedAt = timestampPtr(pausedAt)
	value.ResumesAt = timestampPtr(resumesAt)

	return value, nil
}

func timestampPtr(
	value pgtype.Timestamptz,
) *time.Time {
	if !value.Valid {
		return nil
	}

	result := value.Time

	return &result
}
