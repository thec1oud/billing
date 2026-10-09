# Introduction

Welcome to the **Gebeta Billing System** documentation. 

Gebeta Billing is a robust, self-hosted, and event-driven billing and subscription lifecycle management platform. It is designed to act as the financial backbone for SaaS applications, API providers, and digital services that need to manage accounts, track subscriptions, and process payments reliably.

## What is Gebeta Billing?

At its core, Gebeta Billing is a distributed ledger and subscription engine built in Go. It sits between your application and your Payment Providers (like Stripe, Chapa, etc.), handling the complexities of:

*   **Tariffs & Plans**: Defining what you sell, how much it costs, and the billing cycle.
*   **Subscription Lifecycles**: Managing when a user is active, past due, or cancelled.
*   **Invoicing**: Automatically generating invoices for flat-rate and usage-based plans.
*   **Payment Reconciliation**: securely tracking attempts, successes, and failures for every transaction.

## Key Concepts

To effectively use the billing system, it's helpful to understand the domain language it uses:

*   **Account**: The central entity representing a customer or tenant in the billing system. All subscriptions and invoices belong to an account.
*   **Tariff**: The pricing blueprint. A tariff defines the recurring charge, billing frequency (e.g., monthly, annually), and any usage-based metered limits.
*   **Subscription**: An agreement connecting an Account to a Tariff. Subscriptions are highly dynamic—they can be active, suspended, or cancelled depending on payment status.
*   **Invoice**: A record of a financial charge generated for a specific billing period. It contains line items based on the subscription's tariff.
*   **PPI (Payment Provider Interface)**: The abstraction layer that allows the billing system to securely talk to external payment gateways, generate checkout links, and process incoming webhook events.

## Design Philosophy

The system was built with three core principles:

1.  **Data Integrity First**: Monetary calculations are handled using strict, minor-unit arithmetic to avoid floating-point errors.
2.  **State Machine Driven**: Invoices and subscriptions transition through strict, predictable phases via an embedded Finite State Machine (FSM). 
3.  **Event-Driven Ledger**: Financial state changes are recorded as append-only events, providing a complete, immutable audit trail for every transaction.

## Who is this for?

This system is built for operators and developers who need a highly scalable, self-hostable billing solution. Rather than building subscription logic and webhook handlers from scratch inside your core product, you deploy Gebeta Billing alongside it and interact with its clean, RESTful HTTP API.

---

**Next Steps:**
*   Ready to install? Head over to the **[Getting Started](getting-started.md)** guide.
*   Want to see how it works under the hood? Check out the **[System Architecture](architecture.md)**.
