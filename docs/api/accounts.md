# Accounts API

The Accounts API manages the lifecycle, billing profiles, currencies, and payment methods for customer billing accounts.

---

## 1. Create Account

`POST /api/v1/accounts`

Creates a new account in initial state `PENDING_VERIFICATION`. Appends an `billing.account.created` event to the event store.

### Request Headers
- `Content-Type: application/json`

### Request Body Schema (`model.CreateInput`)
| Field | Type | Required | Description / Rules |
| :--- | :--- | :--- | :--- |
| `external_id` | string | No | Unique external tenant ID (max 255 chars). Must be unique if provided. |
| `currency` | string | **Yes** | 3-letter ISO 4217 currency code (e.g. `"USD"`, `"ETB"`). |
| `timezone` | string | **Yes** | Valid IANA timezone (e.g. `"Africa/Addis_Ababa"`, `"UTC"`). |
| `locale` | string | No | Locale code (e.g. `"en-US"`, max 35 chars). Default: `"en-US"`. |
| `net_terms` | integer | No | Days allowed before invoice due date (min: `0`, max: `32767`). Default: `0`. |
| `dunning_profile_id`| integer | No | ID of custom dunning policy. |
| `tax_identifiers` | object | No | JSON object containing VAT, TIN, or tax registration details. |
| `billing_address` | object | No | JSON object describing physical/postal billing address. |
| `compliance_flags`| object | No | JSON object recording KYC/AML validation flags. |
| `metadata` | object | No | JSON object for client-specific metadata. |

### Example Request
```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "external_id": "cust_org_48102",
    "currency": "USD",
    "timezone": "America/New_York",
    "locale": "en-US",
    "net_terms": 30,
    "billing_address": {
      "line1": "100 Broadway",
      "city": "New York",
      "state": "NY",
      "postal_code": "10005",
      "country": "US"
    }
  }'
```

### Success Response (`201 Created`)
```json
{
  "success": true,
  "data": {
    "id": 1,
    "external_id": "cust_org_48102",
    "status": "PENDING_VERIFICATION",
    "currency": "USD",
    "timezone": "America/New_York",
    "locale": "en-US",
    "net_terms": 30,
    "billing_address": {
      "city": "New York",
      "country": "US",
      "line1": "100 Broadway",
      "postal_code": "10005",
      "state": "NY"
    },
    "tax_identifiers": {},
    "compliance_flags": {},
    "metadata": {},
    "created_at": "2026-10-02T10:00:00Z"
  }
}
```

### Error Responses
- **`400 Bad Request`**: Currency invalid, timezone invalid, or net terms negative.
  ```json
  {
    "success": false,
    "error": {
      "code": "invalid_request",
      "message": "currency must be a three-letter ISO 4217 code"
    }
  }
  ```
- **`409 Conflict`**: External ID already exists.
  ```json
  {
    "success": false,
    "error": {
      "code": "conflict",
      "message": "account external id already exists"
    }
  }
  ```

---

## 2. Get Account

`GET /api/v1/accounts/{accountID}`

Retrieves account record along with its active payment method provider references.

### Path Parameters
- `accountID` (integer, required): Account ID (positive integer).

### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": {
    "id": 1,
    "external_id": "cust_org_48102",
    "status": "ACTIVE",
    "currency": "USD",
    "timezone": "America/New_York",
    "locale": "en-US",
    "net_terms": 30,
    "payment_methods": [
      "pm_chapa_active"
    ],
    "created_at": "2026-10-02T10:00:00Z"
  }
}
```

### Error Responses
- **`400 Bad Request`**: `accountID` is not a positive integer.
- **`404 Not Found`**: Account not found.

---

## 3. Activate Account

`POST /api/v1/accounts/{accountID}/activate`

Transitions an account from `PENDING_VERIFICATION` to `ACTIVE`.

### Success Response (`200 OK`)
```json
{
  "success": true,
  "data": {
    "id": 1,
    "status": "ACTIVE"
  }
}
```

### Error Responses
- **`404 Not Found`**: Account ID does not exist.
- **`409 Conflict`**: Account is not in `PENDING_VERIFICATION` status.

---

## 4. Suspend Account

`POST /api/v1/accounts/{accountID}/suspend`

Transitions an `ACTIVE` account to `SUSPENDED`.

### Request Body
```json
{
  "reason": "Overdue invoices exceeding dunning tolerance"
}
```

### Error Responses
- **`400 Bad Request`**: Reason is empty.
- **`409 Conflict`**: Account is not currently `ACTIVE`.

---

## 5. Reactivate Account

`POST /api/v1/accounts/{accountID}/reactivate`

Transitions a `SUSPENDED` account back to `ACTIVE`.

### Success Response (`200 OK`)
Returns updated account object with `"status": "ACTIVE"`.

---

## 6. Close Account

`POST /api/v1/accounts/{accountID}/close`

Permanently closes an account. An account can only be closed from `ACTIVE` or `SUSPENDED` state. Once closed, an account cannot transition to any other status.

### Request Body
```json
{
  "reason": "Customer requested account termination"
}
```

---

## 7. Add Payment Method

`POST /api/v1/accounts/{accountID}/payment-methods`

Attaches a tokenized payment method to an account.

> [!NOTE]
> Currently, `payment_method_id` must match a recognized identifier in `adapters.MockPaymentMethods`:
> - `"pm_chapa_active"` (Telebirr / Mobile Money)
> - `"pm_stripe_card_fail"` (Card)

### Request Body
```json
{
  "payment_method_id": "pm_chapa_active"
}
```

### Success Response (`200 OK`)
Returns the updated account with the new payment method included in `"payment_methods"`.
