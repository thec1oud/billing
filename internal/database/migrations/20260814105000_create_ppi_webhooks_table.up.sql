CREATE TABLE IF NOT EXISTS ppi_webhooks (
    webhook_id VARCHAR(64) PRIMARY KEY,
    provider_code VARCHAR(25) NOT NULL REFERENCES payment_provider (payment_provider_code),
    event_type VARCHAR(64) NOT NULL,
    internal_tx_id VARCHAR(255),
    provider_tx_id VARCHAR(255),
    status VARCHAR(32) NOT NULL,
    payload JSONB NOT NULL,
    published_at TIMESTAMPTZ,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ppi_webhooks_provider_tx
ON ppi_webhooks (provider_code, provider_tx_id);

CREATE INDEX IF NOT EXISTS idx_ppi_webhooks_internal_tx
ON ppi_webhooks (internal_tx_id);

CREATE INDEX IF NOT EXISTS idx_ppi_webhooks_unpublished
ON ppi_webhooks (processed_at) WHERE published_at IS NULL;
