# Database Entity-Relationship Diagrams (ERD)

This section maps out the physical data models and relational structures across the system.

---

## 1. Core Billing Domain

The primary operational model covers customer accounts, payment methods, the pricing catalog (tariffs, plans, purchasable items), subscriptions, invoices, and the credit ledger.

```mermaid
erDiagram
    account_status ||--o{ accounts : "references"
    accounts ||--o{ payment_methods : "has many"
    payment_provider ||--o{ payment_methods : "processed by"
    payment_type ||--o{ payment_methods : "classified by"
    payment_status ||--o{ payment_methods : "status"

    plan_legacy_price_policy ||--o{ plans : "references"
    tariff_type ||--o{ tariffs : "categorized by"
    
    plans ||--o{ plan_durations : "configured with"
    tariffs ||--o{ plan_durations : "priced by"

    item_type ||--o{ purchasable_items : "type"
    plans ||--o| purchasable_items : "represents"
    
    pricing_rules ||--o{ purchasable_item_pricing_rules : "binds"
    purchasable_items ||--o{ purchasable_item_pricing_rules : "associated"

    accounts ||--o{ subscriptions : "owns"
    plans ||--o{ subscriptions : "subscribed to"
    subscription_status ||--o{ subscriptions : "status"

    accounts ||--o{ invoices : "billed to"
    invoice_status ||--o{ invoices : "status"
    invoices ||--o{ invoice_line_items : "contains"
    purchasable_items ||--o{ invoice_line_items : "charges for"
    subscriptions ||--o{ invoice_line_items : "originated from"

    ledger_party_type ||--o{ credit_ledger : "source party"
    ledger_party_type ||--o{ credit_ledger : "destination party"
    ledger_entry_type ||--o{ credit_ledger : "entry classification"

    accounts {
        bigint account_id PK
        varchar external_id UK
        varchar account_status_code FK
        varchar currency
        varchar timezone
        varchar locale
        smallint net_terms
        bigint dunning_profile_id
        jsonb tax_identifiers
        jsonb billing_address
        jsonb compliance_flags
        jsonb metadata
        timestamptz created_at
    }

    payment_methods {
        bigint payment_method_id PK
        bigint account_id FK
        varchar payment_provider_code FK
        varchar provider_reference
        varchar payment_type_code FK
        boolean is_default
        varchar payment_status_code FK
        jsonb metadata
        timestamptz created_at
    }

    tariffs {
        bigint tariff_id PK
        varchar tariff_code
        int version
        varchar name
        text description
        varchar tariff_type_code FK
        varchar tier_strategy
        varchar quantity_unit
        bigint amount
        varchar currency
        jsonb tier_brackets
        boolean is_active
        jsonb metadata
        timestamptz created_at
    }

    plans {
        bigint plan_id PK
        varchar plan_code
        int version
        timestamptz effective_from
        timestamptz effective_until
        varchar legacy_price_policy_code FK
        jsonb migration_path
        jsonb metadata
        timestamptz created_at
    }

    plan_durations {
        bigint plan_duration_id PK
        bigint plan_id FK
        bigint tariff_id FK
        bigint duration
        boolean is_active
        timestamptz created_at
    }

    purchasable_items {
        bigint item_id PK
        varchar item_code UK
        varchar item_type_code FK
        varchar name
        text description
        bigint plan_id FK
        boolean is_active
        jsonb metadata
        timestamptz created_at
    }

    subscriptions {
        bigint subscription_id PK
        bigint version
        bigint account_id FK
        bigint plan_id FK
        int plan_version
        varchar subscription_status_code FK
        int quantity
        timestamptz current_period_start_at
        timestamptz current_period_end_at
        timestamptz billing_cycle_anchor
        timestamptz trial_start_at
        timestamptz trial_end_at
        boolean cancel_at_period_end
        timestamptz canceled_at
        timestamptz ended_at
        timestamptz paused_at
        timestamptz resumes_at
        jsonb metadata
        timestamptz created_at
    }

    invoices {
        bigint invoice_id PK
        bigint account_id FK
        varchar invoice_number UK
        varchar invoice_status_code FK
        varchar currency
        bigint subtotal_amount
        bigint tax_amount
        bigint discount_amount
        bigint total_amount
        bigint amount_paid
        bigint amount_due
        timestamptz due_at
        timestamptz finalized_at
        timestamptz paid_at
        jsonb billing_address_snapshot
        varchar idempotency_key UK
        jsonb metadata
        timestamptz created_at
    }

    invoice_line_items {
        bigint line_item_id PK
        bigint invoice_id FK
        bigint item_id FK
        bigint subscription_id FK
        text description
        numeric quantity_value
        varchar quantity_unit
        bigint unit_amount
        bigint total_amount
        jsonb metadata
        timestamptz created_at
    }

    credit_ledger {
        bigint entry_id PK
        varchar source_party_type_code FK
        varchar source_party_id
        varchar destination_party_type_code FK
        varchar destination_party_id
        varchar ledger_entry_type_code FK
        bigint amount
        varchar reference_type
        varchar reference_id
        text description
        timestamptz expires_at
        timestamptz created_at
    }
```

