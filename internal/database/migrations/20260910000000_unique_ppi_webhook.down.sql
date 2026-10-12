DROP INDEX IF EXISTS idx_ppi_webhooks_provider_tx_unique;
CREATE INDEX IF NOT EXISTS idx_ppi_webhooks_provider_tx
ON ppi_webhooks (provider_code, provider_tx_id);
