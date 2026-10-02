import { Router } from 'express';
import { createInvoicesHandler } from './handler.js';
import { createInvoicesService } from './service.js';

export function createInvoicesRouter({ billing }) {
  const handler = createInvoicesHandler({ service: createInvoicesService({ billing }) });
  const router = Router();

  router.post('/invoices/:invoiceId/pay', handler.pay);
  // Demo-only: makes the billing service issue an OPEN invoice for a plan.
  router.post('/dev/invoices/generate', handler.generateDev);

  return router;
}
