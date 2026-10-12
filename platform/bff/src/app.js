import express from 'express';
import { config } from './config/index.js';
import { securityMiddleware } from './shared/middleware/security.js';
import { loggerMiddleware } from './shared/middleware/logger.js';
import { requestIdMiddleware } from './shared/middleware/requestId.js';
import { apiRateLimitMiddleware } from './shared/middleware/rateLimit.js';
import {
  notFoundHandler,
  errorHandler,
} from './shared/middleware/errorHandler.js';

import { createBillingClient } from './integrations/index.js';

import { healthRouter } from './features/health/index.js';
import { createAccountsRouter } from './features/accounts/index.js';
import { createTariffsRouter } from './features/tariffs/index.js';
import { createPlansRouter } from './features/plans/index.js';
import { createSubscriptionsRouter } from './features/subscriptions/index.js';
import { createInvoicesRouter } from './features/invoices/index.js';
import { createWebhooksRouter } from './features/webhooks/index.js';

export function createApp({
  billing = createBillingClient({
    baseUrl: config.billingServiceUrl,
    timeoutMs: config.billing.timeoutMs,
  }),
} = {}) {
  const app = express();

  if (config.trustProxy) {
    app.set('trust proxy', 1);
  }

  // Global middleware (order matters)
  app.use(...securityMiddleware());
  app.use(loggerMiddleware());
  app.use(requestIdMiddleware);
  app.use('/api/', apiRateLimitMiddleware());

  app.use('/api', healthRouter); // /api/health, /api/ready

  // Platform API. Each route unpacks and validates the request, calls the Go
  // billing service through the billing client, and returns a shaped response.
  // Only the routes registered here are reachable from the browser.
  const api = express.Router();
  api.use(express.json({ limit: '100kb' }));
  api.use(createAccountsRouter({ billing }));
  api.use(createTariffsRouter({ billing }));
  api.use(createPlansRouter({ billing }));
  api.use(createSubscriptionsRouter({ billing }));
  api.use(createInvoicesRouter({ billing }));
  api.use(createWebhooksRouter({ billing }));
  app.use('/api/v1', api);

  // Fallback
  app.use(notFoundHandler);
  app.use(errorHandler);

  return app;
}
