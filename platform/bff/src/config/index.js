const NODE_ENV = process.env.NODE_ENV || 'development';

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

export const config = {
  env: NODE_ENV,
  isProd,
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
  proxy: {
    timeoutMs: Number(process.env.PROXY_TIMEOUT_MS || 30_000),
  },
};