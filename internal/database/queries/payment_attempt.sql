-- name: CreatePaymentAttempt :one
-- Inserts a new payment attempt. 
-- Will fail if another 'PENDING' attempt exists for this invoice_id
-- or if (invoice_id, attempt_number) is duplicated.
INSERT INTO payment_attempts (
    invoice_id,
    attempt_number,
    idempotency_key,
    provider,
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
-- Computes the next attempt_number for an invoice (e.g., 0 -> 1, 1 -> 2).
SELECT COALESCE(MAX(attempt_number), 0) + 1 AS next_attempt_number
FROM payment_attempts
WHERE invoice_id = $1;

-- name: GetPaymentAttemptByID :one
-- Fetches a single attempt by its primary UUID.
SELECT 
    id, invoice_id, attempt_number, idempotency_key,
    provider, provider_tx_id, amount_minor, currency,
    status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE id = $1;

-- name: GetPaymentAttemptByIdempotencyKey :one
-- Checks if an attempt with this idempotency key already exists.
SELECT 
    id, invoice_id, attempt_number, idempotency_key,
    provider, provider_tx_id, amount_minor, currency,
    status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE idempotency_key = $1;

-- name: GetPaymentAttemptByProviderTxID :one
-- Used by Webhook handlers to find an attempt using the gateway's transaction reference.
SELECT 
    id, invoice_id, attempt_number, idempotency_key,
    provider, provider_tx_id, amount_minor, currency,
    status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE provider = $1 AND provider_tx_id = $2;

-- name: GetPendingPaymentAttemptByInvoiceID :one
-- Checks if an invoice currently has an active 'PENDING' attempt.
SELECT 
    id, invoice_id, attempt_number, idempotency_key,
    provider, provider_tx_id, amount_minor, currency,
    status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE invoice_id = $1 AND status = 'PENDING';

-- name: UpdatePaymentAttemptStatus :exec
-- Updates status, provider transaction ID, and optional raw payload response.
-- Transitioning from 'PENDING' -> 'SUCCESS'/'FAILED' removes the row from the partial unique index.
UPDATE payment_attempts
SET 
    status = $2,
    provider_tx_id = COALESCE($3, provider_tx_id),
    raw_response = COALESCE($4, raw_response),
    updated_at = NOW()
WHERE id = $1;

-- name: ListPaymentAttemptsByInvoiceID :many
-- Retrieves the full history of payment attempts for an invoice, ordered sequentially.
SELECT 
    id, invoice_id, attempt_number, idempotency_key,
    provider, provider_tx_id, amount_minor, currency,
    status, raw_response, created_at, updated_at
FROM payment_attempts
WHERE invoice_id = $1
ORDER BY attempt_number ASC;