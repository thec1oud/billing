import { Router } from 'express';
import { createPlansHandler } from './handler.js';
import { createPlansService } from './service.js';

export function createPlansRouter({ billing }) {
  const handler = createPlansHandler({ service: createPlansService({ billing }) });
  const router = Router();

  router.get('/plans', handler.list);
  router.post('/plans', handler.create);

  return router;
}
