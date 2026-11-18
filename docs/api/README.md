# API Architecture & Conventions

The Gebeta Billing system exposes two distinct API tiers:
1. **Authoritative Core Billing Service API (Go)**: Directly controls the PostgreSQL database, FSM engine, and event store. Listens on port `8080` (or `APP_PORT`).
2. **Platform Backend-For-Frontend (BFF) Gateway API (Node.js)**: Acts as the client-facing proxy, providing request validation, security headers, rate limiting, and response shaping. Listens on port `3000`.

---

## 1. Authentication & Security Status

All business API routes on both the BFF and the Go service require an externally issued OIDC access token in the `Authorization: Bearer <token>` header. Both services verify the JWT signature against the configured JWKS URL and validate its issuer, audience, expiry, and subject. Configure `AUTH_JWKS_URL`, `AUTH_ISSUER`, and `AUTH_AUDIENCE` in both services. The same access token is forwarded by the BFF to Go, so direct access to the Go service is also authenticated.

The event-log actor for HTTP actions is derived from the verified token's `sub` claim. Client-provided `X-Actor-Type` and `X-Actor-ID` headers are ignored. Health checks (`/health`, `/api/health`, `/api/ready`) remain public for deployment probes.

Authentication confirms the caller's identity; account/tenant-level authorization policies are not yet implemented. Every identity accepted by the configured issuer can reach the same business routes.

Provider webhook routes are exempt from bearer authentication because payment providers call them directly; each adapter must verify the provider's signature over the exact raw request body before accepting the event. The fake provider is development-only and requires an HMAC signature; see the [webhook contract](webhooks.md).

- **CORS**: Platform BFF enables CORS support for cross-origin web browser traffic, permitting headers `Content-Type`, `Authorization`, and `X-Request-Id`.
- **Rate Limiting**: Platform BFF defaults to **120 requests per 60 seconds** per IP on `/api/*` routes. Configure with `RATE_LIMIT_WINDOW_MS` and `RATE_LIMIT_MAX`.
- **Request ID Tracking**: The BFF injects or forwards an `X-Request-Id` (UUID) across all incoming and proxied requests for distributed tracing.

---

## 2. Response Envelopes & Error Structures

### Core Billing Service Envelope (Go)

Except for `GET /health` (which outputs raw `{"status":"ok"}`), all responses from the Go billing service follow the standardized JSON envelope:

#### Success Response
```json
{
  "success": true,
  "data": { ... },
  "meta": { ... }
}
```

#### Error Response (HTTP Status >= 400)
```json
{
  "success": false,
  "error": {
    "code": "invalid_request | not_found | conflict | internal_error",
    "message": "Human-readable failure detail"
  }
}
```

#### Core Error Codes
- `invalid_request`: Payload parsing failed, validation constraint violated, or negative amount.
- `not_found`: Referenced entity (`account_id`, `invoice_id`, `plan_id`) does not exist.
- `conflict`: Duplicate external ID, invalid state machine transition, or sequence conflict.
- `internal_error`: Unexpected database or system fault.
- `UNAUTHENTICATED`: Missing or invalid OIDC bearer token (HTTP 401).

---

### Platform BFF Response Format (Node.js)

#### Success Response
```json
{
  "success": true,
  "data": { ... }
}
```

#### Error Response
```json
{
  "code": "VALIDATION_ERROR | UNAUTHENTICATED | NOT_FOUND | UPSTREAM_ERROR | BAD_GATEWAY | GATEWAY_TIMEOUT",
  "message": "Description of the error",
  "request_id": "c1f7a08e-128a-4952-b88a-36b334199c15",
  "details": [ ... ]
}
```

---

## 3. Summary of API Endpoints

### Core Billing Service (`http://billing-service:8080`)

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/health` | Liveness health check |
| `POST` | `/api/v1/accounts` | Create customer billing account |
| `GET` | `/api/v1/accounts/{accountID}` | Retrieve account details |
| `POST` | `/api/v1/accounts/{accountID}/activate` | Transition account to ACTIVE |
| `POST` | `/api/v1/accounts/{accountID}/suspend` | Transition account to SUSPENDED |
| `POST` | `/api/v1/accounts/{accountID}/reactivate` | Return account from SUSPENDED to ACTIVE |
| `POST` | `/api/v1/accounts/{accountID}/close` | Permanently terminate account |
| `POST` | `/api/v1/accounts/{accountID}/payment-methods` | Attach tokenized payment instrument |
| `POST` | `/api/v1/tariffs` | Create immutable pricing tariff version |
| `POST` | `/api/v1/plans` | Create immutable plan version & duration bindings |
| `GET` | `/api/v1/plans` | List active plans (latest versions) |
| `GET` | `/api/v1/purchasable-items` | List purchasable items catalog |
| `POST` | `/api/v1/dev/invoices/generate` | Development invoice generator (requires `ENABLE_DEV_ENDPOINTS=true`; disabled by default) |
| `POST` | `/api/v1/invoices/{invoiceID}/pay` | Trigger PPI payment charge for open invoice |
| `GET` | `/api/v1/invoices/{invoiceID}` | Retrieve invoice and line items |
| `GET` | `/api/v1/accounts/{id}/invoices` | List invoices for account |
| `POST` | `/api/v1/subscriptions` | Create active subscription for account |
| `GET` | `/api/v1/subscriptions/{subscriptionID}` | Retrieve subscription details |
| `GET` | `/api/v1/accounts/{id}/subscriptions` | List active subscriptions for account |
| `POST` | `/api/v1/payments/charge` | Direct gateway payment charge |
| `POST` | `/api/v1/webhooks/{provider}` | Webhook delivery from external payment provider |

### Platform BFF Gateway (`http://platform-bff:3000`)

| Method | Endpoint | Upstream Service Endpoint |
| :--- | :--- | :--- |
| `GET` | `/api/health` | Process liveness |
| `GET` | `/api/ready` | Checks process + upstream Go billing readiness |
| `POST` | `/api/v1/accounts` | `POST /api/v1/accounts` |
| `POST` | `/api/v1/accounts/:accountId/activate` | `POST /api/v1/accounts/{accountID}/activate` |
| `POST` | `/api/v1/tariffs` | `POST /api/v1/tariffs` |
| `GET` | `/api/v1/plans` | `GET /api/v1/plans` |
| `POST` | `/api/v1/plans` | `POST /api/v1/plans` |
| `POST` | `/api/v1/subscriptions` | `POST /api/v1/subscriptions` |
| `GET` | `/api/v1/accounts/:accountId/subscriptions` | `GET /api/v1/accounts/{id}/subscriptions` |
| `POST` | `/api/v1/invoices/:invoiceId/pay` | `POST /api/v1/invoices/{invoiceID}/pay` |
| `POST` | `/api/v1/dev/invoices/generate` | Development invoice generator (requires `ENABLE_DEV_ENDPOINTS=true` and bearer authentication) |
| `POST` | `/api/v1/webhooks/fake` | Signed fake callback simulator (requires dev endpoints and bearer authentication) |
