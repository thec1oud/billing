-- name: GetInvoice :one
SELECT
    invoice_id,
    account_id,
    invoice_number,
    invoice_status_code,
    currency,
    subtotal_amount,
    tax_amount,
    discount_amount,
    total_amount,
    amount_paid,
    amount_due,
    due_at,
    finalized_at,
    paid_at,
    version
FROM invoices
WHERE invoice_id = $1;

-- name: ListInvoiceLineItems :many
SELECT
    line_item_id,
    invoice_id,
    item_id,
    subscription_id,
    description,
    quantity_value,
    quantity_unit,
    unit_amount,
    total_amount,
    metadata
FROM invoice_line_items
WHERE invoice_id = $1
ORDER BY line_item_id ASC;

-- name: CreateInvoice :one
INSERT INTO invoices (
    account_id,
    invoice_status_code,
    currency,
    subtotal_amount,
    tax_amount,
    discount_amount,
    total_amount,
    amount_paid,
    amount_due,
    version
) VALUES (
    $1, $2, $3,
    $4, $5, $6, $7,
    $8, $9, 1
)
RETURNING invoice_id;

-- name: CreateInvoiceLineItem :exec
INSERT INTO invoice_line_items (
    invoice_id,
    item_id,
    subscription_id,
    description,
    quantity_value,
    quantity_unit,
    unit_amount,
    total_amount,
    metadata
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9
);

-- ============================================================================
-- FINE-GRAINED STATE & BALANCE TRANSITIONS
-- ============================================================================

-- name: FinalizeInvoice :execrows
-- Transitions DRAFT -> OPEN. Sets finalized_at timestamp.
UPDATE invoices
SET invoice_status_code = 'OPEN',
    finalized_at = $1,
    version = version + 1
WHERE invoice_id = $2 AND version = $3;

-- name: MarkInvoicePaid :execrows
-- Transitions -> PAID. Updates amounts and sets paid_at timestamp.
UPDATE invoices
SET invoice_status_code = 'PAID',
    amount_paid = $1,
    amount_due = $2,
    paid_at = $3,
    version = version + 1
WHERE invoice_id = $4 AND version = $5;

-- name: VoidInvoice :execrows
-- Transitions -> VOID. Leaves balances intact for historical audit.
UPDATE invoices
SET invoice_status_code = 'VOID',
    version = version + 1
WHERE invoice_id = $1 AND version = $2;

-- name: UpdatePaymentBalances :execrows
-- Updates balance details (partial payments, dunning adjustments) without touching status code.
UPDATE invoices
SET amount_paid = $1,
    amount_due = $2,
    version = version + 1
WHERE invoice_id = $3 AND version = $4;
