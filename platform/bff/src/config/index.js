const NODE_ENV = process.env.NODE_ENV || 'development';

export const config = {
  env: NODE_ENV,
  isProd: NODE_ENV === 'production',
  port: Number(process.env.PORT || 3000),
  billingServiceUrl:
    process.env.BILLING_SERVICE_URL || 'http://localhost:8080',
  trustProxy:
    process.env.TRUST_PROXY === 'true' || NODE_ENV === 'production',
  corsOrigin:
    process.env.CORS_ORIGIN ||
    (NODE_ENV === 'production' ? false : true),
  rateLimit: {
    windowMs: Number(process.env.RATE_LIMIT_WINDOW_MS || 60_000),
    max: Number(process.env.RATE_LIMIT_MAX || 120),
  },
  proxy: {
    timeoutMs: Number(process.env.PROXY_TIMEOUT_MS || 30_000),
  },
};