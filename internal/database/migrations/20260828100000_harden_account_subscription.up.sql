-- Keep payment-method selection and subscription periods consistent under concurrency.
CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_methods_default_per_account
    ON payment_methods (account_id)
    WHERE is_default AND payment_status_code = 'ACTIVE';

CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_methods_provider_reference
    ON payment_methods (payment_provider_code, provider_reference);

ALTER TABLE subscriptions
    ADD CONSTRAINT chk_subscription_period_order
    CHECK (current_period_end_at > current_period_start_at);

CREATE INDEX IF NOT EXISTS idx_subscriptions_account_status
    ON subscriptions (account_id, subscription_status_code);
