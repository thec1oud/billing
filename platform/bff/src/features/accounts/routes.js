import { Router } from 'express';
import { createAccountsHandler } from './handler.js';
import { createAccountsService } from './service.js';

export function createAccountsRouter({ billing }) {
  const handler = createAccountsHandler({ service: createAccountsService({ billing }) });
  const router = Router();

  router.post('/accounts', handler.create);
  router.post('/accounts/:accountId/activate', handler.activate);

  return router;
}
