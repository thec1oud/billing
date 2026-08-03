ALTER TABLE event_log
    ALTER COLUMN aggregate_id TYPE TEXT USING aggregate_id::text;

-- Optimistic concurrency for the invoices projection, mirroring the
-- sequence-conflict guard already enforced on event_log.
ALTER TABLE invoices
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1;
