import { Router } from 'express';
import { createSubscriptionsHandler } from './handler.js';
import { createSubscriptionsService } from './service.js';

export function createSubscriptionsRouter({ billing }) {
  const handler = createSubscriptionsHandler({ service: createSubscriptionsService({ billing }) });
  const router = Router();

  router.post('/subscriptions', handler.create);
  router.get('/accounts/:accountId/subscriptions', handler.listForAccount);

  return router;
}
