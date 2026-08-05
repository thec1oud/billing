-- name: CreatePlan :one
INSERT INTO plans (
    plan_code,
    version,
    effective_from,
    effective_until,
    legacy_price_policy_code,
    migration_path,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;


-- name: CreatePlanDuration :one
INSERT INTO plan_durations (
    plan_id,
    tariff_id,
    duration,
    is_active
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;


-- name: GetPlanByCodeAndVersion :one
SELECT *
FROM plans
WHERE plan_code = $1
  AND version = $2;


-- name: GetLatestPlanVersion :one
SELECT COALESCE(MAX(version), 0)::int AS version
FROM plans
WHERE plan_code = $1;


-- name: GetPlanDurations :many
SELECT *
FROM plan_durations
WHERE plan_id = $1
ORDER BY duration;