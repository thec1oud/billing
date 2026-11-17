const NODE_ENV = process.env.NODE_ENV || 'development';
const rawEnableDevEndpoints = process.env.ENABLE_DEV_ENDPOINTS;

function parseEnableDevEndpoints(raw) {
  if (raw === undefined || raw.trim() === '') return false;
  if (raw === 'true') return true;
  if (raw === 'false') return false;
  throw new Error('ENABLE_DEV_ENDPOINTS must be "true" or "false"');
}

function parseCorsOrigin(raw, isProd) {
  if (raw === undefined || raw === '') {
    return isProd ? false : true;
  }
  if (raw === 'true') return true;
  if (raw === 'false') return false;
  if (raw.includes(',')) {
    return raw
      .split(',')
      .map((origin) => origin.trim())
      .filter(Boolean);
  }
  return raw;
}

const isProd = NODE_ENV === 'production';
const enableDevEndpoints = parseEnableDevEndpoints(rawEnableDevEndpoints);

if (isProd && enableDevEndpoints) {
  throw new Error('ENABLE_DEV_ENDPOINTS cannot be enabled when NODE_ENV=production');
}

export const config = {
  env: NODE_ENV,
  isProd,
  enableDevEndpoints,
  port: Number(process.env.PORT || 3000),
  billingServiceUrl:
    process.env.BILLING_SERVICE_URL || 'http://localhost:8080',
  trustProxy:
    process.env.TRUST_PROXY === 'true' || isProd,
  corsOrigin: parseCorsOrigin(process.env.CORS_ORIGIN, isProd),
  rateLimit: {
    windowMs: Number(process.env.RATE_LIMIT_WINDOW_MS || 60_000),
    max: Number(process.env.RATE_LIMIT_MAX || 120),
  },
  billing: {
    // PROXY_TIMEOUT_MS is the pre-refactor name, still honoured.
    timeoutMs: Number(
      process.env.BILLING_TIMEOUT_MS || process.env.PROXY_TIMEOUT_MS || 30_000
    ),
  },
};