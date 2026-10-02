# Database Indexes & Constraints

This document catalogs all uniqueness constraints, foreign key referential integrity rules, check constraints, and performance indexes implemented in the database.

---

## 1. Indexes Catalog

### Partial & Specialized Indexes

The system makes extensive use of PostgreSQL partial indexes to enforce business invariants without table-level unique constraints.

| Index Name | Table | Columns | Condition | Invariant Enforced |
| :--- | :--- | :--- | :--- | :--- |
| `idx_payment_attempts_single_pending_per_invoice` | `payment_attempts` | `(invoice_id)` | `WHERE status = 'PENDING'` | **Strictly one pending payment attempt per invoice at any given time**. Prevents duplicate checkout races. |
| `uq_payment_methods_default_per_account` | `payment_methods` | `(account_id)` | `WHERE is_default AND payment_status_code = 'ACTIVE'` | **At most one active default payment method per account**. |
| `uq_payment_methods_provider_reference` | `payment_methods` | `(payment_provider_code, provider_reference)` | None | Eliminates duplicate provider tokens per gateway. |
| `idx_sm_definitions_active_per_type` | `sm_definitions` | `(machine_type)` | `WHERE is_active` | **Only one active definition version per machine type** at a time. |
| `idx_ppi_webhooks_unpublished` | `ppi_webhooks` | `(processed_at)` | `WHERE published_at IS NULL` | High-speed indexing for background workers polling unpublished webhooks. |
| `idx_sm_scheduled_transitions_pending` | `sm_scheduled_transitions` | `(fire_at)` | `WHERE status = 'PENDING'` | Hot-path index for the scheduler poller scanning due transitions. |
| `idx_sm_scheduled_transitions_processing` | `sm_scheduled_transitions` | `(claimed_at)` | `WHERE status = 'PROCESSING'` | For reaping stalled/crashed transition claims. |
| `idx_sm_action_outbox_pending` | `sm_action_outbox` | `(created_at)` | `WHERE status = 'PENDING'` | Hot-path index for outbox publisher polling pending side-effects. |
| `idx_sm_action_outbox_processing` | `sm_action_outbox` | `(claimed_at)` | `WHERE status = 'PROCESSING'` | For reaping stalled outbox workers. |
| `idx_sm_action_bindings_state_hook` | `sm_action_bindings` | `(definition_id, state_name, hook_type, seq)` | `WHERE state_name IS NOT NULL` | Fast evaluation of state entry/exit action chains. |
| `idx_sm_action_bindings_transition_hook` | `sm_action_bindings` | `(transition_id, hook_type, seq)` | `WHERE transition_id IS NOT NULL` | Fast evaluation of transition action chains. |

### Query Performance Indexes

| Index Name | Table | Columns | Purpose |
| :--- | :--- | :--- | :--- |
| `idx_subscriptions_account_status` | `subscriptions` | `(account_id, subscription_status_code)` | Accelerated filtering of active subscriptions by account. |
| `idx_ppi_webhooks_provider_tx` | `ppi_webhooks` | `(provider_code, provider_tx_id)` | Webhook deduplication lookups by gateway reference. |
| `idx_ppi_webhooks_internal_tx` | `ppi_webhooks` | `(internal_tx_id)` | Webhook correlation with internal transaction IDs. |
| `idx_sm_transitions_lookup` | `sm_transitions` | `(definition_id, from_state, event_name, priority)` | Candidate transition lookup in FSM `Fire()` hot path. |
| `idx_sm_instances_subject` | `sm_instances` | `(subject_type, subject_id)` | Resolving FSM instance for an invoice or subscription. |
| `idx_sm_transition_history_instance`| `sm_transition_history` | `(instance_id, occurred_at)` | Transition audit trail retrieval ordered by time. |

---

## 2. Check Constraints

