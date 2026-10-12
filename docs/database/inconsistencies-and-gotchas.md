# Inconsistencies and Gotchas

This page tracks known behavior that differs from expectations or is not yet fully implemented. Items are linked to their open tracking issues where one exists.

## Routing and Architecture
### Two Go HTTP routers — [I-24](https://github.com/thec1oud/billing/issues/46)

The repository contains two Go routers:

- `internal/infra/api` is the production server, mounted at `/api/v1/...`.
- `internal/api` is a legacy/testing router, mounted at `/v1/...`.

Use the production `/api/v1/...` routes. The duplicate router is also tracked as dead code in I-24.

## Database Behavior

### Database immutability triggers were dropped — [I-39](https://github.com/thec1oud/billing/issues/60)

Migration `0007_remove_triggers` drops `trg_protect_event_log_immutability` and `trg_protect_account_currency`. PostgreSQL therefore no longer enforces append-only behavior for `event_log` or prevents account currency changes after related financial records exist. These invariants currently depend on application logic and database permissions.

### Plan durations are stored as nanoseconds — [I-38](https://github.com/thec1oud/billing/issues/59)

`plan_durations.duration` is a `BIGINT` populated from Go's `time.Duration`, whose unit is nanoseconds. A 30-day duration is stored as `2,592,000,000,000,000`; an integer such as `30` represents 30 nanoseconds, not 30 days. The schema and API do not make this unit explicit.

## Pricing and Payment Methods

### Volume tier pricing — [I-37](https://github.com/thec1oud/billing/issues/22)

I-37 reported that volume pricing was calculated as graduated pricing. The current implementation has a separate volume calculator and tests that distinguish the strategies (for example, 15 units across tiers priced at 100 and 50 costs 750 under volume pricing and 1,250 under graduated pricing). The issue remains open, so confirm its acceptance criteria before closing it.

### Unsupported tariff calculation types

The `tariff_type` lookup contains `STAIRSTEP`, `PACKAGE`, `MATRIX`, and `COMPOSITE`, but `Tariff.CalculateCharge` only calculates `FLAT_FEE`, `PER_UNIT`, and `TIERED_USAGE`. Requests using the four listed types return an unsupported-calculation error.

### Payment methods use a fake-adapter allowlist — [I-35](https://github.com/thec1oud/billing/issues/57)

`AddPaymentMethod` validates payment method IDs against `MockPaymentMethods` in the fake PPI adapter. Only `pm_chapa_active` and `pm_stripe_card_fail` are accepted, and the account repository also resolves their provider/type details from that mock data. Production payment method support is therefore limited to test fixtures.
