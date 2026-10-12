import { errorHandler } from '../src/shared/middleware/errorHandler.js';

/** Minimal stand-in for an Express response. */
export function makeRes() {
  return {
    statusCode: 200,
    body: undefined,
    headersSent: false,
    status(code) {
      this.statusCode = code;
      return this;
    },
    json(payload) {
      this.body = payload;
      this.headersSent = true;
      return this;
    },
  };
}

/**
 * Run a route handler like Express 5 would: a thrown/rejected error is passed
 * to the error handler. Returns `{ status, body }`.
 */
export async function invoke(handler, { params = {}, body, requestId = 'req-test-1' } = {}) {
  const req = { params, body, requestId };
  const res = makeRes();
  try {
    await handler(req, res);
  } catch (err) {
    errorHandler(err, req, res, () => {});
  }
  return { status: res.statusCode, body: res.body };
}
