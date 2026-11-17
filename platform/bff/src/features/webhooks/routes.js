import { Router } from 'express';
import { createWebhooksHandler } from './handler.js';
import { createWebhooksService } from './service.js';

export function createWebhooksRouter({ billing, enableDevEndpoints = false }) {
  const handler = createWebhooksHandler({ service: createWebhooksService({ billing }) });
  const router = Router();

  if (enableDevEndpoints) {
    router.post('/webhooks/fake', handler.simulateFake);
  }

  return router;
}
