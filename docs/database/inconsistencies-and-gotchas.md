# Database Inconsistencies, Quirks & Codebase Audit

This document records architectural discrepancies, legacy artifacts, and undocumented behaviors uncovered during a deep inspection of the database migrations and application code.

---

## 1. Trigger & Schema Mismatches

### The Invoice `updated_at` Incident
- **Discovery**: In the initial schema migration (`0001_init_schema.up.sql`), the trigger function `protect_finalized_invoice_header()` assigned:
  ```sql
  NEW.updated_at := CURRENT_TIMESTAMP;
  ```
  However, the `invoices` table was defined without an `updated_at` column. Any update to a finalized invoice caused PostgreSQL to throw error `record "new" has no field "updated_at"`.
- **Resolution**: Migration `20260826000000_fix_invoice_trigger.up.sql` resolved this by removing the assignment.
- **Current State**: Note that the `invoices` table **does not have an `updated_at` column**, unlike `payment_attempts` and `sm_instances`.

### Dropped Database-Level Immutability Triggers
- **Discovery**: Migration `0007_remove_triggers.up.sql` dropped two core triggers:
  1. `trg_protect_account_currency` (which prevented changing account currency if invoices or ledger entries existed).
  2. `trg_protect_event_log_immutability` (which blocked `UPDATE` or `DELETE` on `event_log`).
- **Impact**: Currency immutability and append-only event log constraints are **no longer enforced by PostgreSQL triggers**. They rely entirely on application service logic and database role permissions.

### Invoice Version Column Added and Removed
- **Discovery**: Migration `0002_add_invoice_version.up.sql` added a `version BIGINT NOT NULL DEFAULT 1` column to `invoices` for optimistic locking. Subsequently, migration `0010_remove_invoice_version.up.sql` dropped the column (`ALTER TABLE invoices DROP COLUMN version;`).
- **Current State**: Invoices do not have a `version` column. Optimistic concurrency for invoice transitions is instead enforced on `sm_instances.version` by the state machine engine.

---

## 2. Hardcoded Query & Code Constraints

### Hardcoded Provider and Type in Account Payment Methods
- **Discovery**: In `internal/database/queries/account.sql`, the query `CreatePaymentMethod` hardcodes the gateway and instrument type:
  ```sql
  INSERT INTO payment_methods (
      account_id, payment_provider_code, provider_reference,
      payment_type_code, is_default, payment_status_code
  )
  VALUES ($1, 'chapa', $2, 'mobile_money', TRUE, 'ACTIVE');
  ```
- **Impact**: Calling the Go API endpoint `POST /api/v1/accounts/{accountID}/payment-methods` always records the provider as `'chapa'` and instrument as `'mobile_money'`, ignoring any other provider or instrument type.

### Mock Payment Methods In-Memory Allowlist
- **Discovery**: In `internal/account/service/service.go`, `AddPaymentMethod` explicitly validates input against an in-memory map:
  ```go
  if _, ok := adapters.MockPaymentMethods[paymentMethodID]; !ok {
      return model.Account{}, ErrPaymentMethodNotFound
  }
  ```
  The map (`internal/ppi/adapters/fake/fake_payment_methods.go`) contains only two keys:
  - `"pm_chapa_active"`
  - `"pm_stripe_card_fail"`
- **Impact**: Any request passing a payment method ID other than these two will fail with a 400 Bad Request error.

---

## 3. Pricing & Catalog Implementation Limits

### Unimplemented Tariff Calculation Strategies
- **Discovery**: The `tariff_type` lookup table contains 7 types (`FLAT_FEE`, `PER_UNIT`, `TIERED_USAGE`, `STAIRSTEP`, `PACKAGE`, `MATRIX`, `COMPOSITE`). However, in `internal/tariff/model/model.go`:
  - `CalculateCharge` is only implemented for `FLAT_FEE`, `PER_UNIT`, and `TIERED_USAGE`.
  - For `TIERED_USAGE`, only `TierStrategyGraduated` is implemented. If a tariff uses `TierStrategyVolume`, the service returns:
    `"charge calculation for tier strategy VOLUME is not implemented"`.
  - `STAIRSTEP`, `PACKAGE`, `MATRIX`, and `COMPOSITE` return `"charge calculation for tariff type ... is not implemented"`.

### Plan Duration Units
- **Discovery**: `plan_durations.duration` is defined as `BIGINT`. In the Go service, it binds to `time.Duration`, which measures time in **nanoseconds**.
- **Impact**: When inserting a 30-day duration, the value stored is `30 * 24 * 3600 * 1,000,000,000` = `2,592,000,000,000,000`. Supplying small integer numbers like `30` will represent 30 nanoseconds.

---

## 4. Architectural & Routing Duality

### Two Go HTTP Routers
- **Discovery**: The codebase contains two router implementations:
  1. `internal/api/router.go`: Mounted at `/v1/...` (legacy/testing router).
  2. `internal/infra/api/server.go`: Mounted at `/api/v1/...` (production server instantiated in `cmd/main.go`).
- **Production Status**: `cmd/main.go` instantiates `internal/infra/api/server.go`. All production calls must use `/api/v1/...`.
