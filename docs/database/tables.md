# Database Tables Reference

This document provides a comprehensive technical reference for all **36 tables** in the Gebeta Billing database, reflecting the applied migrations up to `20260909000000_add_fake_payment_provider`.

---

## 1. Lookup & Reference Tables

These tables define static enums and valid reference sets. Foreign keys across the domain point to these tables to prevent invalid code insertions.

### `tariff_type`
- **Purpose**: Defines pricing models supported for tariffs.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `tariff_type_code` | `VARCHAR(64)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Optional description |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `FLAT_FEE`, `PER_UNIT`, `TIERED_USAGE`, `STAIRSTEP`, `PACKAGE`, `MATRIX`, `COMPOSITE`.

### `billing_interval`
- **Purpose**: Reference set of standard recurring intervals.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `billing_interval_code` | `VARCHAR(32)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Description of interval |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `DAY`, `WEEK`, `MONTH`, `YEAR`.

### `plan_legacy_price_policy`
- **Purpose**: Determines grandfathering and price migration policies when new plan versions are published.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `legacy_price_policy_code` | `VARCHAR(64)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Policy explanation |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `KEEP_FOREVER`, `MIGRATE_IMMEDIATELY`, `MIGRATE_ON_RENEWAL`.

### `item_type`
- **Purpose**: Categorizes purchasable catalog items.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `item_type_code` | `VARCHAR(64)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Item type details |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `PLAN`, `ONE_TIME_SERVICE`, `PRODUCT`.

### `account_status`
- **Purpose**: Allowed states for a customer account.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `account_status_code` | `VARCHAR(32)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Status details |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `PENDING_VERIFICATION`, `ACTIVE`, `SUSPENDED`, `CLOSED`.

### `payment_provider`
- **Purpose**: Registered payment processing gateways and gateways mock adapters.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `payment_provider_code` | `VARCHAR(50)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Provider name / notes |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `stripe`, `chapa`, `telebirr`, `paypal`, `fake`.

### `payment_type`
- **Purpose**: Classification of payment payment instruments.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `payment_type_code` | `VARCHAR(50)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Instrument description |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `card`, `bank`, `mobile_money`.

### `payment_status`
- **Purpose**: Lifecycle status of stored payment methods.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `payment_status_code` | `VARCHAR(30)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Status notes |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `ACTIVE`, `INACTIVE`, `EXPIRED`, `REVOKED`.

### `subscription_status`
- **Purpose**: Valid subscription states.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `subscription_status_code` | `VARCHAR(32)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Status notes |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `TRIALING`, `ACTIVE`, `PAUSED`, `PAST_DUE`, `CANCELED`, `UNPAID`.

### `invoice_status`
- **Purpose**: Valid states in the invoice lifecycle.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `invoice_status_code` | `VARCHAR(32)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Status notes |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `DRAFT`, `OPEN`, `PAID`, `UNCOLLECTIBLE`, `VOID`.

### `ledger_party_type`
- **Purpose**: Entity classification for credit ledger transactions.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `ledger_party_type_code` | `VARCHAR(32)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Party description |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `ACCOUNT`, `PLATFORM`, `PROVIDER`.

### `ledger_entry_type`
- **Purpose**: Double-entry style credit ledger movement classifications.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `ledger_entry_type_code` | `VARCHAR(40)` | NO | None | **PK** |
  | `description` | `TEXT` | YES | None | Classification details |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Row creation timestamp |
- **Initial Values**: `CREDIT_TRANSFER`, `REFUND`, `PROMOTIONAL`, `MANUAL_ADJUSTMENT`.

---

## 2. Pricing Catalog Tables

### `tariffs`
- **Purpose**: Versioned pricing rate definitions (flat fee, per-unit, or tiered).
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `tariff_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `tariff_code` | `VARCHAR(128)` | NO | None | Natural identifier for grouping versions |
  | `version` | `INT` | NO | `1` | Incrementing version number |
  | `name` | `VARCHAR(255)` | NO | None | Display title |
  | `description` | `TEXT` | YES | None | Detailed pricing terms |
  | `tariff_type_code` | `VARCHAR(64)` | NO | None | **FK** -> `tariff_type.tariff_type_code` |
  | `amount` | `BIGINT` | YES | None | Amount in minor currency units (e.g. cents) |
  | `currency` | `VARCHAR(3)` | YES | None | ISO 4217 three-letter currency code |
  | `tier_brackets` | `JSONB` | YES | None | Serialized tier configurations for tiered pricing |
  | `tier_strategy` | `VARCHAR(32)` | YES | None | Strategy: `VOLUME` or `GRADUATED` |
  | `quantity_unit` | `VARCHAR(32)` | YES | None | Metric unit: `COUNT`, `SEAT`, `GIGABYTE`, `HOUR`, `API_CALL` |
  | `is_active` | `BOOLEAN` | NO | `TRUE` | Soft-disable flag |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Extensible key-value metadata |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |
