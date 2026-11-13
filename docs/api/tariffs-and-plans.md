# Tariffs & Plans API

The Tariffs & Plans API manages versioned pricing structures, recurring billing packages, and the purchasable catalog items.

---

## 1. Create Tariff

`POST /api/v1/tariffs`

Creates a new version of a pricing tariff. If a tariff with `code` already exists, an advisory transaction lock is acquired and the `version` number is automatically incremented.

### Request Body Schema
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `code` | string | **Yes** | Natural tariff identifier (e.g. `"STANDARD_USAGE_V1"`). |
| `name` | string | **Yes** | Display title. |
| `description` | string | No | Pricing description. |
| `tariff_type_code` | string | **Yes** | `FLAT_FEE`, `PER_UNIT`, `TIERED_USAGE`, `STAIRSTEP`, `PACKAGE`, `MATRIX`, `COMPOSITE`. |
| `tier_strategy` | string | Conditional | Required for `TIERED_USAGE`: `GRADUATED` or `VOLUME`. |
| `quantity_unit` | string | Conditional | Required for `PER_UNIT`: `COUNT`, `SEAT`, `GIGABYTE`, `HOUR`, `API_CALL`. |
| `amount` | object | **Yes** | Base monetary amount `{ "amount_minor": 1000, "currency": "ETB" }`. |
| `tiers` | array | Conditional | Required for `TIERED_USAGE`. Array of tier brackets. |
| `metadata` | object | No | Custom JSON metadata. |

### Example Request (Graduated Tiered Usage)
```json
{
  "code": "API_VOLUME_PRICING",
  "name": "API Request Tiered Pricing",
  "description": "Graduated tier pricing for API calls",
  "tariff_type_code": "TIERED_USAGE",
  "tier_strategy": "GRADUATED",
  "quantity_unit": "API_CALL",
  "amount": {
    "amount_minor": 0,
    "currency": "USD"
  },
  "tiers": [
    {
      "up_to": 10000,
      "unit_price": { "amount_minor": 1, "currency": "USD" },
      "flat_fee": { "amount_minor": 0, "currency": "USD" }
    },
    {
      "unit_price": { "amount_minor": 0, "currency": "USD" },
      "flat_fee": { "amount_minor": 5000, "currency": "USD" }
    }
  ]
}
```

### Success Response (`201 Created`)
```json
{
  "success": true,
  "data": {
    "tariff_id": 1,
    "tariff_code": "API_VOLUME_PRICING",
    "version": 1,
    "name": "API Request Tiered Pricing",
    "description": "Graduated tier pricing for API calls",
    "tariff_type_code": "TIERED_USAGE",
    "tier_strategy": "GRADUATED",
    "quantity_unit": "API_CALL",
    "amount": {
      "amount_minor": 0,
      "currency": "USD"
    },
    "is_active": true,
    "created_at": "2026-10-02T10:00:00Z"
  }
}
```

---

## 2. Create Plan

`POST /api/v1/plans`

Creates an immutable plan version. When a plan is created, the system **automatically registers a matching purchasable item** in `purchasable_items` linked to this plan.

### Request Body Schema
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `plan_code` | string | **Yes** | Natural plan identifier (e.g. `"PRO_MONTHLY"`). |
| `legacy_price_policy_code` | string | **Yes** | `KEEP_FOREVER`, `MIGRATE_IMMEDIATELY`, `MIGRATE_ON_RENEWAL`. |
| `effective_from` | string (ISO) | No | Activation date (defaults to current UTC time). |
| `effective_until` | string (ISO) | No | Optional sunset timestamp. |
| `durations` | array | **Yes** | List of durations binding this plan to tariffs. |
| `durations[].tariff_id` | integer | **Yes** | Target tariff ID (> 0). |
| `durations[].duration` | integer | **Yes** | Period in nanoseconds (e.g. 30 days = `2592000000000000`). |

### Example Request
```json
{
  "plan_code": "PRO_SUBSCRIPTION",
  "legacy_price_policy_code": "KEEP_FOREVER",
  "durations": [
    {
      "tariff_id": 1,
      "duration": 2592000000000000
    }
  ]
}
```

### Success Response (`201 Created`)
```json
{
  "success": true,
  "data": {
    "plan_id": 1,
    "plan_code": "PRO_SUBSCRIPTION",
    "version": 1,
    "effective_from": "2026-10-02T10:00:00Z",
    "legacy_price_policy_code": "KEEP_FOREVER",
    "durations": [
      {
        "plan_duration_id": 1,
        "plan_id": 1,
        "tariff_id": 1,
        "duration": 2592000000000000,
        "is_active": true,
        "created_at": "2026-10-02T10:00:00Z"
      }
    ],
    "created_at": "2026-10-02T10:00:00Z"
  }
}
```

---

## 3. List Active Plans

`GET /api/v1/plans`

Returns the active versions of all plans (`effective_until IS NULL`), selecting the highest version for each distinct plan code.

### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": [
    {
      "plan_id": 1,
      "plan_code": "PRO_SUBSCRIPTION",
      "version": 1,
      "effective_from": "2026-10-02T10:00:00Z",
      "legacy_price_policy_code": "KEEP_FOREVER",
      "created_at": "2026-10-02T10:00:00Z"
    }
  ]
}
```

---

## 4. List Purchasable Items

`GET /api/v1/purchasable-items`

Returns all active and inactive purchasable catalog items.

### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": [
    {
      "item_id": 1,
      "item_code": "PRO_SUBSCRIPTION",
      "item_type_code": "PLAN",
      "name": "PRO_SUBSCRIPTION",
      "plan_id": 1,
      "is_active": true,
      "created_at": "2026-10-02T10:00:00Z"
    }
  ]
}
```
