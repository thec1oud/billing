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
   - `current_period_start_at`: `time.Now().UTC()`
   - `current_period_end_at`: `now.AddDate(0, 1, 0)` (1 month default)
   - `billing_cycle_anchor`: `now`
4. The service automatically initializes an `sm_instances` record tied to the `subscription_lifecycle` state machine in state `ACTIVE`.

### Request Body Schema (`model.CreateInput`)
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `account_id` | integer | **Yes** | ID of the paying account (> 0). Must be `ACTIVE`. |
| `plan_id` | integer | **Yes** | ID of the target plan (> 0). |
| `plan_version` | integer | **Yes** | Version number of the plan (> 0). |
| `quantity` | integer | No | License count (default `1`). |

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
    "cancel_at_period_end": false
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
