import helmet from 'helmet';
import cors from 'cors';
import { config } from '../../config/index.js';

export function securityMiddleware() {
  return [
    helmet({
      contentSecurityPolicy: false, // SPA CSP later at Nginx
      crossOriginEmbedderPolicy: false,
    }),
    cors({
      origin: config.corsOrigin,
      methods: ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'],
      allowedHeaders: [
        'Content-Type',
        'Authorization',
        'X-Request-ID',
        'Idempotency-Key',
      ],
      exposedHeaders: ['X-Request-ID'],
      maxAge: 600,
    }),
  ];
}