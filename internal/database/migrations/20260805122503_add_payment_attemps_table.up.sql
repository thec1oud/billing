
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id BIGINT NOT NULL REFERENCES invoices(id) ON DELETE RESTRICT,
    
    -- Client-provided idempotency key 
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    
    -- Sequential attempt counter for a single invoice (1, 2, 3...)
    attempt_number INT NOT NULL DEFAULT 1,
    
    provider VARCHAR(64) NOT NULL,    
    provider_tx_id VARCHAR(255),       
    
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    
    status payment_status NOT NULL DEFAULT 'PENDING',
    
    raw_response JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT chk_payments_amount_positive CHECK (amount_minor > 0),
    CONSTRAINT chk_payments_attempt_positive CHECK (attempt_number > 0)
);



-- can never attempt payment when there already is a pending attemp with pending status
CREATE UNIQUE INDEX idx_payments_single_pending_per_invoice 
    ON payments(invoice_id) 
    WHERE status = 'PENDING';