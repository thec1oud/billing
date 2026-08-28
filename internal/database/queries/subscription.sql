-- name: CreateSubscription :one
INSERT INTO subscriptions (account_id, plan_id, plan_version, subscription_status_code, current_period_start_at, current_period_end_at, billing_cycle_anchor)
VALUES ($1, $2, $3, 'ACTIVE', $4, $5, $6)
RETURNING subscription_id, account_id, plan_id, plan_version, subscription_status_code, current_period_start_at, current_period_end_at, billing_cycle_anchor;
-- name: GetSubscription :one
SELECT subscription_id, account_id, plan_id, plan_version, subscription_status_code, current_period_start_at, current_period_end_at, billing_cycle_anchor FROM subscriptions WHERE subscription_id = $1;
