/**
 * ============================================================================
 * UI WORKFLOW & INTEGRATION GUIDE (For UI Developers & LLMs)
 * ============================================================================
 * 
 * This file contains the frontend service functions for the Billing API. 
 * The backend already has seeded plans and tariffs. 
 * 
 * To build a complete onboarding and checkout flow, follow this sequence:
 * 
 * 1. ACCOUNT CREATION & ACTIVATION:
 *    - Call `accountService.create(...)` with the user's currency and timezone.
 *    - Immediately call `accountService.activate(accountId)` to transition the 
 *      account to an ACTIVE state.
 * 
 * 2. PLAN SUBSCRIPTION:
 *    - Present the seeded plans to the user.
 *    - When the user selects a plan, call `subscriptionService.create(...)` 
 *      using the AccountID and the selected PlanID.
 * 
 * 3. DEV INVOICE GENERATION:
 *    - In this dev environment, subscriptions don't automatically generate bills instantly.
 *    - Immediately after subscription creation, call `invoiceService.generateDevInvoice(...)` 
 *      passing the AccountID and the ItemID tied to the plan. This forces the 
 *      creation of an OPEN invoice for testing.
 * 
 * 4. CHECKOUT & PAYMENT:
 *    - Display the OPEN invoice to the user (Amount Due, Line Items).
 *    - When the user clicks "Pay", generate a unique `idempotency_key` (e.g., UUID)
 *      and call `invoiceService.pay(invoiceId, { provider_code: "fake", idempotency_key })`.
 *    - **CRITICAL UI STATE**: The `pay` response contains `internal_tx_id` and 
 *      `provider_reference`. Save these locally (e.g., in localStorage or global state) 
 *      along with the invoice amount. You will need them to simulate the webhook!
 * 
 * 5. WEBHOOK SIMULATION (The "Fake Provider" Flow):
 *    - Build a dedicated "Dev Webhook Simulator" page in the UI.
 *    - On this page, read the saved payment attempts from local state.
 *    - For each pending payment, provide a button that calls 
 *      `webhookService.simulateFakeProvider(...)`.
 *    - Construct the `FakeWebhookPayload` using the saved `internal_tx_id` (as `tx_ref`) 
 *      and `provider_reference` (as `reference`).
 *    - This simulates the payment provider (e.g., Stripe) asynchronously telling 
 *      our backend that the payment succeeded, which will mark the invoice as PAID.
 * ============================================================================
 */

import type {
  Account,
  CreateAccountInput,
  Tariff,
  CreateTariffInput,
  Plan,
  Subscription,
  CreateSubscriptionInput,
  Invoice,
  PayInvoiceInput,
  PayInvoiceResponse,
  DevGenerateInvoiceInput,
  FakeWebhookPayload,
  APIErrorResponse,
} from "./types";

const API_BASE_URL = "/api/v1";

export class APIError extends Error {
  public code: string;
  public status: number;

  constructor(status: number, errorResponse: APIErrorResponse) {
    super(errorResponse.message);
    this.name = "APIError";
    this.status = status;
    this.code = errorResponse.code;
  }
}

async function request<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    let errorData: APIErrorResponse;
    try {
      errorData = await response.json();
    } catch (err) {
      errorData = {
        code: "UNKNOWN_ERROR",
        message: "An unknown error occurred",
      };
    }
    throw new APIError(response.status, errorData);
  }

  const jsonResponse = await response.json();
  if (jsonResponse && "data" in jsonResponse) {
    return jsonResponse.data as T;
  }

  // Fallback just in case some endpoints don't wrap (though the test assumes all 2xx do)
  return jsonResponse as T;
}

export const accountService = {
  create: (input: CreateAccountInput): Promise<Account> => {
    return request<Account>("/accounts", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },

  activate: (accountId: number): Promise<Account> => {
    return request<Account>(`/accounts/${accountId}/activate`, {
      method: "POST",
    });
  },
};

export const tariffService = {
  create: (input: CreateTariffInput): Promise<Tariff> => {
    return request<Tariff>("/tariffs", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
};

export const planService = {
  // Plan creation actually sends `plan.Plan` struct, omitting ID and metadata in the test
  create: (input: Plan): Promise<Plan> => {
    return request<Plan>("/plans", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },

  list: (): Promise<Plan[]> => {
    return request<Plan[]>("/plans", {
      method: "GET",
    });
  },
};

export const subscriptionService = {
  create: (input: CreateSubscriptionInput): Promise<Subscription> => {
    return request<Subscription>("/subscriptions", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },

  listForAccount: (accountId: number): Promise<Subscription[]> => {
    return request<Subscription[]>(`/accounts/${accountId}/subscriptions`, {
      method: "GET",
    });
  },
};

export const invoiceService = {
  pay: (invoiceId: number, input: PayInvoiceInput): Promise<PayInvoiceResponse> => {
    return request<PayInvoiceResponse>(`/invoices/${invoiceId}/pay`, {
      method: "POST",
      body: JSON.stringify(input),
    });
  },

  generateDevInvoice: (input: DevGenerateInvoiceInput): Promise<Invoice> => {
    return request<Invoice>("/dev/invoices/generate", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
};

export const webhookService = {
  simulateFakeProvider: (payload: FakeWebhookPayload): Promise<void> => {
    return request<void>("/webhooks/fake", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },
};
