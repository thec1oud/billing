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
    paid_at
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
    amount_due
) VALUES (
    $1, $2, $3,
    $4, $5, $6, $7,
    $8, $9
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

-- name: FinalizeInvoice :one
UPDATE invoices
SET invoice_status_code = 'OPEN',
    invoice_number = CONCAT('INV-', TO_CHAR(CURRENT_DATE, 'YYYY'), '-', LPAD(nextval('invoice_number_seq')::text, 6, '0')),
    subtotal_amount = $1,
    tax_amount = $2,
    discount_amount = $3,
    total_amount = $4,
    amount_due = $5,
    due_at = $6,
    finalized_at = $7
WHERE invoice_id = $8
  AND invoice_status_code = 'DRAFT'
RETURNING invoice_number;

-- name: MarkInvoicePaid :execrows
-- Transitions -> PAID. Updates amounts and sets paid_at timestamp.
UPDATE invoices
SET invoice_status_code = 'PAID',
    amount_paid = $1,
    amount_due = $2,
    paid_at = $3
WHERE invoice_id = $4;

-- name: VoidInvoice :execrows
-- Transitions -> VOID. Leaves balances intact for historical audit.
UPDATE invoices
SET invoice_status_code = 'VOID'
WHERE invoice_id = $1;

-- name: UpdatePaymentBalances :execrows
-- Updates balance details (partial payments, dunning adjustments) without touching status code.
UPDATE invoices
SET amount_paid = $1,
    amount_due = $2
WHERE invoice_id = $3;
