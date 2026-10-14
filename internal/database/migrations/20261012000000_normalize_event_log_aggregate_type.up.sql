WITH affected AS (
    SELECT DISTINCT lower(aggregate_type) AS aggregate_type, aggregate_id
    FROM event_log
    WHERE aggregate_type <> lower(aggregate_type)
),
renumbered AS (
    SELECT e.event_id,
           lower(e.aggregate_type) AS aggregate_type,
           ROW_NUMBER() OVER (
               PARTITION BY lower(e.aggregate_type), e.aggregate_id
               ORDER BY e.occurred_at, e.sequence, e.event_id
           ) AS new_sequence
    FROM event_log e
    JOIN affected a
      ON a.aggregate_type = lower(e.aggregate_type)
     AND a.aggregate_id = e.aggregate_id
)
UPDATE event_log e
SET aggregate_type = r.aggregate_type,
    sequence = -r.new_sequence
FROM renumbered r
WHERE e.event_id = r.event_id;

UPDATE event_log SET sequence = -sequence WHERE sequence < 0;
