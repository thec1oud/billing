# Database Enums & Lookup Tables

The Gebeta Billing architecture combines database reference tables with strongly-typed Go type definitions to maintain consistency across the persistence and application layers.

---

## 1. Summary of Lookup Tables

| Lookup Table | Primary Key Column | Seed Values | Referenced By | Go Type Representation |
| :--- | :--- | :--- | :--- | :--- |
| `tariff_type` | `tariff_type_code` | `FLAT_FEE`, `PER_UNIT`, `TIERED_USAGE`, `STAIRSTEP`, `PACKAGE`, `MATRIX`, `COMPOSITE` | `tariffs.tariff_type_code` | `tariffmodel.TariffTypeCode` |
| `billing_interval` | `billing_interval_code` | `DAY`, `WEEK`, `MONTH`, `YEAR` | *Available for period schemas* | *Duration string* |
| `plan_legacy_price_policy` | `legacy_price_policy_code` | `KEEP_FOREVER`, `MIGRATE_IMMEDIATELY`, `MIGRATE_ON_RENEWAL` | `plans.legacy_price_policy_code` | `planmodel.LegacyPricePolicy` |
| `item_type` | `item_type_code` | `PLAN`, `ONE_TIME_SERVICE`, `PRODUCT` | `purchasable_items.item_type_code` | `itemmodel.ItemTypeCode` |
| `account_status` | `account_status_code` | `PENDING_VERIFICATION`, `ACTIVE`, `SUSPENDED`, `CLOSED` | `accounts.account_status_code` | `accountmodel.Status` |
| `payment_provider` | `payment_provider_code` | `stripe`, `chapa`, `telebirr`, `paypal`, `fake` | `payment_methods`, `payment_attempts`, `ppi_webhooks` | `string` / Adapter Provider Code |
| `payment_type` | `payment_type_code` | `card`, `bank`, `mobile_money` | `payment_methods.payment_type_code` | `fake.MethodType` (`CARD`, `MOBILE_MONEY`) |
| `payment_status` | `payment_status_code` | `ACTIVE`, `INACTIVE`, `EXPIRED`, `REVOKED` | `payment_methods.payment_status_code` | `string` |
| `subscription_status` | `subscription_status_code` | `TRIALING`, `ACTIVE`, `PAUSED`, `PAST_DUE`, `CANCELED`, `UNPAID` | `subscriptions.subscription_status_code` | `subscriptionmodel.Status` |
| `invoice_status` | `invoice_status_code` | `DRAFT`, `OPEN`, `PAID`, `UNCOLLECTIBLE`, `VOID` | `invoices.invoice_status_code` | `invoicemodel.Status` |
| `ledger_party_type` | `ledger_party_type_code` | `ACCOUNT`, `PLATFORM`, `PROVIDER` | `credit_ledger.(source/destination)` | `string` |
| `ledger_entry_type` | `ledger_entry_type_code` | `CREDIT_TRANSFER`, `REFUND`, `PROMOTIONAL`, `MANUAL_ADJUSTMENT` | `credit_ledger.ledger_entry_type_code` | `string` |

---

## 2. In-Code Enumerations (Not in DB Lookup Tables)

Certain domain enums are enforced in application code or JSON columns:

### Quantity Units (`tariffmodel.QuantityUnit`)
Used in `tariffs.quantity_unit` and `invoice_line_items.quantity_unit`:
- `COUNT`: Simple integer item counts.
- `SEAT`: User or seat licenses.
- `GIGABYTE`: Data storage or network bandwidth.
- `HOUR`: Compute runtime or service duration.
- `API_CALL`: Request volume.

### Tier Strategies (`tariffmodel.TierStrategy`)
Used in `tariffs.tier_strategy` for `TIERED_USAGE` tariffs:
- `VOLUME`: Entire volume is billed at the price bracket reached (calculation currently unimplemented in Go service).
- `GRADUATED`: Usage is sliced across successive tier brackets, each priced according to its boundary (fully implemented in `calculateGraduatedCharge`).

### Payment Attempt Status (`paymentattemptmodel.Status`)
Column `payment_attempts.status`:
- `PENDING`: Payment initiated with payment provider, awaiting customer redirect or asynchronous webhook.
- `SUCCESS`: Transaction confirmed and settled.
- `FAILED`: Transaction rejected or expired.
- `REFUNDED`: Reversal completed.

### State Machine Engine Constants (`sm_model`)
- **Hook Types**: `ON_ENTER`, `ON_EXIT`, `ON_TRANSITION`
- **Action Modes**: `SYNC` (inline in transaction), `ASYNC` (delegated to outbox)
- **Error Policies**: `ABORT` (rolls back transition), `CONTINUE` (logs error and advances)
- **Instance Status**: `RUNNING`, `COMPLETED`
- **Outbox Status**: `PENDING`, `PROCESSING`, `PUBLISHED`, `FAILED`
- **Schedule Status**: `PENDING`, `PROCESSING`, `FIRED`, `CANCELED`, `FAILED`
