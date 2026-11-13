# Quickstart Guide

This guide will walk you through the core "Happy Path" of the Gebeta Billing system. By the end, you will have created a pricing plan, registered a customer account, and subscribed that customer to your plan using the HTTP API.

> [!NOTE]
> All examples assume your Gebeta Billing instance is running locally on `http://localhost:8080`.

---

## Step 1: Create a Tariff
A **Tariff** defines *what* you are charging for and *how much* it costs. Let's create a simple flat-fee monthly tariff for a "Pro" plan that costs $10.00 USD.

```bash
curl -X POST http://localhost:8080/api/v1/tariffs \
  -H "Content-Type: application/json" \
  -d '{
    "code": "PRO_MONTHLY_FEE",
    "name": "Pro Plan Base Fee",
    "tariff_type_code": "FLAT_FEE",
    "amount": {
      "amount_minor": 1000, 
      "currency": "USD"
    }
  }'
```
*Note: `amount_minor: 1000` equals $10.00, as amounts are always handled in the lowest currency denominator to prevent floating-point errors.*

---

## Step 2: Create a Plan
A **Plan** is what a customer actually subscribes to. It bundles one or more tariffs together and gives them a billing duration (e.g., 30 days).

Let's create a plan referencing the Tariff ID (`1`) we just created. The duration must be expressed in nanoseconds (30 days = `2592000000000000` ns).

```bash
curl -X POST http://localhost:8080/api/v1/plans \
  -H "Content-Type: application/json" \
  -d '{
    "plan_code": "PRO_SUBSCRIPTION",
    "legacy_price_policy_code": "KEEP_FOREVER",
    "durations": [
      {
        "tariff_id": 1,
        "duration": 2592000000000000
      }
    ]
  }'
```

---

## Step 3: Create an Account
An **Account** represents your customer. 

```bash
curl -X POST http://localhost:8080/api/v1/accounts \
  -H "Content-Type: application/json" \
  -d '{
    "external_id": "tenant_12345",
    "currency": "USD",
    "timezone": "UTC"
  }'
```

By default, new accounts are created in a `PENDING_VERIFICATION` state. Before they can be billed, you need to activate them.

```bash
# Activate the account we just created (Account ID 1)
curl -X POST http://localhost:8080/api/v1/accounts/1/activate
```

---

## Step 4: Add a Payment Method
Before creating a subscription, the account needs a payment method on file. For testing purposes, we can attach a mock active payment method.

```bash
curl -X POST http://localhost:8080/api/v1/accounts/1/payment-methods \
  -H "Content-Type: application/json" \
  -d '{
    "payment_method_id": "pm_chapa_active"
  }'
```

---

## Step 5: Subscribe the Account
Finally, we attach the activated Account (`account_id: 1`) to our Plan (`plan_id: 1`).

```bash
curl -X POST http://localhost:8080/api/v1/subscriptions \
  -H "Content-Type: application/json" \
  -d '{
    "account_id": 1,
    "plan_id": 1,
    "plan_version": 1,
    "quantity": 1
  }'
```

**Success!** The system will now automatically:
1. Generate an invoice for $10.00.
2. Attempt to charge the attached payment method via the Payment Provider Interface (PPI).
3. Set the subscription status to `ACTIVE`.

---

**Next Steps:**
* Learn more about API endpoints in the [API Reference](api/README.md).
* Understand how data is structured in the [Database Overview](database/README.md).
