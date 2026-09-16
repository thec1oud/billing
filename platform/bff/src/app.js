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

import { healthRouter } from './features/health/index.js';
import { billingRouter } from './features/billing/index.js';

export function createApp() {
  const app = express();

  if (config.trustProxy) {
    app.set('trust proxy', 1);
  }

  // Global middleware (order matters)
  app.use(...securityMiddleware());
  app.use(loggerMiddleware());
  app.use(requestIdMiddleware);
  app.use('/api/', apiRateLimitMiddleware());


  app.use('/api', healthRouter);       // /api/health, /api/ready
  app.use('/api/v1', billingRouter);   // proxy → Go billing service

  // Fallback
  app.use(notFoundHandler);
  app.use(errorHandler);

  return app;
}