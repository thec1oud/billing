import { config } from '../../config/index.js';

export function notFoundHandler(req, res) {
  res.status(404).json({
    code: 'NOT_FOUND',
    message: `No route for ${req.method} ${req.path}`,
    request_id: req.requestId,
  });
}

export function errorHandler(err, req, res, _next) {
  console.error(`[BFF] Unhandled error (${req.requestId}):`, err);
  res.status(500).json({
    code: 'INTERNAL_ERROR',
    message: config.isProd ? 'Internal server error' : err.message,
    request_id: req.requestId,
  });
}