CREATE TABLE events (

    event_id UUID PRIMARY KEY,

    event_type TEXT NOT NULL,
    event_version INT NOT NULL,

    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,

    sequence BIGINT NOT NULL,

    actor TEXT NOT NULL,

    payload JSONB NOT NULL,

    occurred_at TIMESTAMPTZ(6) NOT NULL,

    causation_id UUID NULL,
    correlation_id UUID NULL,

    CONSTRAINT uq_event_sequence
        UNIQUE (aggregate_type, aggregate_id, sequence)
);

CREATE INDEX idx_event_stream
ON events (
    aggregate_type,
    aggregate_id,
    sequence
);
