# Database Triggers, Functions & Concurrency

This document details database triggers, stored procedures, transaction-level advisory locks, and sequence mechanisms used to enforce data integrity and optimistic concurrency.

---

## 1. Active Triggers & Stored Functions

### `protect_finalized_invoice_header()`

- **Trigger Attached**: `trg_protect_invoice_immutability` ON `invoices` (`BEFORE UPDATE FOR EACH ROW`).
- **Purpose**: Enforces financial immutability once an invoice leaves the `'DRAFT'` state.
- **SQL Implementation**:
  ```sql
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
      RETURN NEW;
  END;
  $$ LANGUAGE plpgsql;
  ```
- **Rules Enforced**:
  - Once an invoice transitions to `OPEN`, `PAID`, `UNCOLLECTIBLE`, or `VOID`, the financial amounts (`subtotal_amount`, `tax_amount`, `discount_amount`, `total_amount`), currency, and customer `account_id` **can never be modified**.
  - Subsequent updates are restricted to settlement balances (`amount_paid`, `amount_due`), timestamps (`due_at`, `paid_at`), and status transitions.
- **Fix History**:
  In migration `0001_init_schema.up.sql`, this trigger function attempted `NEW.updated_at := CURRENT_TIMESTAMP;`, which threw an error because the `invoices` table does not have an `updated_at` column. Migration `20260826000000_fix_invoice_trigger.up.sql` corrected this by removing the invalid column assignment.

---

## 2. Deprecated / Removed Triggers (Migration `0007`)

The initial schema included two database-level triggers that were subsequently dropped in migration `0007_remove_triggers.up.sql`:

1. **`trg_protect_account_currency`**:
   - Dropped Function: `enforce_account_currency_immutability()`
   - Reason for removal: Enforcing currency immutability by querying `credit_ledger` and `invoices` in a row-level trigger caused transaction lock contention during account updates. Immutability checks are now handled at the application service level.

2. **`trg_protect_event_log_immutability`**:
   - Dropped Function: `prevent_event_log_mutation()`
   - Reason for removal: Replaced by database role permissions and application repository boundaries.

---

## 3. Concurrency Control Mechanisms

### Transactional Advisory Locks for Auto-Versioning

To allow concurrent creation of tariffs and plans without risking duplicate version numbers or gap collisions, the service uses PostgreSQL 64-bit transactional advisory locks (`pg_advisory_xact_lock`):

```sql
WITH locked AS (
    SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
), next_version AS (
    SELECT COALESCE(MAX(version), 0) + 1 AS version
    FROM plans, locked
    WHERE plan_code = $1
)
INSERT INTO plans (plan_code, version, ...)
SELECT $1, next_version.version, ...
FROM next_version
RETURNING ...;
```

- **Mechanism**:
  - `hashtextextended(code, 0)` computes a deterministic 64-bit integer hash of the natural grouping key (`plan_code` or `tariff_code`).
  - `pg_advisory_xact_lock(...)` serializes concurrent transactions targeting the **same code** without locking unrelated codes or whole tables.
  - The lock releases automatically upon transaction `COMMIT` or `ROLLBACK`.

### Event Store Sequence Monotonicity

The event store guarantees strictly ordered sequences for each aggregate instance through an atomic query pattern:

```sql
INSERT INTO event_log (
    event_type, event_version, aggregate_type, aggregate_id, sequence, actor, causation_id, correlation_id, payload
) 
SELECT 
    $1, $2, $3, $4, 
    COALESCE(MAX(e.sequence), 0) + 1,
    $5, $6, $7, $8
FROM (SELECT 1) AS dummy
LEFT JOIN event_log e ON e.aggregate_type = $3 AND e.aggregate_id = $4
GROUP BY e.aggregate_type, e.aggregate_id
RETURNING event_id, sequence, occurred_at;
```

Any competing insert attempting the same sequence causes a unique constraint violation on `uq_aggregate_sequence (aggregate_type, aggregate_id, sequence)`, returning `ErrSequenceConflict`.

### Queue Processing with `FOR UPDATE SKIP LOCKED`

Workers processing background jobs (`sm_action_outbox` and `sm_scheduled_transitions`) use Postgres 9.5+ row-level lock skipping:

```sql
WITH candidates AS (
    SELECT outbox_id
    FROM sm_action_outbox
    WHERE status = 'PENDING'
    ORDER BY created_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE sm_action_outbox o
SET status = 'PROCESSING',
    claimed_at = now(),
    claimed_by = $2,
    attempts = attempts + 1
FROM candidates c
WHERE o.outbox_id = c.outbox_id
RETURNING o.outbox_id, ...;
```

This guarantees conflict-free, multi-worker parallel execution without blocking or lock contention.
