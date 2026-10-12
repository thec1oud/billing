-- name: SavePPIWebhook :one
INSERT INTO ppi_webhooks (
    webhook_id, provider_code, event_type, internal_tx_id, provider_tx_id, status, payload, processed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (provider_code, provider_tx_id) WHERE provider_tx_id IS NOT NULL DO NOTHING
RETURNING webhook_id;

-- name: GetPPIWebhookByID :one
SELECT webhook_id, provider_code, event_type, internal_tx_id, provider_tx_id, status, payload, published_at, processed_at
FROM ppi_webhooks
WHERE webhook_id = $1;

-- name: GetPPIWebhookPublishedStatus :one
SELECT (published_at IS NOT NULL)::boolean AS is_published
FROM ppi_webhooks
WHERE provider_code = $1 AND provider_tx_id = $2;

-- name: MarkPPIWebhookPublished :exec
UPDATE ppi_webhooks
SET published_at = NOW()
WHERE webhook_id = $1;
