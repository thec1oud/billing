
-- Plan versions must start from 1.
ALTER TABLE plans
    ADD CONSTRAINT chk_plan_version_positive
    CHECK (version > 0);

-- If an effective end date exists,
-- it must be after the effective start date.
ALTER TABLE plans
    ADD CONSTRAINT chk_plan_effective_dates
    CHECK (
        effective_until IS NULL
        OR effective_until > effective_from
    );

-- Only supported legacy price policies are allowed.
ALTER TABLE plans
    ADD CONSTRAINT chk_plan_legacy_price_policy
    CHECK (
        legacy_price_policy_code IN (
            'KEEP_FOREVER',
            'MIGRATE_IMMEDIATELY',
            'MIGRATE_ON_RENEWAL'
        )
    );


-- Tariff ID must reference a valid positive ID.
ALTER TABLE plan_durations
    ADD CONSTRAINT chk_plan_duration_tariff_positive
    CHECK (tariff_id > 0);