- **Constraints**:
  - `uq_tariff_code_version`: `UNIQUE (tariff_code, version)`
  - `chk_tariff_amount_currency`: `CHECK ((amount IS NULL AND currency IS NULL) OR (amount IS NOT NULL AND currency IS NOT NULL))`

### `plans`
- **Purpose**: Top-level commercial package bundling recurring durations and migration policies.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `plan_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `plan_code` | `VARCHAR(128)` | NO | None | Natural grouping identifier |
  | `version` | `INT` | NO | `1` | Incrementing version number |
  | `effective_from` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Activation timestamp |
  | `effective_until` | `TIMESTAMPTZ` | YES | None | Sunset timestamp (NULL if active) |
  | `legacy_price_policy_code` | `VARCHAR(64)` | NO | `'KEEP_FOREVER'` | **FK** -> `plan_legacy_price_policy.legacy_price_policy_code` |
  | `migration_path` | `JSONB` | NO | `'{}'::jsonb` | Directed migration path to newer plans |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Custom metadata attributes |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |
- **Constraints**:
  - `uq_plan_code_version`: `UNIQUE (plan_code, version)`
  - `chk_plan_version_positive`: `CHECK (version > 0)`
  - `chk_plan_effective_dates`: `CHECK (effective_until IS NULL OR effective_until > effective_from)`
  - `chk_plan_legacy_price_policy`: `CHECK (legacy_price_policy_code IN ('KEEP_FOREVER', 'MIGRATE_IMMEDIATELY', 'MIGRATE_ON_RENEWAL'))`

### `plan_durations`
- **Purpose**: Defines billing intervals and durations available for a plan, binding each to a specific tariff.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `plan_duration_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `plan_id` | `BIGINT` | NO | None | **FK** -> `plans.plan_id` (`ON DELETE CASCADE`) |
  | `tariff_id` | `BIGINT` | NO | None | **FK** -> `tariffs.tariff_id` |
  | `duration` | `BIGINT` | NO | None | Period in nanoseconds (Go `time.Duration`) |
  | `is_active` | `BOOLEAN` | NO | `TRUE` | Active flag |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |
- **Constraints**:
  - `uq_plan_duration`: `UNIQUE (plan_id, duration)`
  - `chk_plan_duration_tariff_positive`: `CHECK (tariff_id > 0)`

### `purchasable_items`
- **Purpose**: Catalog items that can be invoiced or billed as line items. Plans automatically register a matching item.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `item_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `item_code` | `VARCHAR(128)` | NO | None | Unique sku/item identifier |
  | `item_type_code` | `VARCHAR(64)` | NO | None | **FK** -> `item_type.item_type_code` |
  | `name` | `VARCHAR(255)` | NO | None | Display title |
  | `description` | `TEXT` | YES | None | Item description |
  | `plan_id` | `BIGINT` | YES | None | **FK** -> `plans.plan_id` (`ON DELETE SET NULL`) |
  | `is_active` | `BOOLEAN` | NO | `TRUE` | Active state |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Arbitrary metadata |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |
- **Constraints**:
  - `uq_purchasable_item_code`: `UNIQUE (item_code)`
  - `chk_purchasable_item_type`: `CHECK (item_type_code IN ('PLAN', 'ONE_TIME_SERVICE', 'PRODUCT'))`
  - `chk_purchasable_item_name`: `CHECK (length(trim(name)) > 0)`
  - `chk_purchasable_item_plan_positive`: `CHECK (plan_id IS NULL OR plan_id > 0)`

### `pricing_rules`
- **Purpose**: Rule engine configurations for dynamic discounts or regional modifiers.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `rule_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `rule_code` | `VARCHAR(128)` | NO | None | **UNIQUE** rule code |
  | `is_active` | `BOOLEAN` | NO | `TRUE` | Active flag |
  | `rule_payload` | `JSONB` | NO | None | Evaluated condition and formula payload |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |

