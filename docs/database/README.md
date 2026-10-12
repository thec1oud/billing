# Database Overview & Conventions

The Gebeta Billing database runs on PostgreSQL 16. It combines relational billing primitives with an append-only event log, transactional outboxes, and a schema-backed finite state machine (FSM).

---

## Design Principles

1. **Monetary Precision**:
   - All financial figures (`amount`, `subtotal_amount`, `tax_amount`, `discount_amount`, `total_amount`, `amount_paid`, `amount_due`, `unit_amount`) are stored as signed 64-bit integers (`BIGINT`) representing minor currency units (e.g., USD cents, ETB cents).
   - No floating-point or standard decimal columns are used for money arithmetic, avoiding rounding errors.

2. **Immutable Pricing & Versioning**:
   - Both `tariffs` and `plans` are versioned append-only entities.
   - Updates never overwrite previous pricing; instead, new versions are created with an incremented `version` column.
   - PostgreSQL transaction-level advisory locks (`pg_advisory_xact_lock`) are acquired on hash representations of codes to prevent race conditions during version calculation.

3. **Event Sourcing & Auditing**:
   - Critical lifecycle changes (account creation/activation, subscription creation, invoice creation/finalization/payment) write immutable audit records to the `event_log` table.
   - The sequence is strictly monotonic per aggregate via `(aggregate_type, aggregate_id, sequence)` uniqueness.

4. **Dynamic State Machine Engine**:
   - Complex workflows (invoice lifecycle and subscription lifecycle) are orchestrated through 9 specialized database tables prefixed with `sm_`.
   - State definitions, transitions, guards, actions, execution instances, history, and scheduled timeouts are stored directly in PostgreSQL, with an outbox pattern (`sm_action_outbox`) for asynchronous side effects.

5. **Reference Lookup Normalization**:
   - Enums and system categories are modeled as primary-keyed lookup tables (e.g. `tariff_type`, `account_status`, `invoice_status`) referenced via foreign keys, providing runtime extensibility without DDL alterations.

---

## Global Sequences

- **`invoice_number_seq`**:
  - Purpose: Generates sequential invoice identifiers upon finalization.
  - Definition: `CREATE SEQUENCE invoice_number_seq START WITH 1 INCREMENT BY 1 NO CYCLE;`
  - Format: Formatted as `INV-YYYY-XXXXXX` (e.g., `INV-2026-000001`) via SQL:
    ```sql
    CONCAT('INV-', TO_CHAR(CURRENT_DATE, 'YYYY'), '-', LPAD(nextval('invoice_number_seq')::text, 6, '0'))
    ```
