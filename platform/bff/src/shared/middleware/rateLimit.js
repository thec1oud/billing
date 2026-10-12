import rateLimit from 'express-rate-limit';
import { config } from '../../config/index.js';

export function apiRateLimitMiddleware() {
  return rateLimit({
    windowMs: config.rateLimit.windowMs,
    max: config.rateLimit.max,
    standardHeaders: true,
    legacyHeaders: false,
    message: {
      code: 'RATE_LIMITED',
      message: 'Too many requests. Please try again later.',
    },
    keyGenerator: (req) => req.ip || 'unknown',
  });
}