### `purchasable_item_pricing_rules`
- **Purpose**: Many-to-many junction between purchasable items and pricing rules.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `item_id` | `BIGINT` | NO | None | **PK, FK** -> `purchasable_items.item_id` (`ON DELETE CASCADE`) |
  | `rule_id` | `BIGINT` | NO | None | **PK, FK** -> `pricing_rules.rule_id` (`ON DELETE CASCADE`) |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Binding timestamp |

---

## 3. Accounts & Payment Methods

### `accounts`
- **Purpose**: Root billing entity representing a tenant or customer.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `account_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `external_id` | `VARCHAR(255)` | YES | None | **UNIQUE** external customer reference |
  | `account_status_code` | `VARCHAR(32)` | NO | `'PENDING_VERIFICATION'` | **FK** -> `account_status.account_status_code` |
  | `currency` | `VARCHAR(3)` | NO | None | ISO 4217 billing currency |
  | `timezone` | `VARCHAR(64)` | NO | None | IANA timezone identifier |
  | `locale` | `VARCHAR(35)` | NO | `'en-US'` | Locale preference |
  | `net_terms` | `SMALLINT` | NO | `0` | Payment term days |
  | `dunning_profile_id` | `BIGINT` | YES | None | Profile ID for retry/dunning policies |
  | `tax_identifiers` | `JSONB` | NO | `'{}'::jsonb` | VAT, TIN, or tax exemptions |
  | `billing_address` | `JSONB` | NO | `'{}'::jsonb` | Address fields |
  | `compliance_flags` | `JSONB` | NO | `'{}'::jsonb` | Sanction/compliance check results |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Custom metadata attributes |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |

### `payment_methods`
- **Purpose**: Saved payment credentials and provider tokenizations linked to an account.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `payment_method_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `account_id` | `BIGINT` | NO | None | **FK** -> `accounts.account_id` |
  | `payment_provider_code` | `VARCHAR(50)` | NO | None | **FK** -> `payment_provider.payment_provider_code` |
  | `provider_reference` | `VARCHAR(255)` | NO | None | Gateway token or payment method ID |
  | `payment_type_code` | `VARCHAR(50)` | NO | None | **FK** -> `payment_type.payment_type_code` |
  | `is_default` | `BOOLEAN` | NO | `FALSE` | Primary payment method indicator |
  | `payment_status_code` | `VARCHAR(30)` | NO | `'ACTIVE'` | **FK** -> `payment_status.payment_status_code` |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Card brand, last4, exp date |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |
- **Indexes & Constraints**:
  - `uq_payment_methods_default_per_account`: `UNIQUE (account_id) WHERE is_default AND payment_status_code = 'ACTIVE'` (enforces exactly one default active payment method per account).
  - `uq_payment_methods_provider_reference`: `UNIQUE (payment_provider_code, provider_reference)`.

---

## 4. Subscriptions

