import { ValidationError } from './errors.js';

/**
 * Small, dependency-free request validation.
 *
 * The BFF validates *shape*: presence, types, obvious bounds. Business rules
 * (valid tariff types, tier ordering, account state, ...) stay with the
 * billing service, which is the authority and answers with a 4xx we relay.
 *
 * Only fields that a parser explicitly reads are returned, so anything else a
 * client sends is dropped rather than forwarded upstream.
 */

const ID_PATTERN = /^[1-9]\d*$/;
const CURRENCY_PATTERN = /^[A-Za-z]{3}$/;
const ISO_TIMESTAMP_PATTERN =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})$/;

const isPlainObject = (value) =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

/** Parse a positive integer path parameter such as `:accountId`. */
export function parseId(raw, name) {
  if (
    typeof raw !== 'string' ||
    !ID_PATTERN.test(raw) ||
    !Number.isSafeInteger(Number(raw))
  ) {
    throw new ValidationError([{ field: name, message: 'must be a positive integer' }]);
  }
  return Number(raw);
}

/**
 * Validate a JSON request body. `build` receives a `Fields` reader and returns
 * the parsed value; if any field reported an issue, a ValidationError listing
 * all of them is thrown.
 */
export function parseBody(body, build) {
  if (!isPlainObject(body)) {
    throw new ValidationError([{ field: 'body', message: 'must be a JSON object' }]);
  }
  const issues = [];
  const value = build(new Fields(body, issues, ''));
  if (issues.length > 0) throw new ValidationError(issues);
  return value;
}

export class Fields {
  constructor(source, issues, prefix) {
    this.source = source;
    this.issues = issues;
    this.prefix = prefix;
  }

  // `null` is treated the same as "not provided".
  #raw(key) {
    const value = this.source[key];
    return value === null ? undefined : value;
  }

  #fail(key, message) {
    this.issues.push({ field: `${this.prefix}${key}`, message });
    return undefined;
  }

  #missing(key, required) {
    return required ? this.#fail(key, 'is required') : undefined;
  }

  string(key, { required = false, max } = {}) {
    const raw = this.#raw(key);
    if (raw === undefined) return this.#missing(key, required);
    if (typeof raw !== 'string') return this.#fail(key, 'must be a string');
    const value = raw.trim();
    if (value === '') return this.#missing(key, required);
    if (max !== undefined && value.length > max) {
      return this.#fail(key, `must be at most ${max} characters`);
    }
    return value;
  }

  integer(key, { required = false, min, max } = {}) {
    const raw = this.#raw(key);
    if (raw === undefined) return this.#missing(key, required);
    if (typeof raw !== 'number' || !Number.isInteger(raw)) {
      return this.#fail(key, 'must be an integer');
    }
    if (min !== undefined && raw < min) return this.#fail(key, `must be at least ${min}`);
    if (max !== undefined && raw > max) return this.#fail(key, `must be at most ${max}`);
    return raw;
  }

  /** Any JSON value, passed through untouched (free-form bags like `metadata`). */
  json(key, { required = false } = {}) {
    const raw = this.#raw(key);
    return raw === undefined ? this.#missing(key, required) : raw;
  }

  /** ISO 8601 date-time with an explicit offset, normalised to UTC. */
  timestamp(key, { required = false } = {}) {
    const raw = this.#raw(key);
    if (raw === undefined) return this.#missing(key, required);
    if (
      typeof raw !== 'string' ||
      !ISO_TIMESTAMP_PATTERN.test(raw) ||
      Number.isNaN(Date.parse(raw))
    ) {
      return this.#fail(key, 'must be an ISO 8601 date-time, e.g. 2026-01-31T00:00:00Z');
    }
    return new Date(raw).toISOString();
  }

  /** Three-letter currency code, upper-cased. */
  currency(key, { required = false } = {}) {
    const raw = this.#raw(key);
    if (raw === undefined) return this.#missing(key, required);
    if (typeof raw !== 'string' || !CURRENCY_PATTERN.test(raw.trim())) {
      return this.#fail(key, 'must be a three-letter currency code');
    }
    return raw.trim().toUpperCase();
  }

  /** A nested object, returned as its own reader so issue paths stay accurate. */
  nested(key, { required = false } = {}) {
    const raw = this.#raw(key);
    if (raw === undefined) return this.#missing(key, required);
    if (!isPlainObject(raw)) return this.#fail(key, 'must be an object');
    return new Fields(raw, this.issues, `${this.prefix}${key}.`);
  }

  /** `{ amount_minor, currency }` */
  money(key, { required = false } = {}) {
    const inner = this.nested(key, { required });
    if (!inner) return undefined;
    return {
      amount_minor: inner.integer('amount_minor', {
        required: true,
        min: 0,
        max: Number.MAX_SAFE_INTEGER,
      }),
      currency: inner.currency('currency', { required: true }),
    };
  }

  /** An array of objects; `item` is called with a reader per element. */
  list(key, { required = false, min = 0, max = 100, item }) {
    const raw = this.#raw(key);
    if (raw === undefined) return this.#missing(key, required);
    if (!Array.isArray(raw)) return this.#fail(key, 'must be an array');
    if (raw.length < min) return this.#fail(key, `must contain at least ${min} item(s)`);
    if (raw.length > max) return this.#fail(key, `must contain at most ${max} item(s)`);

    const out = [];
    raw.forEach((element, index) => {
      if (!isPlainObject(element)) {
        this.#fail(`${key}[${index}]`, 'must be an object');
        return;
      }
      const reader = new Fields(element, this.issues, `${this.prefix}${key}[${index}].`);
      out.push(item(reader, index));
    });
    return out;
  }
}
