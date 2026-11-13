# Subscriptions API

The Subscriptions API creates and manages recurring billing agreements linked to customer accounts and versioned plans.

---

## 1. Create Subscription

`POST /api/v1/subscriptions`

Creates an `ACTIVE` subscription agreement.

### Pre-conditions Enforced by Service
1. `account_id` must reference an existing account in `ACTIVE` status.
2. `plan_id` must exist, and the provided `plan_version` must match the stored plan version.
3. The service **automatically computes** the current period:
   - `current_period_start_at`: `timeutil.NowUTC()` (or caller-supplied `current_period_start` in UTC)
   - `billing_cycle_anchor`: `current_period_start_at` (or caller-supplied anchor in UTC)
   - `current_period_end_at`: computed via `timeutil.MonthlyPeriodEnd(start, anchorDay)` (or caller-supplied end), which clamps month-end days (29, 30, 31) to target month ends without month overflow
4. The service automatically initializes an `sm_instances` record tied to the `subscription_lifecycle` state machine in state `ACTIVE`.

### Timezone Policy
All subscription periods, billing cycle anchors, and timestamp records are computed and stored in **UTC**. The customer account's `timezone` attribute is an informational field retained for localized invoice display, formatting, and customer communication. Internal period boundaries and recurring interval calculations are strictly UTC-normalized to ensure consistent, monotonic recurrence unaffected by Daylight Saving Time (DST) shifts.

### Request Body Schema (`model.CreateInput`)
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `account_id` | integer | **Yes** | ID of the paying account (> 0). Must be `ACTIVE`. |
| `plan_id` | integer | **Yes** | ID of the target plan (> 0). |
| `plan_version` | integer | **Yes** | Version number of the plan (> 0). |
| `quantity` | integer | No | License count (default `1`). |
| `current_period_start` | string | No | Optional ISO 8601 UTC start time (defaults to now). |
| `current_period_end` | string | No | Optional ISO 8601 UTC end time (defaults to MonthlyPeriodEnd). |
| `billing_cycle_anchor` | string | No | Optional ISO 8601 UTC anchor (defaults to start time). |

### Example Request
```json
{
  "account_id": 1,
  "plan_id": 1,
  "plan_version": 1,
  "quantity": 1
}
```

### Success Response (`201 Created`)
```json
{
  "success": true,
  "data": {
    "id": 1,
    "version": 1,
    "account_id": 1,
    "plan_id": 1,
    "plan_version": 1,
    "status": "ACTIVE",
    "quantity": 1,
    "current_period_start": "2026-10-02T10:00:00Z",
    "current_period_end": "2026-11-02T10:00:00Z",
    "billing_cycle_anchor": "2026-10-02T10:00:00Z",
    "cancel_at_period_end": false,
    "resumed_at": null
  }
}
```

### Error Responses
- **`400 Bad Request`**: Account not active or plan version mismatch.
  ```json
  {
    "success": false,
    "error": {
      "code": "invalid_request",
      "message": "account 1 is not active"
    }
  }
  ```

---

## 2. Get Subscription

`GET /api/v1/subscriptions/{subscriptionID}`

Fetches an existing subscription by its primary key ID.

### Path Parameters
- `subscriptionID` (integer, required): Target subscription ID.

### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": {
    "id": 1,
    "version": 1,
    "account_id": 1,
    "plan_id": 1,
    "plan_version": 1,
    "status": "ACTIVE",
    "quantity": 1,
    "current_period_start": "2026-10-02T10:00:00Z",
    "current_period_end": "2026-11-02T10:00:00Z",
    "billing_cycle_anchor": "2026-10-02T10:00:00Z",
    "cancel_at_period_end": false
  }
}
```

---

## 3. List Account Subscriptions

`GET /api/v1/accounts/{id}/subscriptions`

Lists all subscriptions in status `ACTIVE` owned by the specified account ID.

### Path Parameters
- `id` (integer, required): Account ID.

### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "version": 1,
      "account_id": 1,
      "plan_id": 1,
      "plan_version": 1,
      "status": "ACTIVE",
      "quantity": 1,
      "current_period_start": "2026-10-02T10:00:00Z",
      "current_period_end": "2026-11-02T10:00:00Z",
      "billing_cycle_anchor": "2026-10-02T10:00:00Z",
      "cancel_at_period_end": false
    }
  ]
}
```
