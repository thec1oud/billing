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

export const billingApi = {
  // Accounts
  createAccount: (input: CreateAccountInput) =>
    request<Account>('POST', '/api/v1/accounts', input),

  activateAccount: (id: number) =>
    request<Account>('POST', `/api/v1/accounts/${id}/activate`),

  // Catalog
  listPlans: () => request<Plan[]>('GET', '/api/v1/plans'),

  listPurchasableItems: () =>
    request<PurchasableItem[]>('GET', '/api/v1/purchasable-items'),

  // Subscriptions
  createSubscription: (input: CreateSubscriptionInput) =>
    request<Subscription>('POST', '/api/v1/subscriptions', input),

  // Invoices
  createDraftInvoice: (input: CreateDraftInvoiceInput) =>
    request<Invoice>('POST', '/api/v1/invoices', input),

  finalizeInvoice: (id: number) =>
    request<Invoice>('POST', `/api/v1/invoices/${id}/finalize`),

  getInvoice: (id: number) =>
    request<Invoice>('GET', `/api/v1/invoices/${id}`),

  // PPI Payments
  attemptPayment: (input: ChargeRequest) =>
    request<ChargeResult>('POST', '/api/v1/payments/charge', input),

  sendWebhook: (provider: string, payload: Record<string, unknown>) =>
    request<Record<string, unknown>>('POST', `/api/v1/webhooks/${provider}`, payload),
}

