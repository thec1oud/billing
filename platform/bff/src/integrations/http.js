import { BillingApiError } from './errors.js';

const API_PREFIX = '/api/v1';

/**
 * Minimal JSON-over-HTTP transport for the Go billing API.
 *
 * - Unwraps the Go `{ success, data }` envelope and returns `data`.
 * - Some endpoints (provider webhooks) answer with a bare JSON object; that is
 *   returned as-is.
 * - Failures are always thrown as BillingApiError.
 * - No retries: most calls are non-idempotent POSTs.
 */
export function createHttpClient({ baseUrl, timeoutMs, fetchImpl = globalThis.fetch }) {
  const root = `${baseUrl.replace(/\/+$/, '')}${API_PREFIX}`;

  async function request(method, path, { body, ctx } = {}) {
    const headers = { Accept: 'application/json', 'X-Forwarded-By': 'platform-bff' };
    if (ctx?.requestId) headers['X-Request-ID'] = ctx.requestId;

    let payload;
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
      payload = JSON.stringify(body);
    }

    let response;
    let text;
    try {
      response = await fetchImpl(`${root}${path}`, {
        method,
        headers,
        body: payload,
        signal: AbortSignal.timeout(timeoutMs),
      });
      text = await response.text();
    } catch (cause) {
      if (cause?.name === 'TimeoutError' || cause?.name === 'AbortError') {
        throw new BillingApiError({
          kind: 'timeout',
          message: `billing service did not respond within ${timeoutMs}ms`,
          cause,
        });
      }
      throw new BillingApiError({
        kind: 'network',
        message: `billing service unreachable: ${cause?.cause?.code ?? cause?.message}`,
        cause,
      });
    }

    let json;
    let parsed = false;
    if (text !== '') {
      try {
        json = JSON.parse(text);
        parsed = true;
      } catch {
        // handled below depending on status
      }
    }

    if (!response.ok) {
      const upstream = parsed && json && typeof json === 'object' ? json.error : undefined;
      throw new BillingApiError({
        kind: 'http',
        status: response.status,
        code: typeof upstream?.code === 'string' ? upstream.code : null,
        message:
          typeof upstream?.message === 'string'
            ? upstream.message
            : `billing service returned HTTP ${response.status}`,
      });
    }

    if (text !== '' && !parsed) {
      throw new BillingApiError({
        kind: 'invalid_response',
        status: response.status,
        message: 'billing service returned a non-JSON body',
      });
    }

    if (json && typeof json === 'object' && json.success === true) {
      return json.data ?? null;
    }
    return json ?? null;
  }

  return {
    get: (path, ctx) => request('GET', path, { ctx }),
    post: (path, body, ctx) => request('POST', path, { body, ctx }),
  };
}
