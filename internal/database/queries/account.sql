-- name: CreateAccount :one
INSERT INTO accounts (
    external_id, account_status_code, currency, timezone, locale, net_terms,
    dunning_profile_id, tax_identifiers, billing_address, compliance_flags, metadata
) VALUES ($1, 'PENDING_VERIFICATION', $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING external_id, account_id, account_status_code, currency, timezone, locale,
    net_terms, dunning_profile_id, tax_identifiers, billing_address, compliance_flags, metadata;
-- name: GetAccount :one
SELECT external_id, account_id, account_status_code, currency, timezone, locale, net_terms,
    dunning_profile_id, tax_identifiers, billing_address, compliance_flags, metadata
FROM accounts WHERE account_id = $1;
-- name: UpdateAccountStatus :exec
UPDATE accounts SET account_status_code = $2 WHERE account_id = $1;
-- name: CreatePaymentMethod :exec
INSERT INTO payment_methods (account_id, payment_provider_code, provider_reference, payment_type_code, is_default, payment_status_code)
VALUES ($1, 'chapa', $2, 'mobile_money', TRUE, 'ACTIVE');
-- name: ListPaymentMethodReferences :many
SELECT provider_reference FROM payment_methods WHERE account_id = $1 AND payment_status_code = 'ACTIVE' ORDER BY is_default DESC, payment_method_id;
