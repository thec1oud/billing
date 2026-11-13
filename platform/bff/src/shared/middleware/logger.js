import morgan from 'morgan';
import { config } from '../../config/index.js';

export function loggerMiddleware() {
  return morgan(config.isProd ? 'combined' : 'dev', {
    skip: (req) =>
      req.url === '/api/health' || req.url === '/api/ready',
  });
}