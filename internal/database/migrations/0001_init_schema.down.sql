-- ============================================================================
-- 000001_init_schema.down.sql
-- Reverses all schema objects created in up migration (reverse dependency order)
-- ============================================================================

-- 1. DROP TRIGGERS & FUNCTIONS
DROP TRIGGER IF EXISTS trg_protect_invoice_immutability ON invoices;
DROP FUNCTION IF EXISTS protect_finalized_invoice_header();

DROP TRIGGER IF EXISTS trg_protect_event_log_immutability ON event_log;
DROP FUNCTION IF EXISTS prevent_event_log_mutation();

DROP TRIGGER IF EXISTS trg_protect_account_currency ON accounts;
DROP FUNCTION IF EXISTS enforce_account_currency_immutability();

-- 2. DROP DOMAIN TABLES
DROP TABLE IF EXISTS credit_ledger;
DROP TABLE IF EXISTS invoice_line_items;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS event_log;
DROP TABLE IF EXISTS payment_methods;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS purchasable_item_pricing_rules;
DROP TABLE IF EXISTS pricing_rules;
DROP TABLE IF EXISTS purchasable_items;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS tariffs;

-- 3. DROP LOOKUP & REFERENCE TABLES
DROP TABLE IF EXISTS ledger_entry_type;
DROP TABLE IF EXISTS ledger_party_type;
DROP TABLE IF EXISTS invoice_status;
DROP TABLE IF EXISTS subscription_status;
DROP TABLE IF EXISTS payment_status;
DROP TABLE IF EXISTS payment_type;
DROP TABLE IF EXISTS payment_provider;
DROP TABLE IF EXISTS account_status;
DROP TABLE IF EXISTS item_type;
DROP TABLE IF EXISTS plan_legacy_price_policy;
DROP TABLE IF EXISTS billing_interval;
DROP TABLE IF EXISTS tariff_type;

-- 4. DROP EXTENSIONS
DROP EXTENSION IF EXISTS "pgcrypto";
