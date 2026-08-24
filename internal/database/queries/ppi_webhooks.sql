-- name: SavePPIWebhook :exec
INSERT INTO ppi_webhooks (
    webhook_id, provider_code, event_type, internal_tx_id, provider_tx_id, status, payload, processed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (webhook_id) DO NOTHING;

-- name: GetPPIWebhookByID :one
SELECT webhook_id, provider_code, event_type, internal_tx_id, provider_tx_id, status, payload, published_at, processed_at
FROM ppi_webhooks
WHERE webhook_id = $1;

-- name: CheckPPIWebhookStatus :one
SELECT 
    (EXISTS (SELECT 1 FROM ppi_webhooks w1 WHERE w1.provider_code = $1 AND w1.provider_tx_id = $2))::boolean AS is_duplicate,
    (EXISTS (SELECT 1 FROM ppi_webhooks w2 WHERE w2.provider_code = $1 AND w2.provider_tx_id = $2 AND w2.published_at IS NOT NULL))::boolean AS is_published;

-- name: MarkPPIWebhookPublished :exec
UPDATE ppi_webhooks
SET published_at = NOW()
WHERE webhook_id = $1;
