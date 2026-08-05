-- Remove the legacy tariff type.
DELETE FROM tariff_type
WHERE tariff_type_code = 'ONE_TIME';

-- Add supported pricing models.
INSERT INTO tariff_type (tariff_type_code)
VALUES
    ('STAIRSTEP'),
    ('PACKAGE'),
    ('MATRIX'),
    ('COMPOSITE')
ON CONFLICT (tariff_type_code) DO NOTHING;

-- Amount and currency must either both exist
-- or both be NULL.
ALTER TABLE tariffs
    ADD CONSTRAINT chk_tariff_amount_currency
    CHECK (
        (amount IS NULL AND currency IS NULL)
        OR
        (amount IS NOT NULL AND currency IS NOT NULL)
    );