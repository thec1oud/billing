ALTER TABLE event_log ALTER COLUMN aggregate_id TYPE UUID USING aggregate_id::UUID;
