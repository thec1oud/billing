-- name: CreateSubscription :one
INSERT INTO subscriptions (account_id, plan_id, plan_version, subscription_status_code, current_period_start_at, current_period_end_at, billing_cycle_anchor)
VALUES ($1, $2, $3, 'ACTIVE', $4, $5, $6)
RETURNING subscription_id, version, account_id, plan_id, plan_version, subscription_status_code, current_period_start_at, current_period_end_at, billing_cycle_anchor, canceled_at, ended_at;
-- name: GetSubscription :one
SELECT subscription_id, version, account_id, plan_id, plan_version, subscription_status_code, current_period_start_at, current_period_end_at, billing_cycle_anchor, canceled_at, ended_at FROM subscriptions WHERE subscription_id = $1;

-- name: ListAccountSubscriptions :many
SELECT subscription_id, version, account_id, plan_id, plan_version, subscription_status_code, current_period_start_at, current_period_end_at, billing_cycle_anchor, canceled_at, ended_at
FROM subscriptions 
WHERE account_id = $1 AND subscription_status_code = 'ACTIVE';
