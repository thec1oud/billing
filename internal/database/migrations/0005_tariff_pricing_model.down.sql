-- ==========================================
-- ROLLBACK TARIFF PRICING MODEL REFINEMENT
-- ==========================================

ALTER TABLE tariffs
    DROP CONSTRAINT IF EXISTS chk_tariff_amount_currency;

DELETE FROM tariff_type
WHERE tariff_type_code IN (
    'STAIRSTEP',
    'PACKAGE',
    'MATRIX',
    'COMPOSITE'
);

INSERT INTO tariff_type (tariff_type_code)
VALUES ('ONE_TIME')
ON CONFLICT (tariff_type_code) DO NOTHING;