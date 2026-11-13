DROP INDEX IF EXISTS idx_ppi_webhooks_provider_tx;
CREATE UNIQUE INDEX IF NOT EXISTS idx_ppi_webhooks_provider_tx_unique
ON ppi_webhooks (provider_code, provider_tx_id)
WHERE provider_tx_id IS NOT NULL;
