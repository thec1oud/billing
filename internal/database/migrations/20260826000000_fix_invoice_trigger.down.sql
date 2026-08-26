CREATE OR REPLACE FUNCTION protect_finalized_invoice_header()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.invoice_status_code IS DISTINCT FROM 'DRAFT' THEN
        IF (NEW.subtotal_amount <> OLD.subtotal_amount) OR
           (NEW.tax_amount <> OLD.tax_amount) OR
           (NEW.discount_amount <> OLD.discount_amount) OR
           (NEW.total_amount <> OLD.total_amount) OR
           (NEW.currency <> OLD.currency) OR
           (NEW.account_id <> OLD.account_id) THEN
            RAISE EXCEPTION 'Cannot modify financial structure of a finalized invoice (ID: %, Status: %).',
                OLD.invoice_id, OLD.invoice_status_code;
        END IF;
    END IF;
    NEW.updated_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
