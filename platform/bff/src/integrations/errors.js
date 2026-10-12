/**
 * A failed call to the Go billing service.
 *
 * kind:
 *   'http'             upstream answered with a non-2xx status
 *   'timeout'          no complete response within the configured timeout
 *   'network'          connection refused / reset / DNS failure
 *   'invalid_response' 2xx answer whose body could not be parsed
 */
export class BillingApiError extends Error {
  constructor({ kind, status = null, code = null, message, cause }) {
    super(message, cause ? { cause } : undefined);
    this.name = 'BillingApiError';
    this.kind = kind;
    this.status = status;
    this.code = code;
  }
}
