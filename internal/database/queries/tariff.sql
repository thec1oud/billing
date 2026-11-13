-- name: CreateTariff :one

WITH locked AS (
    SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
),
next_version AS (
    SELECT COALESCE(MAX(version), 0) + 1 AS version
    FROM tariffs, locked
    WHERE tariff_code = $1
)

INSERT INTO tariffs (
    tariff_code,
    version,
    name,
    description,
    tariff_type_code,
    tier_strategy,
    quantity_unit,
    amount,
    currency,
    tier_brackets,
    is_active,
    metadata
)

SELECT
    $1,
    next_version.version,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10,
    $11

FROM next_version

RETURNING
    tariff_id,
    tariff_code,
    version,
    name,
    description,
    tariff_type_code,
    tier_strategy,
    quantity_unit,
    amount,
    currency,
    tier_brackets,
    is_active,
    metadata,
    created_at;


-- name: GetTariffByCodeAndVersion :one
SELECT
    tariff_id,
    tariff_code,
    version,
    name,
    description,
    tariff_type_code,
    tier_strategy,
    quantity_unit,
    amount,
    currency,
    tier_brackets,
    is_active,
    metadata,
    created_at
FROM tariffs
WHERE tariff_code = $1
  AND version = $2;

-- name: GetTariffByID :one
SELECT
    tariff_id,
    tariff_code,
    version,
    name,
    description,
    tariff_type_code,
    tier_strategy,
    quantity_unit,
    amount,
    currency,
    tier_brackets,
    is_active,
    metadata,
    created_at
FROM tariffs
WHERE tariff_id = $1;


-- name: GetLatestTariffVersion :one
SELECT COALESCE(MAX(version), 0)::int
FROM tariffs
WHERE tariff_code = $1;


-- name: ListTariffVersions :many
SELECT
    tariff_id,
    tariff_code,
    version,
    name,
    description,
    tariff_type_code,
    tier_strategy,
    quantity_unit,
    amount,
    currency,
    tier_brackets,
    is_active,
    metadata,
    created_at
FROM tariffs
WHERE tariff_code = $1
ORDER BY version DESC;

