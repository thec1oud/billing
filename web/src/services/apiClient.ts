import {
  Account,
  CreateAccountInput,
  Plan,
  PurchasableItem,
  Subscription,
  CreateSubscriptionInput,
  Invoice,
  CreateDraftInvoiceInput,
  ChargeRequest,
  ChargeResult,
  ApiLogEntry,
} from '../types/api'

// Subscribable logger store for API Inspector
type LogListener = (logs: ApiLogEntry[]) => void
let logs: ApiLogEntry[] = []
const listeners: Set<LogListener> = new Set()

export function subscribeLogs(listener: LogListener): () => void {
  listeners.add(listener)
  listener(logs)
  return () => {
    listeners.delete(listener)
  }
}

export function clearLogs(): void {
  logs = []
  listeners.forEach((l) => l(logs))
}

async function request<T>(
  method: string,
  url: string,
  body?: unknown
): Promise<{ data: T; status: number }> {
  const startTime = performance.now()
  const logId = Math.random().toString(36).substring(2, 9)

  const entry: ApiLogEntry = {
    id: logId,
    timestamp: new Date().toLocaleTimeString(),
    method,
    url,
    durationMs: 0,
    requestBody: body,
    isError: false,
  }

  try {
    const res = await fetch(url, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    })

    const duration = Math.round(performance.now() - startTime)
    entry.durationMs = duration
    entry.status = res.status
    entry.statusText = res.statusText

    let responseData: any = null
    const text = await res.text()
    if (text) {
      try {
        responseData = JSON.parse(text)
      } catch {
        responseData = text
      }
    }

    entry.responseBody = responseData
    entry.isError = !res.ok

    logs = [entry, ...logs].slice(0, 50)
    listeners.forEach((l) => l(logs))

    if (!res.ok) {
      let errorMsg = `HTTP ${res.status}: ${res.statusText}`
      if (typeof responseData === 'object' && responseData !== null) {
        if (responseData.error?.message) {
          errorMsg = responseData.error.message
        } else if (responseData.error?.code) {
          errorMsg = `${responseData.error.code}: ${responseData.error.message || 'error'}`
        } else if (responseData.message) {
          errorMsg = responseData.message
        }
      }
      throw new Error(errorMsg)
    }

    // Unwrap standardized backend envelope: { success: true, data: ... }
    let payload = responseData
    if (
      responseData &&
      typeof responseData === 'object' &&
      'data' in responseData &&
      'success' in responseData
    ) {
      payload = responseData.data
    }

    return { data: (payload ?? null) as T, status: res.status }
  } catch (err: any) {
    if (!entry.status) {
      entry.durationMs = Math.round(performance.now() - startTime)
      entry.isError = true
      entry.responseBody = { error: err.message || 'Network request failed' }
      logs = [entry, ...logs].slice(0, 50)
      listeners.forEach((l) => l(logs))
    }
    throw err
  }
}

// ---------------------------------------------------------------------------
// Backend <-> frontend adapters
//
// The Go backend's JSON field names and validation rules don't line up
// 1:1 with the frontend types (e.g. Plan uses `plan_id` not `id`, Account
// has no `name`/`email`, Subscription requires an exact `plan_version`
// rather than the `plan_duration_id`/`auto_renew` this form collects).
// Rather than rewriting every component's field names, the functions
// below translate on the way out (build the payload the Go handler
// actually expects) and on the way in (normalize the response into the
// shape the components already render).
// ---------------------------------------------------------------------------

function normalizeAccount(raw: any): Account {
  const metadata = raw && typeof raw.metadata === 'object' ? raw.metadata : {}
  return {
    id: raw?.id,
    name: metadata.name || raw?.external_id || `Account #${raw?.id ?? ''}`,
    email: metadata.email || raw?.external_id || '',
    currency: raw?.currency,
    country_code: metadata.country_code || '',
    // Backend's real status is PENDING_VERIFICATION; the UI's "Activate"
    // button looks for PENDING_ACTIVATION, so map it here.
    status: raw?.status === 'PENDING_VERIFICATION' ? 'PENDING_ACTIVATION' : raw?.status,
    created_at: raw?.created_at,
  }
}

function normalizePlan(raw: any): Plan {
  return { ...raw, id: raw?.plan_id ?? raw?.id }
}

function normalizePurchasableItem(raw: any): PurchasableItem {
  return { ...raw, id: raw?.item_id ?? raw?.id }
}

