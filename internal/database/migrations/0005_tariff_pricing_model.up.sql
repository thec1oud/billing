
-- ==========================================
-- TARIFF PRICING MODEL REFINEMENT
-- ==========================================


DELETE FROM tariff_type
WHERE tariff_type_code = 'ONE_TIME';

INSERT INTO tariff_type (tariff_type_code)
VALUES
    ('STAIRSTEP'),
    ('PACKAGE'),
    ('MATRIX'),
    ('COMPOSITE')
ON CONFLICT (tariff_type_code) DO NOTHING;

-- Money must always have an associated currency.
ALTER TABLE tariffs
    ADD COLUMN currency VARCHAR(3);

-- Existing rows need to be backfilled before enforcing the invariant.
-- Replace this value if the existing development data uses another currency.
UPDATE tariffs
SET currency = 'USD'
WHERE amount IS NOT NULL
  AND currency IS NULL;

ALTER TABLE tariffs
    ALTER COLUMN currency SET NOT NULL;

-- Amount and currency must appear together.
ALTER TABLE tariffs
    ADD CONSTRAINT chk_tariff_amount_currency
    CHECK (
        (amount IS NULL AND currency IS NULL)
        OR
        (amount IS NOT NULL AND currency IS NOT NULL)
    );

-- A duration count must be positive whenever supplied.
ALTER TABLE tariffs
    ADD CONSTRAINT chk_tariff_interval_count_positive
    CHECK (
        interval_count IS NULL
        OR interval_count > 0
    );

-- A billing interval is meaningful only when an interval count exists.
ALTER TABLE tariffs
    ADD CONSTRAINT chk_tariff_interval_consistency
    CHECK (
        billing_interval_code IS NULL
        OR interval_count IS NOT NULL
    );