### `subscriptions`
- **Purpose**: Agreement granting ongoing access to a plan, tracking billing periods and renewals.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `subscription_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `version` | `BIGINT` | NO | `1` | Concurrency version |
  | `account_id` | `BIGINT` | NO | None | **FK** -> `accounts.account_id` |
  | `plan_id` | `BIGINT` | NO | None | **FK** -> `plans.plan_id` |
  | `plan_version` | `INT` | NO | `1` | Immutable version of the subscribed plan |
  | `subscription_status_code` | `VARCHAR(32)` | NO | `'TRIALING'` | **FK** -> `subscription_status.subscription_status_code` |
  | `quantity` | `INT` | NO | `1` | Seat or license count |
  | `current_period_start_at` | `TIMESTAMPTZ` | NO | None | Cycle start |
  | `current_period_end_at` | `TIMESTAMPTZ` | NO | None | Cycle end |
  | `billing_cycle_anchor` | `TIMESTAMPTZ` | NO | None | Anchor for recurrence calculations |
  | `trial_start_at` | `TIMESTAMPTZ` | YES | None | Free trial begin |
  | `trial_end_at` | `TIMESTAMPTZ` | YES | None | Free trial end |
  | `cancel_at_period_end` | `BOOLEAN` | NO | `FALSE` | Scheduled cancellation flag |
  | `canceled_at` | `TIMESTAMPTZ` | YES | None | Timestamp when cancellation was requested |
  | `ended_at` | `TIMESTAMPTZ` | YES | None | Timestamp when subscription expired/terminated |
  | `paused_at` | `TIMESTAMPTZ` | YES | None | Timestamp when service was suspended/paused |
  | `resumes_at` | `TIMESTAMPTZ` | YES | None | Scheduled resumption date |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Arbitrary subscription metadata |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Insertion timestamp |
- **Constraints & Indexes**:
  - `chk_subscription_period_order`: `CHECK (current_period_end_at > current_period_start_at)`
  - `idx_subscriptions_account_status`: `INDEX (account_id, subscription_status_code)`

---

## 5. Invoices & Line Items

### `invoices`
- **Purpose**: Financial statement recording charges, payments, and balances.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `invoice_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `account_id` | `BIGINT` | NO | None | **FK** -> `accounts.account_id` |
  | `invoice_number` | `VARCHAR(64)` | YES | None | **UNIQUE** formatted number (`INV-YYYY-XXXXXX`) |
  | `invoice_status_code` | `VARCHAR(32)` | NO | `'DRAFT'` | **FK** -> `invoice_status.invoice_status_code` |
  | `currency` | `VARCHAR(3)` | NO | None | ISO 4217 currency code |
  | `subtotal_amount` | `BIGINT` | NO | `0` | Sum of line items before tax/discount (minor units) |
  | `tax_amount` | `BIGINT` | NO | `0` | Calculated tax in minor units |
  | `discount_amount` | `BIGINT` | NO | `0` | Deductions in minor units |
  | `total_amount` | `BIGINT` | NO | `0` | Net payable amount in minor units |
  | `amount_paid` | `BIGINT` | NO | `0` | Cumulative settled payments in minor units |
  | `amount_due` | `BIGINT` | NO | `0` | Outstanding balance in minor units |
  | `due_at` | `TIMESTAMPTZ` | YES | None | Payment due date |
  | `finalized_at` | `TIMESTAMPTZ` | YES | None | Timestamp when transitioned from DRAFT to OPEN |
  | `paid_at` | `TIMESTAMPTZ` | YES | None | Timestamp when balance reached zero |
  | `billing_address_snapshot`| `JSONB` | NO | `'{}'::jsonb` | Immutable copy of address at finalization |
  | `idempotency_key` | `VARCHAR(255)` | YES | None | **UNIQUE** request idempotency key |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Arbitrary invoice metadata |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Creation timestamp |
- **Note on Concurrency**: Migration `0002` added a `version` column, which was subsequently dropped in migration `0010`. Concurrency control for invoice lifecycles is handled by `sm_instances.version`.

