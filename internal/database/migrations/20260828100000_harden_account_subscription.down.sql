DROP INDEX IF EXISTS idx_subscriptions_account_status;
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS chk_subscription_period_order;
DROP INDEX IF EXISTS uq_payment_methods_provider_reference;
DROP INDEX IF EXISTS uq_payment_methods_default_per_account;
