import { config } from '../../config/index.js';
import { HttpError } from '../http/errors.js';
import { BillingApiError } from '../../integrations/errors.js';

export function notFoundHandler(req, res) {
  res.status(404).json({
    code: 'NOT_FOUND',
    message: `No route for ${req.method} ${req.path}`,
    request_id: req.requestId,
  });
}

/**
 * What the browser should see when a call to the billing service fails.
 * - 4xx: the billing service is telling the caller what is wrong (validation,
 *   not found, conflict, ...), so status, code and message are relayed.
 * - 5xx / unusable answers: an upstream fault; the details stay in the logs
 *   (and in the response outside production).
 */
export function fromBillingError(err) {
  switch (err.kind) {
    case 'timeout':
      return new HttpError(504, 'GATEWAY_TIMEOUT', 'Billing service timed out');
    case 'network':
      return new HttpError(502, 'BAD_GATEWAY', 'Billing service is temporarily unavailable');
    case 'http':
      if (err.status >= 400 && err.status < 500) {
        return new HttpError(err.status, err.code ?? 'REQUEST_REJECTED', err.message);
      }
    // falls through
    default:
      return new HttpError(
        502,
        'UPSTREAM_ERROR',
        config.isProd ? 'Billing service failed to process the request' : err.message
      );
  }
}

function toHttpError(err) {
  if (err instanceof HttpError) return err;
  if (err instanceof BillingApiError) return fromBillingError(err);
  // body-parser (express.json) failures
  if (err?.type === 'entity.parse.failed') {
    return new HttpError(400, 'INVALID_JSON', 'Request body is not valid JSON');
  }
  if (err?.type === 'entity.too.large') {
    return new HttpError(413, 'PAYLOAD_TOO_LARGE', 'Request body is too large');
  }
  return null;
}

export function errorHandler(err, req, res, next) {
  if (res.headersSent) return next(err);

  const httpError = toHttpError(err);

  if (!httpError) {
    console.error(`[BFF] Unhandled error (${req.requestId}):`, err);
    return res.status(500).json({
      code: 'INTERNAL_ERROR',
      message: config.isProd ? 'Internal server error' : err.message,
      request_id: req.requestId,
    });
  }

  if (httpError.status >= 500) {
    console.error(
      `[BFF] ${httpError.code} (${req.requestId}): ${err.message}`,
      err instanceof BillingApiError ? { kind: err.kind, upstream_status: err.status } : ''
    );
  }

  const body = {
    code: httpError.code,
    message: httpError.message,
    request_id: req.requestId,
  };
  if (httpError.details) body.details = httpError.details;

  return res.status(httpError.status).json(body);
}
