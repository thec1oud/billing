import { Router } from 'express';
import { createTariffsHandler } from './handler.js';
import { createTariffsService } from './service.js';

export function createTariffsRouter({ billing }) {
  const handler = createTariffsHandler({ service: createTariffsService({ billing }) });
  const router = Router();

  router.post('/tariffs', handler.create);

  return router;
}