### `invoice_line_items`
- **Purpose**: Individual charge entries itemized on an invoice.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `line_item_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `invoice_id` | `BIGINT` | NO | None | **FK** -> `invoices.invoice_id` (`ON DELETE CASCADE`) |
  | `item_id` | `BIGINT` | NO | None | **FK** -> `purchasable_items.item_id` |
  | `subscription_id` | `BIGINT` | YES | None | **FK** -> `subscriptions.subscription_id` (`ON DELETE SET NULL`) |
  | `description` | `TEXT` | NO | None | Line item narrative |
  | `quantity_value` | `NUMERIC(18, 4)` | NO | `1.0000` | Units or consumption volume |
  | `quantity_unit` | `VARCHAR(32)` | NO | `'UNIT'` | Unit of measure |
  | `unit_amount` | `BIGINT` | NO | None | Minor unit price per unit |
  | `total_amount` | `BIGINT` | NO | None | Extended charge amount in minor units |
  | `metadata` | `JSONB` | NO | `'{}'::jsonb` | Arbitrary line item metadata |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Creation timestamp |

---

## 6. Credit Ledger

### `credit_ledger`
- **Purpose**: Immutable ledger recording credits, promotional funds, manual adjustments, and balances.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `entry_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `source_party_type_code` | `VARCHAR(32)` | NO | None | **FK** -> `ledger_party_type.ledger_party_type_code` |
  | `source_party_id` | `VARCHAR(255)` | NO | None | Source identifier (e.g. Account ID, Platform) |
  | `destination_party_type_code` | `VARCHAR(32)` | NO | None | **FK** -> `ledger_party_type.ledger_party_type_code` |
  | `destination_party_id` | `VARCHAR(255)` | NO | None | Destination identifier |
  | `ledger_entry_type_code` | `VARCHAR(40)` | NO | None | **FK** -> `ledger_entry_type.ledger_entry_type_code` |
  | `amount` | `BIGINT` | NO | None | Credit value in minor units |
  | `reference_type` | `VARCHAR(50)` | YES | None | External reference domain (e.g. INVOICE) |
  | `reference_id` | `VARCHAR(255)` | YES | None | External reference ID |
  | `description` | `TEXT` | YES | None | Audit memo |
  | `expires_at` | `TIMESTAMPTZ` | YES | None | Credit expiration timestamp |
  | `created_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Journal entry timestamp |
- **Constraints**:
  - `amount > 0` (`CHECK (amount > 0)`)

---

## 7. Event Store

### `event_log`
- **Purpose**: Append-only system event ledger enabling event sourcing and auditing.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `event_id` | `UUID` | NO | `gen_random_uuid()` | **PK** |
  | `event_type` | `VARCHAR(128)` | NO | None | Domain event name (e.g. `billing.invoice.paid`) |
  | `event_version` | `INT` | NO | `1` | Schema version of payload |
  | `aggregate_type`| `VARCHAR(64)` | NO | None | Domain aggregate (`ACCOUNT`, `SUBSCRIPTION`, `INVOICE`) |
  | `aggregate_id` | `TEXT` | NO | None | Identifier of aggregate root (migrated from UUID) |
  | `sequence` | `BIGINT` | NO | None | Strictly incrementing sequence per aggregate |
  | `occurred_at` | `TIMESTAMPTZ` | NO | `CURRENT_TIMESTAMP` | Event timestamp |
  | `actor` | `JSONB` | NO | None | Initiator: `{ "id": "...", "type": "USER|SYSTEM" }` |
  | `causation_id` | `UUID` | YES | None | **FK** -> `event_log.event_id` (`ON DELETE SET NULL`) |
  | `correlation_id`| `UUID` | YES | None | Trace correlation across distributed flows |
  | `payload` | `JSONB` | NO | `'{}'::jsonb` | Domain event payload |
- **Constraints**:
  - `uq_aggregate_sequence`: `UNIQUE (aggregate_type, aggregate_id, sequence)`
  - `chk_event_version_positive`: `CHECK (event_version > 0)`

---

## 8. Payment Attempts & PPI Webhooks

### `payment_attempts`
- **Purpose**: Tracks payment attempts against an open invoice, isolating gateway charges and states.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `attempt_id` | `BIGINT` | NO | `IDENTITY` | **PK** |
  | `invoice_id` | `BIGINT` | NO | None | **FK** -> `invoices.invoice_id` (`ON DELETE RESTRICT`) |
  | `provider_code` | `VARCHAR(25)` | NO | None | **FK** -> `payment_provider.payment_provider_code` |
  | `internal_tx_id`| `VARCHAR(255)` | NO | None | Idempotency key generated by billing service |
  | `provider_tx_id`| `VARCHAR(255)` | YES | None | External transaction ID returned by provider |
  | `amount_minor` | `BIGINT` | NO | None | Charge amount in minor units |
  | `currency` | `VARCHAR(3)` | NO | None | ISO 4217 currency |
  | `status` | `VARCHAR(25)` | NO | `'PENDING'` | `PENDING`, `SUCCESS`, `FAILED`, `REFUNDED` |
  | `raw_response` | `JSONB` | YES | None | Verbatim provider response payload |
  | `raw_request` | `JSONB` | YES | None | Request payload dispatched to provider |
  | `created_at` | `TIMESTAMPTZ` | NO | `NOW()` | Attempt timestamp |
  | `updated_at` | `TIMESTAMPTZ` | NO | `NOW()` | Last state update timestamp |
- **Indexes**:
  - `idx_payment_attempts_single_pending_per_invoice`: `UNIQUE INDEX (invoice_id) WHERE status = 'PENDING'` (guarantees at most one pending charge per invoice).

### `ppi_webhooks`
- **Purpose**: Ingest log and idempotency register for inbound payment provider webhook notifications.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `webhook_id` | `VARCHAR(64)` | NO | None | **PK** |
  | `provider_code` | `VARCHAR(25)` | NO | None | **FK** -> `payment_provider.payment_provider_code` |
  | `event_type` | `VARCHAR(64)` | NO | None | Gateway-specific event type |
  | `internal_tx_id`| `VARCHAR(255)` | YES | None | Internal transaction correlation key |
  | `provider_tx_id`| `VARCHAR(255)` | YES | None | Gateway transaction reference |
  | `status` | `VARCHAR(32)` | NO | None | Status (`SUCCEEDED`, `FAILED`, `PENDING`) |
  | `payload` | `JSONB` | NO | None | Raw webhook payload |
  | `published_at` | `TIMESTAMPTZ` | YES | None | Timestamp published to RabbitMQ |
  | `processed_at` | `TIMESTAMPTZ` | NO | `NOW()` | Timestamp received and processed |
- **Indexes**:
  - `idx_ppi_webhooks_provider_tx`: `INDEX (provider_code, provider_tx_id)`
  - `idx_ppi_webhooks_internal_tx`: `INDEX (internal_tx_id)`
  - `idx_ppi_webhooks_unpublished`: `INDEX (processed_at) WHERE published_at IS NULL`

---

## 9. State Machine Engine Tables

### `sm_definitions`
- **Purpose**: Master registry of compiled state machine types and versions.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `definition_id` | `UUID` | NO | `gen_random_uuid()` | **PK** |
  | `machine_type` | `VARCHAR(64)` | NO | None | State machine name (e.g. `invoice_lifecycle`) |
  | `version` | `INT` | NO | None | Incrementing version number |
  | `initial_state`| `VARCHAR(64)` | NO | None | State name when instance is initialized |
  | `is_active` | `BOOLEAN` | NO | `false` | True if this version accepts new instances |
  | `created_at` | `TIMESTAMPTZ` | NO | `now()` | Definition creation timestamp |
- **Constraints & Indexes**:
  - `UNIQUE (machine_type, version)`
  - `idx_sm_definitions_active_per_type`: `UNIQUE INDEX (machine_type) WHERE is_active` (at most one active version per type).

### `sm_states`
- **Purpose**: Nodes in a state machine definition graph.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `definition_id` | `UUID` | NO | None | **PK, FK** -> `sm_definitions.definition_id` (`ON DELETE CASCADE`) |
  | `state_name` | `VARCHAR(64)` | NO | None | **PK** state identifier |
  | `is_final` | `BOOLEAN` | NO | `false` | Terminal state flag |
  | `metadata` | `JSONB` | NO | `'{}'` | Custom state annotations |

### `sm_transitions`
- **Purpose**: Directed edges in a state machine definition graph.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `transition_id` | `UUID` | NO | `gen_random_uuid()` | **PK** |
  | `definition_id` | `UUID` | NO | None | **FK** -> `sm_definitions.definition_id` (`ON DELETE CASCADE`) |
  | `from_state` | `VARCHAR(64)` | NO | None | **FK** -> `sm_states(definition_id, state_name)` |
  | `event_name` | `VARCHAR(64)` | NO | None | Trigger event identifier |
  | `to_state` | `VARCHAR(64)` | NO | None | **FK** -> `sm_states(definition_id, state_name)` |
  | `priority` | `INT` | NO | `0` | Order of transition evaluation |
  | `emit_event_type`| `VARCHAR(128)`| YES | None | Domain event type emitted upon transition |
- **Constraints & Indexes**:
  - `UNIQUE (definition_id, from_state, event_name, priority)`
  - `idx_sm_transitions_lookup`: `INDEX (definition_id, from_state, event_name, priority)`

### `sm_guard_bindings`
- **Purpose**: Pre-condition predicates evaluated before a transition fires.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `transition_id` | `UUID` | NO | None | **PK, FK** -> `sm_transitions.transition_id` (`ON DELETE CASCADE`) |
  | `seq` | `INT` | NO | None | **PK** evaluation order sequence |
  | `implementation_kind`| `VARCHAR(12)`| NO | `'GO_REGISTRY'`| `GO_REGISTRY` or `SCRIPT` |
  | `guard_name` | `VARCHAR(128)` | YES | None | Registered Go guard identifier |
  | `script` | `TEXT` | YES | None | Yaegi interpreted Go script source |
  | `params` | `JSONB` | NO | `'{}'` | Guard evaluation parameters |

### `sm_action_bindings`
- **Purpose**: Side-effect operations executed on state entry, state exit, or transition.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `binding_id` | `UUID` | NO | `gen_random_uuid()` | **PK** |
  | `definition_id` | `UUID` | NO | None | **FK** -> `sm_definitions.definition_id` (`ON DELETE CASCADE`) |
  | `hook_type` | `VARCHAR(20)` | NO | None | `ON_ENTER`, `ON_EXIT`, or `ON_TRANSITION` |
  | `state_name` | `VARCHAR(64)` | YES | None | **FK** -> `sm_states(definition_id, state_name)` |
  | `transition_id` | `UUID` | YES | None | **FK** -> `sm_transitions.transition_id` (`ON DELETE CASCADE`) |
  | `seq` | `INT` | NO | None | Action execution order |
  | `action_name` | `VARCHAR(128)` | NO | None | Registered Go action name |
  | `params_kind` | `VARCHAR(12)` | NO | `'STATIC'` | `STATIC` or `SCRIPT` |
  | `params` | `JSONB` | YES | None | Static parameter payload |
  | `params_script`| `TEXT` | YES | None | Dynamic parameter script |
  | `mode` | `VARCHAR(10)` | NO | None | `SYNC` (inline in transaction) or `ASYNC` (outbox) |
  | `on_error` | `VARCHAR(10)` | NO | `'ABORT'` | `ABORT` or `CONTINUE` |
- **Indexes**:
  - `idx_sm_action_bindings_state_hook`: `INDEX (definition_id, state_name, hook_type, seq) WHERE state_name IS NOT NULL`
  - `idx_sm_action_bindings_transition_hook`: `INDEX (transition_id, hook_type, seq) WHERE transition_id IS NOT NULL`

### `sm_instances`
- **Purpose**: State of an active or completed subject execution instance.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `instance_id` | `UUID` | NO | `gen_random_uuid()` | **PK** |
  | `definition_id` | `UUID` | NO | None | **FK** -> `sm_definitions.definition_id` (`ON DELETE RESTRICT`) |
  | `machine_type` | `VARCHAR(64)` | NO | None | Name of state machine |
  | `subject_type` | `VARCHAR(64)` | NO | None | Domain entity type (`invoice`, `subscription`) |
  | `subject_id` | `VARCHAR(128)` | NO | None | Domain entity ID string |
  | `current_state` | `VARCHAR(64)` | NO | None | **FK** -> `sm_states(definition_id, state_name)` |
  | `context` | `JSONB` | NO | `'{}'` | Mutable state machine working memory |
  | `version` | `BIGINT` | NO | `0` | Optimistic locking counter |
  | `status` | `VARCHAR(12)` | NO | `'RUNNING'` | `RUNNING` or `COMPLETED` |
  | `created_at` | `TIMESTAMPTZ` | NO | `now()` | Initial state creation |
  | `updated_at` | `TIMESTAMPTZ` | NO | `now()` | Last transition timestamp |
- **Constraints & Indexes**:
  - `UNIQUE (subject_type, subject_id, machine_type)`
  - `idx_sm_instances_subject`: `INDEX (subject_type, subject_id)`

### `sm_transition_history`
- **Purpose**: Historical audit log of every state machine transition executed.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `history_id` | `BIGSERIAL` | NO | `AUTO` | **PK** |
  | `instance_id` | `UUID` | NO | None | **FK** -> `sm_instances.instance_id` (`ON DELETE CASCADE`) |
  | `transition_id` | `UUID` | YES | None | **FK** -> `sm_transitions.transition_id` |
  | `from_state` | `VARCHAR(64)` | YES | None | Pre-transition state |
  | `to_state` | `VARCHAR(64)` | NO | None | Post-transition state |
  | `event_name` | `VARCHAR(64)` | YES | None | Fired event |
  | `event_payload` | `JSONB` | YES | None | Triggering payload |
  | `context_before`| `JSONB` | YES | None | Instance context snapshot before transition |
  | `context_after` | `JSONB` | YES | None | Instance context snapshot after transition |
  | `triggered_by` | `VARCHAR(128)` | YES | None | Actor/worker identity |
  | `occurred_at` | `TIMESTAMPTZ` | NO | `now()` | Timestamp of transition |
- **Indexes**:
  - `idx_sm_transition_history_instance`: `INDEX (instance_id, occurred_at)`

### `sm_scheduled_transitions`
- **Purpose**: Durable timers for delayed/scheduled transitions (e.g. trial expiration).
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `schedule_id` | `UUID` | NO | `gen_random_uuid()` | **PK** |
  | `instance_id` | `UUID` | NO | None | **FK** -> `sm_instances.instance_id` (`ON DELETE CASCADE`) |
  | `event_name` | `VARCHAR(64)` | NO | None | Event to fire upon deadline |
  | `event_payload` | `JSONB` | YES | None | Payload to supply with event |
  | `fire_at` | `TIMESTAMPTZ` | NO | None | Deadline timestamp |
  | `status` | `VARCHAR(12)` | NO | `'PENDING'` | `PENDING`, `PROCESSING`, `FIRED`, `CANCELED`, `FAILED` |
  | `attempts` | `INT` | NO | `0` | Execution attempt count |
  | `claimed_at` | `TIMESTAMPTZ` | YES | None | Lease timestamp when claimed by worker |
  | `claimed_by` | `VARCHAR(128)` | YES | None | Worker node identifier |
  | `last_error` | `TEXT` | YES | None | Last failure error message |
  | `created_at` | `TIMESTAMPTZ` | NO | `now()` | Creation timestamp |
- **Indexes**:
  - `idx_sm_scheduled_transitions_pending`: `INDEX (fire_at) WHERE status = 'PENDING'`
  - `idx_sm_scheduled_transitions_processing`: `INDEX (claimed_at) WHERE status = 'PROCESSING'`

### `sm_action_outbox`
- **Purpose**: Transactional outbox queue for ASYNC action side-effects.
- **Columns**:
  | Column | Type | Nullable | Default | Constraints / Description |
  | :--- | :--- | :--- | :--- | :--- |
  | `outbox_id` | `BIGSERIAL` | NO | `AUTO` | **PK** |
  | `instance_id` | `UUID` | NO | None | **FK** -> `sm_instances.instance_id` (`ON DELETE CASCADE`) |
  | `history_id` | `BIGINT` | YES | None | **FK** -> `sm_transition_history.history_id` |
  | `binding_id` | `UUID` | YES | None | **FK** -> `sm_action_bindings.binding_id` |
  | `action_name` | `VARCHAR(128)` | NO | None | Registered Go action name |
  | `params` | `JSONB` | NO | None | Serialized execution parameters |
  | `status` | `VARCHAR(12)` | NO | `'PENDING'` | `PENDING`, `PROCESSING`, `PUBLISHED`, `FAILED` |
  | `attempts` | `INT` | NO | `0` | Processing attempts |
  | `claimed_at` | `TIMESTAMPTZ` | YES | None | Lease timestamp |
  | `claimed_by` | `VARCHAR(128)` | YES | None | Worker identity |
  | `last_error` | `TEXT` | YES | None | Last failure error message |
  | `created_at` | `TIMESTAMPTZ` | NO | `now()` | Queue creation timestamp |
- **Indexes**:
  - `idx_sm_action_outbox_pending`: `INDEX (created_at) WHERE status = 'PENDING'`
  - `idx_sm_action_outbox_processing`: `INDEX (claimed_at) WHERE status = 'PROCESSING'`
