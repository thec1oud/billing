CREATE TABLE IF NOT EXISTS payment_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id BIGINT NOT NULL REFERENCES invoices(invoice_id) ON DELETE RESTRICT,
    
    -- Sequential attempt counter for a single invoice (1, 2, 3...)
    attempt_number INT NOT NULL DEFAULT 1,
    
    -- Client-provided idempotency key per attempt
    idempotency_key VARCHAR(255) NOT NULL,
    
    provider VARCHAR(64) NOT NULL,    
    provider_tx_id VARCHAR(255),       
    
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    
    status VARCHAR(25) NOT NULL DEFAULT 'PENDING',
    
    raw_response JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT chk_payment_attempts_amount_positive CHECK (amount_minor > 0),
    CONSTRAINT chk_payment_attempts_attempt_positive CHECK (attempt_number > 0),
    CONSTRAINT uq_payment_attempts_invoice_attempt UNIQUE (invoice_id, attempt_number)
);

-- Guarantees at most one active PENDING attempt per invoice
CREATE UNIQUE INDEX idx_payment_attempts_single_pending_per_invoice 
    ON payment_attempts(invoice_id) 
    WHERE status = 'PENDING';