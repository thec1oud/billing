CREATE TABLE IF NOT EXISTS payment_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id BIGINT NOT NULL REFERENCES invoices(invoice_id) ON DELETE RESTRICT,
    
    -- Foreign key to stored payment method (NULL if supporting one-off/guest payments)
    payment_method_id BIGINT REFERENCES payment_methods(payment_method_id) ON DELETE RESTRICT,
    
    attempt_number INT NOT NULL DEFAULT 1,
    idempotency_key VARCHAR(255) NOT NULL,
    
    provider_tx_id VARCHAR(255),       
    
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    
    status VARCHAR(25) NOT NULL DEFAULT 'PENDING',
    
    raw_response JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_payment_attempts_amount_positive CHECK (amount_minor > 0),
    CONSTRAINT chk_payment_attempts_attempt_positive CHECK (attempt_number > 0),
    CONSTRAINT uq_payment_attempts_invoice_attempt UNIQUE (invoice_id, attempt_number)
);

CREATE UNIQUE INDEX idx_payment_attempts_single_pending_per_invoice 
    ON payment_attempts(invoice_id) 
    WHERE status = 'PENDING';

CREATE INDEX idx_payment_attempts_payment_method 
    ON payment_attempts(payment_method_id);