export class HttpError extends Error {
  constructor(status, code, message, details) {
    super(message);
    this.name = 'HttpError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

/** `details` is a list of `{ field, message }` entries. */
export class ValidationError extends HttpError {
  constructor(details) {
    super(400, 'VALIDATION_ERROR', 'Request validation failed', details);
    this.name = 'ValidationError';
  }
}
