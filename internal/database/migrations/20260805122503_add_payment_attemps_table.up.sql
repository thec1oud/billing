CREATE TABLE IF NOT EXISTS payment_attempts (
    attempt_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id BIGINT NOT NULL REFERENCES invoices(invoice_id) ON DELETE RESTRICT,
    
    provider_code VARCHAR(25) REFERENCES payment_provider (payment_provider_code) NOT NULL,
    
    internal_tx_id VARCHAR(255) NOT NULL,
    
    provider_tx_id VARCHAR(255),       
    
    amount_minor BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL,
    
    status VARCHAR(25) NOT NULL DEFAULT 'PENDING',
    
    raw_response JSONB,
    raw_request JSONB,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_payment_attempts_single_pending_per_invoice 
    ON payment_attempts(invoice_id) 
    WHERE status = 'PENDING';

