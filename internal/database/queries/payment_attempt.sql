-- name: CreatePaymentAttempt :one
INSERT INTO payment_attempts (
    invoice_id,
    provider_code,
    internal_tx_id,
    amount_minor,
    currency,
    status,
    raw_request
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING attempt_id, created_at, updated_at;

-- name: GetPaymentAttemptByID :one
SELECT
    attempt_id, invoice_id, provider_code, internal_tx_id, provider_tx_id,
    amount_minor, currency, status, raw_response, raw_request, created_at, updated_at
FROM payment_attempts
WHERE attempt_id = $1;

-- name: GetPaymentAttemptByInternalTxID :one
SELECT
    attempt_id, invoice_id, provider_code, internal_tx_id, provider_tx_id,
    amount_minor, currency, status, raw_response, raw_request, created_at, updated_at
FROM payment_attempts
WHERE internal_tx_id = $1;

-- name: GetPaymentAttemptByProviderTxID :one
SELECT
    attempt_id, invoice_id, provider_code, internal_tx_id, provider_tx_id,
    amount_minor, currency, status, raw_response, raw_request, created_at, updated_at
FROM payment_attempts
WHERE provider_tx_id = $1;

-- name: GetPendingPaymentAttemptByInvoiceID :one
SELECT
    attempt_id, invoice_id, provider_code, internal_tx_id, provider_tx_id,
    amount_minor, currency, status, raw_response, raw_request, created_at, updated_at
FROM payment_attempts
WHERE invoice_id = $1 AND status = 'PENDING';

-- name: UpdatePaymentAttemptResult :exec
UPDATE payment_attempts
SET
    status = $2,
    provider_tx_id = COALESCE($3, provider_tx_id),
    raw_response = COALESCE($4, raw_response),
    updated_at = NOW()
WHERE attempt_id = $1;

-- name: ListPaymentAttemptsByInvoiceID :many
SELECT
    attempt_id, invoice_id, provider_code, internal_tx_id, provider_tx_id,
    amount_minor, currency, status, raw_response, raw_request, created_at, updated_at
FROM payment_attempts
WHERE invoice_id = $1
ORDER BY created_at ASC;


-- name: DeletePaymentAttempt :execresult
DELETE FROM payment_attempts
WHERE attempt_id = $1;

-- name: GetStalePendingAttempts :many
SELECT
    attempt_id, invoice_id, provider_code, internal_tx_id, provider_tx_id,
    amount_minor, currency, status, raw_response, raw_request, created_at, updated_at
FROM payment_attempts
WHERE status = 'PENDING' AND updated_at < $1
ORDER BY updated_at ASC
LIMIT $2;
