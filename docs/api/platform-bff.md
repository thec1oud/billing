# Platform BFF Gateway API

The **Platform BFF** (`platform/bff/`) is a Node.js Express service running on port `3000`. It serves as the secure edge gateway for browser web clients, sanitizing inputs, enforcing security headers and rate limits, and communicating with the Core Billing Service over private HTTP.

---

## 1. Gateway Middleware Pipeline

Every request passing through the BFF undergoes strict validation and enrichment:

1. **Security Headers**: Managed by `helmet` (Strict-Transport-Security, X-Content-Type-Options, DNS Prefetch Control, Frameguard).
2. **CORS**: Configured to restrict origin access while allowing headers `Content-Type`, `Authorization`, and `X-Request-Id`.
3. **Request Correlation (`X-Request-Id`)**: Injected into every request context and forwarded downstream to the Go service.
4. **Rate Limiting**: Configured at **100 requests per minute** per client IP on `/api/*`. Returns `429 Too Many Requests` when exceeded.
5. **Payload Size Guard**: Body parser caps JSON input at **100kb** (`413 Payload Too Large` on violation).
6. **Strict Field Parsing**: Uses custom validator builders (`shared/http/validate.js`) that reject unexpected keys or malformed datatypes before dispatching to the Go billing service.

---

## 2. Health & Readiness Endpoints

### Liveness Probe
`GET /api/health`
- **Purpose**: Checks if the BFF Node.js process is responsive.
- **Response (`200 OK`)**:
  ```json
  {
    "status": "ok",
    "service": "platform-bff",
    "time": "2026-10-02T10:00:00.000Z"
  }
  ```

### Readiness Probe
`GET /api/ready`
- **Purpose**: Checks if both the BFF process and the upstream Go Billing Service (`/api/v1/plans`) are healthy.
- **Success (`200 OK`)**:
  ```json
  {
    "status": "ready",
    "billing_service": {
      "url": "http://billing-service:8080",
      "status": 200
    }
  }
  ```
- **Degraded / Unavailable (`503 Service Unavailable`)**:
  ```json
  {
    "status": "not_ready",
    "billing_service": {
      "url": "http://billing-service:8080",
      "error": "ECONNREFUSED"
    }
  }
  ```

---

## 3. BFF API Endpoint Surface & Field Mapping

The BFF exposes a curated subset of endpoints designed for browser interaction:

### Accounts
- `POST /api/v1/accounts`: Validates body (strips unexpected keys) and calls `POST /api/v1/accounts` on the billing service.
- `POST /api/v1/accounts/:accountId/activate`: Calls `POST /api/v1/accounts/{accountID}/activate`.

### Tariffs & Plans
- `POST /api/v1/tariffs`: Parses monetary figures and tier arrays, forwarding to `POST /api/v1/tariffs`.
- `GET /api/v1/plans`: Fetches active plans and formats durations as guaranteed arrays (Go marshals empty slices as `null`).
- `POST /api/v1/plans`: Validates durations array, ensuring duration is passed in nanoseconds.

### Subscriptions
- `POST /api/v1/subscriptions`:
  - **Transformation Rule**: Billing period fields (`current_period_start`, `current_period_end`, `billing_cycle_anchor`) are **intentionally ignored and stripped**. The billing service derives these values server-side.
  - Accepts only `{ account_id, plan_id, plan_version, quantity }`.
- `GET /api/v1/accounts/:accountId/subscriptions`: Relays to `GET /api/v1/accounts/{id}/subscriptions`, returning `[]` if no subscriptions exist.

### Invoices
- `POST /api/v1/invoices/:invoiceId/pay`:
  - Calls `POST /api/v1/invoices/{invoiceID}/pay`.
  - **Data Scrubbing**: Strips raw provider payload (`raw_response`) and returns only browser-essential checkout fields: `{ status, internal_tx_id, provider_reference, checkout_url }`.
- `POST /api/v1/dev/invoices/generate`: Relays `{ account_id, plan_id }` to `POST /api/v1/dev/invoices/generate`.

### Webhook Simulation
- `POST /api/v1/webhooks/fake`:
  - Simulates the external fake payment provider calling the billing webhook endpoint from the frontend demo interface.
  - Relays to `POST /api/v1/webhooks/fake` on the billing service.

---

## 4. Key Differences: BFF vs Direct Billing API

| Aspect | Platform BFF (`:3000`) | Core Billing Service (`:8080`) |
| :--- | :--- | :--- |
| **Intended Consumer** | Web browsers, Single Page Apps (SPA) | Internal services, workers, admin systems |
| **Path Prefix** | `/api/v1/...` (and `/api/health`, `/api/ready`) | `/api/v1/...` (and `/health`) |
| **Rate Limiting** | Enforced (100 req / min) | Unrestricted |
| **Request ID** | Generated / propagated (`X-Request-Id`) | Handled via context |
| **Payload Capping** | Capped at 100kb | Capped at 1MB |
| **Account Operations** | Create, Activate | Create, Get, Activate, Suspend, Reactivate, Close, Add Payment Method |
| **Sensitive Data Scrubbing**| Strips `raw_response` from payment charges | Returns full gateway telemetry |
| **Error Handling** | Relays 4xx status/codes; masks 5xx internal errors behind generic gateway codes | Returns exact Go internal error messages |