---

## 2. Payments & Webhooks Processing

The payment interface layer isolates external gateway interactions, partial payment attempts, and webhook audit trails.

```mermaid
erDiagram
    invoices ||--o{ payment_attempts : "attempts payment for"
    payment_provider ||--o{ payment_attempts : "processed by"
    payment_provider ||--o{ ppi_webhooks : "received from"

    payment_attempts {
        bigint attempt_id PK
        bigint invoice_id FK
        varchar provider_code FK
        varchar internal_tx_id
        varchar provider_tx_id
        bigint amount_minor
        varchar currency
        varchar status
        jsonb raw_response
        jsonb raw_request
        timestamptz created_at
        timestamptz updated_at
    }

    ppi_webhooks {
        varchar webhook_id PK
        varchar provider_code FK
        varchar event_type
        varchar internal_tx_id
        varchar provider_tx_id
        varchar status
        jsonb payload
        timestamptz published_at
        timestamptz processed_at
    }
```

---

## 3. Dynamic State Machine Engine

The generic state machine framework manages lifecycle execution for domains such as `invoice_lifecycle` and `subscription_lifecycle`.

```mermaid
erDiagram
    sm_definitions ||--o{ sm_states : "defines"
    sm_definitions ||--o{ sm_transitions : "defines"
    sm_definitions ||--o{ sm_action_bindings : "attaches"
    sm_definitions ||--o{ sm_instances : "instantiates"

    sm_states ||--o{ sm_transitions : "from_state"
    sm_states ||--o{ sm_transitions : "to_state"
    sm_states ||--o{ sm_action_bindings : "hook on state"
    sm_states ||--o{ sm_instances : "current_state"

    sm_transitions ||--o{ sm_guard_bindings : "guarded by"
    sm_transitions ||--o{ sm_action_bindings : "hook on transition"
    sm_transitions ||--o{ sm_transition_history : "records"

    sm_instances ||--o{ sm_transition_history : "logs transitions"
    sm_instances ||--o{ sm_scheduled_transitions : "schedules"
    sm_instances ||--o{ sm_action_outbox : "queues side effects"

    sm_definitions {
        uuid definition_id PK
        varchar machine_type
        int version
        varchar initial_state
        boolean is_active
        timestamptz created_at
    }

    sm_states {
        uuid definition_id PK,FK
        varchar state_name PK
        boolean is_final
        jsonb metadata
    }

    sm_transitions {
        uuid transition_id PK
        uuid definition_id FK
        varchar from_state FK
        varchar event_name
        varchar to_state FK
        int priority
        varchar emit_event_type
    }

    sm_guard_bindings {
        uuid transition_id PK,FK
        int seq PK
        varchar implementation_kind
        varchar guard_name
        text script
        jsonb params
    }

    sm_action_bindings {
        uuid binding_id PK
        uuid definition_id FK
        varchar hook_type
        varchar state_name FK
        uuid transition_id FK
        int seq
        varchar action_name
        varchar params_kind
        jsonb params
        text params_script
        varchar mode
        varchar on_error
    }

    sm_instances {
        uuid instance_id PK
        uuid definition_id FK
        varchar machine_type
        varchar subject_type
        varchar subject_id
        varchar current_state FK
        jsonb context
        bigint version
        varchar status
        timestamptz created_at
        timestamptz updated_at
    }

    sm_transition_history {
        bigserial history_id PK
        uuid instance_id FK
        uuid transition_id FK
        varchar from_state
        varchar to_state
        varchar event_name
        jsonb event_payload
        jsonb context_before
        jsonb context_after
        varchar triggered_by
        timestamptz occurred_at
    }

    sm_scheduled_transitions {
        uuid schedule_id PK
        uuid instance_id FK
        varchar event_name
        jsonb event_payload
        timestamptz fire_at
        varchar status
        int attempts
        timestamptz claimed_at
        varchar claimed_by
        text last_error
        timestamptz created_at
    }

    sm_action_outbox {
        bigserial outbox_id PK
        uuid instance_id FK
        bigint history_id FK
        uuid binding_id FK
        varchar action_name
        jsonb params
        varchar status
        int attempts
        timestamptz claimed_at
        varchar claimed_by
        text last_error
        timestamptz created_at
    }
```

---

## 4. Append-Only Event Store

The event ledger records historical state mutations.

```mermaid
erDiagram
    event_log ||--o{ event_log : "causation_id references event_id"

    event_log {
        uuid event_id PK
        varchar event_type
        int event_version
        varchar aggregate_type
        text aggregate_id
        bigint sequence
        jsonb actor
        timestamptz occurred_at
        uuid causation_id FK
        uuid correlation_id
        jsonb payload
    }
```
