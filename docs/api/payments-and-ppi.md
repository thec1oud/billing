# Payments & Payment Provider Interface (PPI) API

The Payments API and Payment Provider Interface (PPI) subsystem abstract external payment gateways (Chapa, Stripe, Telebirr, and the development Fake Provider), recording charge attempts, checkout redirections, and settlement states.

---

## 1. Direct Payment Charge

`POST /api/v1/payments/charge`

Initiates an immediate charge or checkout session with a payment gateway for an invoice.

### Request Body Schema
| Field | Type | Required | Description |
| :--- | :--- | :--- | :--- |
| `provider` | string | **Yes** | Gateway code (e.g. `"fake"`, `"chapa"`, `"stripe"`). |
| `invoice_id` | integer | **Yes** | ID of the target invoice (> 0). |
| `amount_minor` | integer | **Yes** | Charge amount in minor currency units (> 0). |
| `currency` | string | **Yes** | 3-letter ISO 4217 currency code. |

### Example Request
```json
{
  "provider": "fake",
  "invoice_id": 1,
  "amount_minor": 1000,
  "currency": "ETB"
}
```

### Response (`200 OK`)
Depending on the provider adapter and instrument type, the response returns either an immediate settlement or a hosted checkout redirect:

#### Hosted Checkout / Mobile Money (Status `PENDING`)
```json
{
  "status": "PENDING",
  "idempotency_key": "tx_1_1727865600000000000",
  "provider_reference": "fake_ref_6b1e5a28-3e4e-4f7d-8f2c-e5d0a68c92a1",
  "checkout_url": "https://checkout.fake-provider.com/pay/fake_ref_6b1e5a28-3e4e-4f7d-8f2c-e5d0a68c92a1",
  "raw_response": {
    "status": "PENDING",
    "tx_ref": "tx_1_1727865600000000000",
    "reference": "fake_ref_6b1e5a28-3e4e-4f7d-8f2c-e5d0a68c92a1",
    "checkout_url": "https://checkout.fake-provider.com/pay/fake_ref_6b1e5a28-3e4e-4f7d-8f2c-e5d0a68c92a1"
  }
}
```

#### Synchronous Direct Settlement (Status `SUCCESS`)
```json
{
  "status": "SUCCESS",
  "idempotency_key": "tx_1_1727865600000000000",
  "provider_reference": "fake_ref_6b1e5a28-3e4e-4f7d-8f2c-e5d0a68c92a1",
  "raw_response": {
    "status": "SUCCESS",
    "tx_ref": "tx_1_1727865600000000000",
    "reference": "fake_ref_6b1e5a28-3e4e-4f7d-8f2c-e5d0a68c92a1"
  }
}
```

---

## 2. Payment Attempts Tracking

Every charge execution inserts an immutable tracking row into `payment_attempts`:
- **Single Pending Rule**: A partial unique index `idx_payment_attempts_single_pending_per_invoice` prevents multiple concurrent `PENDING` attempts for the same invoice.
- **State Progression**:
  ```mermaid
  stateDiagram-v2
      [*] --> PENDING : Initiated
      PENDING --> SUCCESS : Webhook confirmed / synchronous settlement
      PENDING --> FAILED : Webhook failure / expired session
      SUCCESS --> REFUNDED : Reversal
  ```
- **Audit Fields**: The row captures `raw_request` (outbound payload) and `raw_response` (inbound gateway response) as JSONB.

---

## 3. Development Fake Adapter

The codebase provides an integrated fake adapter (`internal/ppi/adapters/fake`) for local development and CI testing:
- **Provider Code**: `"fake"`
- **Behavior**:
  - Validates `amount_minor > 0`.
  - When `provider` contains `"mobile"`, `"chapa"`, or equals `"fake"`, it generates a hosted checkout URL `https://checkout.fake-provider.com/pay/fake_ref_<uuid>` with status `PENDING`.
  - Other provider codes simulate immediate `SUCCESS`.
