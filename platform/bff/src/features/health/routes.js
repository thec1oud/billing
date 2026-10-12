import { Router } from 'express';
import { config } from '../../config/index.js';

const router = Router();

/** Liveness – process is up */
router.get('/health', (_req, res) => {
  res.status(200).json({
    status: 'ok',
    service: 'platform-bff',
    time: new Date().toISOString(),
  });
});

/** Readiness – process + upstream billing service */
router.get('/ready', async (_req, res) => {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 3000);

  try {
    const upstream = await fetch(
      `${config.billingServiceUrl}/api/v1/plans`,
      {
        method: 'GET',
        signal: controller.signal,
        headers: { Accept: 'application/json' },
      }
    );
    clearTimeout(timeout);

    const ok = upstream.status < 500;
    res.status(ok ? 200 : 503).json({
      status: ok ? 'ready' : 'degraded',
      billing_service: {
        url: config.billingServiceUrl,
        status: upstream.status,
      },
    });
  } catch (err) {
    clearTimeout(timeout);
    res.status(503).json({
      status: 'not_ready',
      billing_service: {
        url: config.billingServiceUrl,
        error: err.name === 'AbortError' ? 'timeout' : err.message,
      },
    });
  }
});

export default router;