| Table | Constraint Name | Clause | Business Rule Enforced |
| :--- | :--- | :--- | :--- |
| `tariffs` | `chk_tariff_amount_currency` | `((amount IS NULL AND currency IS NULL) OR (amount IS NOT NULL AND currency IS NOT NULL))` | Tariffs with an amount must specify currency; tiered tariffs without base amounts can leave both null. |
| `plans` | `chk_plan_version_positive` | `version > 0` | Version numbers must start from 1. |
| `plans` | `chk_plan_effective_dates` | `effective_until IS NULL OR effective_until > effective_from` | Sunset date must follow activation date. |
| `plans` | `chk_plan_legacy_price_policy`| `legacy_price_policy_code IN ('KEEP_FOREVER', 'MIGRATE_IMMEDIATELY', 'MIGRATE_ON_RENEWAL')` | Allowed price migration strategies. |
| `plan_durations` | `chk_plan_duration_tariff_positive` | `tariff_id > 0` | Foreign identifier validity. |
| `purchasable_items` | `chk_purchasable_item_type` | `item_type_code IN ('PLAN', 'ONE_TIME_SERVICE', 'PRODUCT')` | Valid item classifications. |
| `purchasable_items` | `chk_purchasable_item_name` | `length(trim(name)) > 0` | Non-blank item names required. |
| `purchasable_items` | `chk_purchasable_item_plan_positive` | `plan_id IS NULL OR plan_id > 0` | Positive plan link when item represents a plan. |
| `subscriptions` | `chk_subscription_period_order` | `current_period_end_at > current_period_start_at` | Cycle end date must be strictly after cycle start date. |
| `credit_ledger` | *(inline check)* | `amount > 0` | Credit adjustments must represent positive magnitudes. |
| `event_log` | `chk_event_version_positive` | `event_version > 0` | Event schema versions must be >= 1. |
| `sm_guard_bindings` | *(inline check)* | `(implementation_kind = 'GO_REGISTRY' AND guard_name IS NOT NULL AND script IS NULL) OR (implementation_kind = 'SCRIPT' AND script IS NOT NULL AND guard_name IS NULL)` | Strict mutual exclusivity between Go registry guards and Yaegi scripts. |
| `sm_action_bindings` | *(inline check)* | `(hook_type IN ('ON_ENTER', 'ON_EXIT') AND state_name IS NOT NULL AND transition_id IS NULL) OR (hook_type = 'ON_TRANSITION' AND transition_id IS NOT NULL AND state_name IS NULL)` | Hook targeting consistency: state hooks attach to states, transition hooks attach to transitions. |
| `sm_action_bindings` | *(inline check)* | `(params_kind = 'STATIC' AND params_script IS NULL) OR (params_kind = 'SCRIPT' AND params_script IS NOT NULL)` | Static params cannot specify a script; script params must provide script source. |
| `sm_instances` | *(inline check)* | `status IN ('RUNNING', 'COMPLETED')` | FSM instance lifecycle states. |
| `sm_scheduled_transitions` | *(inline check)* | `status IN ('PENDING', 'PROCESSING', 'FIRED', 'CANCELED', 'FAILED')` | Timer queue states. |
| `sm_action_outbox` | *(inline check)* | `status IN ('PENDING', 'PROCESSING', 'PUBLISHED', 'FAILED')` | Outbox side-effect execution lifecycle. |

---

## 3. Referential Integrity (Foreign Key Deletion Rules)

- `purchasable_items.plan_id`: `ON DELETE SET NULL` (preserving invoice line item catalog history even if a draft plan is removed).
- `invoice_line_items.invoice_id`: `ON DELETE CASCADE` (deleting an invoice cascade-purges its line items).
- `invoice_line_items.subscription_id`: `ON DELETE SET NULL` (preserving line items if subscription is purged).
- `event_log.causation_id`: `ON DELETE SET NULL` (event chain links preserved if parent event is truncated).
- `payment_attempts.invoice_id`: `ON DELETE RESTRICT` (**Invoices with payment attempts cannot be deleted**, guaranteeing audit compliance).
- `sm_states`, `sm_transitions`, `sm_action_bindings`: `ON DELETE CASCADE` with `sm_definitions`.
- `sm_instances.definition_id`: `ON DELETE RESTRICT` (A definition cannot be deleted while live or historical instances exist).
