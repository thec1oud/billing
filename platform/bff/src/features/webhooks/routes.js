import { Router } from 'express';
import { createWebhooksHandler } from './handler.js';
import { createWebhooksService } from './service.js';

export function createWebhooksRouter({ billing }) {
  const handler = createWebhooksHandler({ service: createWebhooksService({ billing }) });
  const router = Router();

  // Demo-only: simulates the fake payment provider's callback.
  router.post('/webhooks/fake', handler.simulateFake);

  return router;
}
