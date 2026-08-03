ALTER TABLE invoices
    DROP COLUMN IF EXISTS version;

ALTER TABLE event_log
    ALTER COLUMN aggregate_id TYPE BIGINT USING aggregate_id::bigint;
