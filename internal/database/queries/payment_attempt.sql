-- name: CreatePaymentAttempt :one
INSERT INTO payment_attempts (
    invoice_id,
    payment_method_id,
    attempt_number,
    idempotency_key,
    provider_tx_id,
    amount_minor,
    currency,
    status,
    raw_response
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING id, created_at, updated_at;

-- name: GetNextAttemptNumber :one
SELECT COALESCE(MAX(attempt_number), 0) + 1 AS next_attempt_number
FROM payment_attempts
WHERE invoice_id = $1;

-- name: GetPaymentAttemptByID :one
SELECT 
    id, invoice_id, payment_method_id,
    attempt_number, idempotency_key, provider_tx_id, amount_minor,
    currency, status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE id = $1;

-- name: GetPaymentAttemptByIdempotencyKey :one
SELECT 
    id, invoice_id, payment_method_id,
    attempt_number, idempotency_key, provider_tx_id, amount_minor,
    currency, status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE idempotency_key = $1;

-- name: GetPaymentAttemptByProviderTxID :one
SELECT 
    id, invoice_id, payment_method_id,
    attempt_number, idempotency_key, provider_tx_id, amount_minor,
    currency, status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE provider_tx_id = $1;

-- name: GetPendingPaymentAttemptByInvoiceID :one
SELECT 
    id, invoice_id, payment_method_id,
    attempt_number, idempotency_key, provider_tx_id, amount_minor,
    currency, status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE invoice_id = $1 AND status = 'PENDING';

-- name: UpdatePaymentAttemptStatus :exec
UPDATE payment_attempts
SET 
    status = $2,
    provider_tx_id = COALESCE($3, provider_tx_id),
    raw_response = COALESCE($4, raw_response),
    updated_at = NOW()
WHERE id = $1;

-- name: ListPaymentAttemptsByInvoiceID :many
SELECT 
    id, invoice_id, payment_method_id,
    attempt_number, idempotency_key, provider_tx_id, amount_minor,
    currency, status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE invoice_id = $1
ORDER BY attempt_number ASC;

-- name: ListPaymentAttemptsByPaymentMethodID :many
SELECT 
    id, invoice_id, payment_method_id,
    attempt_number, idempotency_key, provider_tx_id, amount_minor,
    currency, status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE payment_method_id = $1
ORDER BY created_at DESC;