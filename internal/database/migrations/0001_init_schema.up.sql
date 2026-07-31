CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ==========================================
-- 0. LOOKUP & REFERENCE TABLES
-- ==========================================

CREATE TABLE tariff_type (
    tariff_type_code VARCHAR(64) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE billing_interval (
    billing_interval_code VARCHAR(32) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE plan_legacy_price_policy (
    legacy_price_policy_code VARCHAR(64) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE item_type (
    item_type_code VARCHAR(64) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE account_status (
    account_status_code VARCHAR(32) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE payment_provider (
    payment_provider_code VARCHAR(50) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE payment_type (
    payment_type_code VARCHAR(50) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE payment_status (
    payment_status_code VARCHAR(30) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE subscription_status (
    subscription_status_code VARCHAR(32) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE invoice_status (
    invoice_status_code VARCHAR(32) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ledger_party_type (
    ledger_party_type_code VARCHAR(32) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE ledger_entry_type (
    ledger_entry_type_code VARCHAR(40) PRIMARY KEY,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed Initial Reference Values
INSERT INTO tariff_type (tariff_type_code) VALUES ('FLAT_FEE'), ('PER_UNIT'), ('TIERED_USAGE'), ('ONE_TIME');
INSERT INTO billing_interval (billing_interval_code) VALUES ('DAY'), ('WEEK'), ('MONTH'), ('YEAR');
INSERT INTO plan_legacy_price_policy (legacy_price_policy_code) VALUES ('KEEP_FOREVER'), ('MIGRATE_IMMEDIATELY'), ('MIGRATE_ON_RENEWAL');
INSERT INTO item_type (item_type_code) VALUES ('PLAN'), ('ONE_TIME_SERVICE'), ('PRODUCT');
INSERT INTO account_status (account_status_code) VALUES ('PENDING_VERIFICATION'), ('ACTIVE'), ('SUSPENDED'), ('CLOSED');
INSERT INTO payment_provider (payment_provider_code) VALUES ('stripe'), ('chapa'), ('telebirr'), ('paypal');
INSERT INTO payment_type (payment_type_code) VALUES ('card'), ('bank'), ('mobile_money');
INSERT INTO payment_status (payment_status_code) VALUES ('ACTIVE'), ('INACTIVE'), ('EXPIRED'), ('REVOKED');
INSERT INTO subscription_status (subscription_status_code) VALUES ('TRIALING'), ('ACTIVE'), ('PAUSED'), ('PAST_DUE'), ('CANCELED'), ('UNPAID');
INSERT INTO invoice_status (invoice_status_code) VALUES ('DRAFT'), ('OPEN'), ('PAID'), ('UNCOLLECTIBLE'), ('VOID');
INSERT INTO ledger_party_type (ledger_party_type_code) VALUES ('ACCOUNT'), ('PLATFORM'), ('PROVIDER');
INSERT INTO ledger_entry_type (ledger_entry_type_code) VALUES ('CREDIT_TRANSFER'), ('REFUND'), ('PROMOTIONAL'), ('MANUAL_ADJUSTMENT');

-- ==========================================
-- 1. TARIFFS & PLANS
-- ==========================================


CREATE TABLE tariffs (
    tariff_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tariff_code VARCHAR(128) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tariff_type_code VARCHAR(64) NOT NULL REFERENCES tariff_type(tariff_type_code),
    amount BIGINT,
    billing_interval_code VARCHAR(32) REFERENCES billing_interval(billing_interval_code),
    interval_count INT,
    tier_brackets JSONB,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_tariff_code_version UNIQUE (tariff_code, version)
);

CREATE TABLE plans (
    plan_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    plan_code VARCHAR(128) UNIQUE NOT NULL,
    version INT NOT NULL DEFAULT 1,
    tariff_id BIGINT NOT NULL REFERENCES tariffs(tariff_id),
    effective_from TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    effective_until TIMESTAMPTZ,
    legacy_price_policy_code VARCHAR(64) NOT NULL DEFAULT 'KEEP_FOREVER' REFERENCES plan_legacy_price_policy(legacy_price_policy_code),
    migration_path JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_plan_code_version UNIQUE (plan_code, version)
);

CREATE TABLE purchasable_items (
    item_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    item_code VARCHAR(128) UNIQUE NOT NULL,
    item_type_code VARCHAR(64) NOT NULL REFERENCES item_type(item_type_code),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    plan_id BIGINT REFERENCES plans(plan_id) ON DELETE SET NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

);

-- ==========================================
-- 2. PRICING RULES
-- ==========================================

CREATE TABLE pricing_rules (
    rule_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    rule_code VARCHAR(128) UNIQUE NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    rule_payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE purchasable_item_pricing_rules (
    item_id BIGINT REFERENCES purchasable_items(item_id) ON DELETE CASCADE,
    rule_id BIGINT REFERENCES pricing_rules(rule_id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (item_id, rule_id)
);

-- ==========================================
-- 3. ACCOUNTS & PAYMENT METHODS
-- ==========================================

CREATE TABLE accounts (
    account_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    external_id VARCHAR(255) UNIQUE,
    account_status_code VARCHAR(32) NOT NULL DEFAULT 'PENDING_VERIFICATION' REFERENCES account_status(account_status_code),
    currency VARCHAR(3) NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    locale VARCHAR(35) NOT NULL DEFAULT 'en-US',
    net_terms SMALLINT NOT NULL DEFAULT 0,
    dunning_profile_id BIGINT,
    tax_identifiers JSONB NOT NULL DEFAULT '{}'::jsonb,
    billing_address JSONB NOT NULL DEFAULT '{}'::jsonb,
    compliance_flags JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

);


CREATE TABLE payment_methods (
    payment_method_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(account_id),
    payment_provider_code VARCHAR(50) NOT NULL REFERENCES payment_provider(payment_provider_code),
    provider_reference VARCHAR(255) NOT NULL,
    payment_type_code VARCHAR(50) NOT NULL REFERENCES payment_type(payment_type_code),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    payment_status_code VARCHAR(30) NOT NULL DEFAULT 'ACTIVE' REFERENCES payment_status(payment_status_code),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

);

-- ==========================================
-- 4. EVENT LOG
-- ==========================================

CREATE TABLE event_log (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(128) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id UUID NOT NULL,
    sequence BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor JSONB NOT NULL,
    causation_id UUID REFERENCES event_log(event_id),
    correlation_id UUID,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT uq_aggregate_sequence UNIQUE (aggregate_type, aggregate_id, sequence)
);

-- ==========================================
-- 5. SUBSCRIPTIONS
-- ==========================================

CREATE TABLE subscriptions (
    subscription_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(account_id),
    plan_id BIGINT NOT NULL REFERENCES plans(plan_id),
    plan_version INT NOT NULL DEFAULT 1,
    subscription_status_code VARCHAR(32) NOT NULL DEFAULT 'TRIALING' REFERENCES subscription_status(subscription_status_code),
    quantity INT NOT NULL DEFAULT 1,
    current_period_start_at TIMESTAMPTZ NOT NULL,
    current_period_end_at TIMESTAMPTZ NOT NULL,
    billing_cycle_anchor TIMESTAMPTZ NOT NULL,
    trial_start_at TIMESTAMPTZ,
    trial_end_at TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    canceled_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    paused_at TIMESTAMPTZ,
    resumes_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

);

-- ==========================================
-- 6. INVOICES & LINE ITEMS
-- ==========================================

CREATE TABLE invoices (
    invoice_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(account_id),
    invoice_number VARCHAR(64) UNIQUE,
    invoice_status_code VARCHAR(32) NOT NULL DEFAULT 'DRAFT' REFERENCES invoice_status(invoice_status_code),
    currency VARCHAR(3) NOT NULL,
    subtotal_amount BIGINT NOT NULL DEFAULT 0,
    tax_amount BIGINT NOT NULL DEFAULT 0,
    discount_amount BIGINT NOT NULL DEFAULT 0,
    total_amount BIGINT NOT NULL DEFAULT 0,
    amount_paid BIGINT NOT NULL DEFAULT 0,
    amount_due BIGINT NOT NULL DEFAULT 0,
    due_at TIMESTAMPTZ,
    finalized_at TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    billing_address_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key VARCHAR(255) UNIQUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

);


CREATE TABLE invoice_line_items (
    line_item_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    invoice_id BIGINT NOT NULL REFERENCES invoices(invoice_id) ON DELETE CASCADE,
    item_id BIGINT NOT NULL REFERENCES purchasable_items(item_id),
    subscription_id BIGINT REFERENCES subscriptions(subscription_id) ON DELETE SET NULL,
    description TEXT NOT NULL,
    quantity_value NUMERIC(18, 4) NOT NULL DEFAULT 1.0000,
    quantity_unit VARCHAR(32) NOT NULL DEFAULT 'UNIT',
    unit_amount BIGINT NOT NULL,
    total_amount BIGINT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ==========================================
-- 7. CREDIT LEDGER
-- ==========================================

CREATE TABLE credit_ledger (
    entry_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_party_type_code VARCHAR(32) NOT NULL REFERENCES ledger_party_type(ledger_party_type_code),
    source_party_id VARCHAR(255) NOT NULL,
    destination_party_type_code VARCHAR(32) NOT NULL REFERENCES ledger_party_type(ledger_party_type_code),
    destination_party_id VARCHAR(255) NOT NULL,
    ledger_entry_type_code VARCHAR(40) NOT NULL REFERENCES ledger_entry_type(ledger_entry_type_code),
    amount BIGINT NOT NULL CHECK (amount > 0),
    reference_type VARCHAR(50),
    reference_id VARCHAR(255),
    description TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ==========================================
-- 8. FUNCTIONS & TRIGGERS
-- ==========================================

CREATE OR REPLACE FUNCTION enforce_account_currency_immutability()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.currency IS DISTINCT FROM NEW.currency THEN
        IF EXISTS (
            SELECT 1 FROM credit_ledger
            WHERE (source_party_type_code = 'ACCOUNT' AND source_party_id = OLD.account_id::VARCHAR)
               OR (destination_party_type_code = 'ACCOUNT' AND destination_party_id = OLD.account_id::VARCHAR)
            UNION ALL
            SELECT 1 FROM invoices WHERE account_id = OLD.account_id
        ) THEN
            RAISE EXCEPTION 'Currency cannot be modified after financial activity has been recorded for account %', OLD.account_id;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_protect_account_currency
    BEFORE UPDATE ON accounts
    FOR EACH ROW
    EXECUTE FUNCTION enforce_account_currency_immutability();

CREATE OR REPLACE FUNCTION prevent_event_log_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Event log is strictly append-only. Operation "%" is forbidden.', TG_OP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_protect_event_log_immutability
    BEFORE UPDATE OR DELETE ON event_log
    FOR EACH ROW
    EXECUTE FUNCTION prevent_event_log_mutation();

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
    NEW.;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_protect_invoice_immutability
    BEFORE UPDATE ON invoices
    FOR EACH ROW
    EXECUTE FUNCTION protect_finalized_invoice_header();
