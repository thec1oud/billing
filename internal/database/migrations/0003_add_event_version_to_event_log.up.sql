ALTER TABLE event_log
    ADD COLUMN event_version INT NOT NULL DEFAULT 1;

-- Add a safety check constraint to enforce positive version numbers
ALTER TABLE event_log
    ADD CONSTRAINT chk_event_version_positive CHECK (event_version > 0);