function normalizeSubscription(raw: any, requested: CreateSubscriptionInput): Subscription {
  return {
    id: raw?.id,
    account_id: raw?.account_id,
    plan_id: raw?.plan_id,
    // Backend doesn't track a duration id; echo back what was requested
    // so the UI has something stable to display.
    plan_duration_id: requested.plan_duration_id,
    status: raw?.status,
    auto_renew: !raw?.cancel_at_period_end,
    current_period_start: raw?.current_period_start,
    current_period_end: raw?.current_period_end,
    created_at: raw?.current_period_start,
  }
}

function normalizeInvoice(raw: any): Invoice {
  return {
    id: raw?.invoice_id ?? raw?.id,
    account_id: raw?.account_id,
    invoice_number: raw?.invoice_number,
    status: raw?.status,
    currency: raw?.currency,
    subtotal_minor: raw?.subtotal?.amount_minor ?? raw?.subtotal_minor ?? 0,
    tax_minor: raw?.tax?.amount_minor ?? raw?.tax_minor ?? 0,
    total_minor: raw?.total?.amount_minor ?? raw?.total_minor ?? 0,
    due_date: raw?.due_at ?? raw?.due_date,
    // Backend doesn't return a created_at on Invoice; fall back to "now"
    // purely for display ordering.
    created_at: raw?.created_at ?? new Date().toISOString(),
    lines: raw?.line_items ?? raw?.lines,
  }
}

function normalizeChargeResult(raw: any): ChargeResult {
  return {
    provider_tx_id: raw?.provider_reference ?? raw?.provider_tx_id,
    checkout_url: raw?.checkout_url,
    status: raw?.status,
    error: raw?.failure_code ?? raw?.error,
  }
}

export const billingApi = {
  // Accounts
  createAccount: async (input: CreateAccountInput) => {
    const payload = {
      external_id: input.email,
      currency: input.currency,
      timezone: 'UTC',
      locale: 'en-US',
      metadata: {
        name: input.name,
        email: input.email,
        country_code: input.country_code,
      },
    }
    const res = await request<any>('POST', '/api/v1/accounts', payload)
    return { data: normalizeAccount(res.data), status: res.status }
  },

  activateAccount: async (id: number) => {
    const res = await request<any>('POST', `/api/v1/accounts/${id}/activate`)
    return { data: normalizeAccount(res.data), status: res.status }
  },

  // Catalog
  listPlans: async () => {
    const res = await request<any[]>('GET', '/api/v1/plans')
    return { data: (res.data || []).map(normalizePlan), status: res.status }
  },

  listPurchasableItems: async () => {
    const res = await request<any[]>('GET', '/api/v1/purchasable-items')
    return { data: (res.data || []).map(normalizePurchasableItem), status: res.status }
  },

  // Subscriptions
  createSubscription: async (input: CreateSubscriptionInput) => {
    // The backend requires the plan's exact immutable `plan_version`,
    // which this form doesn't collect directly (it only knows plan_id).
    // Resolve it from the live catalog so creation isn't rejected with
    // "plan version must be greater than zero".
    let planVersion = 1
    try {
      const plans = await request<any[]>('GET', '/api/v1/plans')
      const match = (plans.data || []).find(
        (p) => (p.plan_id ?? p.id) === input.plan_id
      )
      if (match?.version) planVersion = match.version
    } catch {
      // Fall through with the default version; the backend will reject
      // the request with a clear 400 if it's wrong, instead of a blind 500.
    }

    const payload = {
      account_id: input.account_id,
      plan_id: input.plan_id,
      plan_version: planVersion,
    }

    const res = await request<any>('POST', '/api/v1/subscriptions', payload)
    return { data: normalizeSubscription(res.data, input), status: res.status }
  },

  // Invoices
  createDraftInvoice: async (input: CreateDraftInvoiceInput) => {
    const res = await request<any>('POST', '/api/v1/invoices', input)
    return { data: normalizeInvoice(res.data), status: res.status }
  },

  finalizeInvoice: async (id: number) => {
    const res = await request<any>('POST', `/api/v1/invoices/${id}/finalize`)
    return { data: normalizeInvoice(res.data), status: res.status }
  },

  getInvoice: async (id: number) => {
    const res = await request<any>('GET', `/api/v1/invoices/${id}`)
    return { data: normalizeInvoice(res.data), status: res.status }
  },

  // PPI Payments
  attemptPayment: async (input: ChargeRequest) => {
    const res = await request<any>('POST', '/api/v1/payments/charge', input)
    return { data: normalizeChargeResult(res.data), status: res.status }
  },

  sendWebhook: (provider: string, payload: Record<string, unknown>) =>
    request<Record<string, unknown>>('POST', `/api/v1/webhooks/${provider}`, payload),
}

