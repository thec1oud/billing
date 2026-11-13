-- name: AppendEvent :one
INSERT INTO event_log (
    event_type,
    event_version,
    aggregate_type,
    aggregate_id,
    actor,
    causation_id,
    correlation_id,
    payload
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
RETURNING event_id, sequence, occurred_at;

-- name: ReadStream :many
SELECT 
    event_id,
    event_type,
    event_version,
    aggregate_type,
    aggregate_id,
    sequence,
    actor,
    causation_id,
    correlation_id,
    payload,
    occurred_at
FROM event_log
WHERE aggregate_type = $1 AND aggregate_id = $2
ORDER BY sequence ASC;

-- name: ReadStreamFrom :many
SELECT 
    event_id,
    event_type,
    event_version,
    aggregate_type,
    aggregate_id,
    sequence,
    actor,
    causation_id,
    correlation_id,
    payload,
    occurred_at
FROM event_log
WHERE aggregate_type = $1 
  AND aggregate_id = $2 
  AND sequence >= $3
ORDER BY sequence ASC;