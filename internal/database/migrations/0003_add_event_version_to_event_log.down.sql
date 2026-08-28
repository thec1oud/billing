ALTER TABLE event_log
    DROP CONSTRAINT IF EXISTS chk_event_version_positive;

ALTER TABLE event_log
    DROP COLUMN IF EXISTS event_version;
