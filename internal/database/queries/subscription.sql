-- name: CreateSubscription :one
INSERT INTO subscriptions (
    account_id,
    plan_id,
    plan_version,
    subscription_status_code,
    quantity,
    current_period_start_at,
    current_period_end_at,
    billing_cycle_anchor
) VALUES ($1, $2, $3, 'ACTIVE', $4, $5, $6, $7)
RETURNING subscription_id, version, account_id, plan_id, plan_version, subscription_status_code, quantity, current_period_start_at, current_period_end_at, billing_cycle_anchor, cancel_at_period_end, canceled_at, ended_at, paused_at, resumes_at;

-- name: GetSubscription :one
SELECT subscription_id, version, account_id, plan_id, plan_version, subscription_status_code, quantity, current_period_start_at, current_period_end_at, billing_cycle_anchor, cancel_at_period_end, canceled_at, ended_at, paused_at, resumes_at
FROM subscriptions
WHERE subscription_id = $1;

-- name: ListAccountSubscriptions :many
SELECT subscription_id, version, account_id, plan_id, plan_version, subscription_status_code, quantity, current_period_start_at, current_period_end_at, billing_cycle_anchor, cancel_at_period_end, canceled_at, ended_at, paused_at, resumes_at
FROM subscriptions
WHERE account_id = $1
ORDER BY subscription_id;

-- name: TransitionSubscription :execrows
UPDATE subscriptions
SET subscription_status_code = $3,
    version = version + 1,
    canceled_at = CASE WHEN $3 = 'CANCELED' THEN $4 ELSE canceled_at END,
    ended_at = CASE WHEN $3 = 'CANCELED' THEN $4 ELSE ended_at END,
    paused_at = CASE WHEN $3 = 'PAUSED' THEN $4 ELSE paused_at END,
    resumes_at = CASE WHEN $3 = 'ACTIVE' THEN $4 ELSE resumes_at END
WHERE subscription_id = $1
  AND subscription_status_code = $2;

-- name: ScheduleSubscriptionCancellation :execrows
UPDATE subscriptions
SET cancel_at_period_end = TRUE,
    version = version + 1
WHERE subscription_id = $1
  AND subscription_status_code IN ('ACTIVE', 'PAUSED');

-- name: CancelSubscriptionNow :execrows
UPDATE subscriptions
SET subscription_status_code = 'CANCELED',
    version = version + 1,
    canceled_at = $2,
    ended_at = $2,
    cancel_at_period_end = FALSE
WHERE subscription_id = $1
  AND subscription_status_code IN ('ACTIVE', 'PAUSED');
