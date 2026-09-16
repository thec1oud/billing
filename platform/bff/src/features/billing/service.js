import { createProxyMiddleware } from 'http-proxy-middleware';
import { config } from '../../config/index.js';

/**
 * Billing domain: reverse-proxy client to the Go billing service.
 * Paths are preserved (/api/v1/* → upstream /api/v1/*).
 */
export function createUpstreamProxy() {
  return createProxyMiddleware({
    target: config.billingServiceUrl,
    changeOrigin: true,
    proxyTimeout: config.proxy.timeoutMs,
    timeout: config.proxy.timeoutMs,
    on: {
      proxyReq: (proxyReq, req) => {
        if (req.requestId) {
          proxyReq.setHeader('X-Request-ID', req.requestId);
        }
        proxyReq.setHeader('X-Forwarded-By', 'platform-bff');
      },
      proxyRes: (proxyRes) => {
        delete proxyRes.headers['x-powered-by'];
        delete proxyRes.headers['server'];
      },
      error: (err, req, res) => {
        console.error(`[BFF] Proxy error (${req.requestId}):`, err.message);
        if (!res.headersSent) {
          res.status(502).json({
            code: 'BAD_GATEWAY',
            message: 'Billing service is temporarily unavailable',
            request_id: req.requestId,
          });
        }
      },
    },
  });
}