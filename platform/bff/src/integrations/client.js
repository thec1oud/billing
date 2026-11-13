import { createHttpClient } from './http.js';

/**
 * Typed-by-convention client for the Go billing API (/api/v1).
 * One method per upstream endpoint the platform uses. Bodies are sent exactly
 * as given; building them is the calling service's job.
 *
 * Every method takes an optional trailing `ctx` ({ requestId }).
 */
export function createBillingClient(options) {
  const http = createHttpClient(options);

  return {
    accounts: {
      create: (body, ctx) => http.post('/accounts', body, ctx),
      activate: (accountId, ctx) => http.post(`/accounts/${accountId}/activate`, undefined, ctx),
    },

    tariffs: {
      create: (body, ctx) => http.post('/tariffs', body, ctx),
    },

    plans: {
      list: (ctx) => http.get('/plans', ctx),
      create: (body, ctx) => http.post('/plans', body, ctx),
    },

    subscriptions: {
      create: (body, ctx) => http.post('/subscriptions', body, ctx),
      listForAccount: (accountId, ctx) => http.get(`/accounts/${accountId}/subscriptions`, ctx),
    },

    invoices: {
      pay: (invoiceId, body, ctx) => http.post(`/invoices/${invoiceId}/pay`, body, ctx),
      generateDev: (body, ctx) => http.post('/dev/invoices/generate', body, ctx),
    },

    webhooks: {
      deliver: (provider, body, ctx) =>
        http.post(`/webhooks/${encodeURIComponent(provider)}`, body, ctx),
    },
  };
}
