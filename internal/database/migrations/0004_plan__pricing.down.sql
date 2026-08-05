ALTER TABLE plan_durations
    DROP CONSTRAINT IF EXISTS chk_plan_duration_tariff_positive;

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS chk_plan_legacy_price_policy;

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS chk_plan_effective_dates;

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS chk_plan_version_positive;

ALTER TABLE plans
    DROP CONSTRAINT IF EXISTS uq_plan_code_version;