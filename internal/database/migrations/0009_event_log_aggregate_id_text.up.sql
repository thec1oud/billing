ALTER TABLE event_log ALTER COLUMN aggregate_id TYPE TEXT USING aggregate_id::TEXT;
