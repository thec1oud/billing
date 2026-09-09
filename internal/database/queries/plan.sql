-- name: CreatePlan :one
WITH locked AS (
    SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
), next_version AS (
    SELECT COALESCE(MAX(version), 0) + 1 AS version
    FROM plans, locked
    WHERE plan_code = $1
)
INSERT INTO plans (
    plan_code,
    version,
    effective_from,
    effective_until,
    legacy_price_policy_code,
    migration_path,
    metadata
)
SELECT
    $1,
    next_version.version,
    $2,
    $3,
    $4,
    $5,
    $6
FROM next_version
RETURNING
    plan_id,
    plan_code,
    version,
    effective_from,
    effective_until,
    legacy_price_policy_code,
    migration_path,
    metadata,
    created_at;


-- name: GetLatestPlanVersion :one
SELECT COALESCE(MAX(version), 0)::int
FROM plans
WHERE plan_code = $1;


-- name: GetPlanByCodeAndVersion :one
SELECT
    plan_id,
    plan_code,
    version,
    effective_from,
    effective_until,
    legacy_price_policy_code,
    migration_path,
    metadata,
    created_at
FROM plans
WHERE plan_code = $1
  AND version = $2;

-- name: GetPlanByID :one
SELECT plan_id, plan_code, version, effective_from, effective_until,
    legacy_price_policy_code, migration_path, metadata, created_at
FROM plans
WHERE plan_id = $1;


-- name: ListPlanVersions :many
SELECT
    plan_id,
    plan_code,
    version,
    effective_from,
    effective_until,
    legacy_price_policy_code,
    migration_path,
    metadata,
    created_at
FROM plans
WHERE plan_code = $1
ORDER BY version DESC;

-- name: ListPlans :many
SELECT
    plan_id,
    plan_code,
    version,
    effective_from,
    effective_until,
    legacy_price_policy_code,
    migration_path,
    metadata,
    created_at
FROM plans
ORDER BY plan_id DESC;


-- name: CreatePlanDuration :one
INSERT INTO plan_durations (
    plan_id,
    tariff_id,
    duration,
    is_active
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    plan_duration_id,
    plan_id,
    tariff_id,
    duration,
    is_active,
    created_at;


-- name: GetPlanDuration :one
SELECT
    plan_duration_id,
    plan_id,
    tariff_id,
    duration,
    is_active,
    created_at
FROM plan_durations
WHERE plan_duration_id = $1;


-- name: ListPlanDurations :many
SELECT
    plan_duration_id,
    plan_id,
    tariff_id,
    duration,
    is_active,
    created_at
FROM plan_durations
WHERE plan_id = $1
ORDER BY plan_duration_id;


-- name: GetPlanDurationByPlanAndDuration :one
SELECT
    plan_duration_id,
    plan_id,
    tariff_id,
    duration,
    is_active,
    created_at
FROM plan_durations
WHERE plan_id = $1
  AND duration = $2;


-- name: UpdatePlanDurationTariff :one
UPDATE plan_durations
SET
    tariff_id = $2,
    is_active = $3
WHERE plan_duration_id = $1
RETURNING
    plan_duration_id,
    plan_id,
    tariff_id,
    duration,
    is_active,
    created_at;

-- name: ListActivePlans :many
SELECT DISTINCT ON (plan_code)
    plan_id,
    plan_code,
    version,
    effective_from,
    effective_until,
    legacy_price_policy_code,
    migration_path,
    metadata,
    created_at
FROM plans
WHERE effective_until IS NULL
ORDER BY plan_code, version DESC;
