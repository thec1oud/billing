import { Router } from 'express';
import { createInvoicesHandler } from './handler.js';
import { createInvoicesService } from './service.js';

export function createInvoicesRouter({ billing, enableDevEndpoints = false }) {
  const handler = createInvoicesHandler({ service: createInvoicesService({ billing }) });
  const router = Router();

  router.post('/invoices/:invoiceId/pay', handler.pay);
  if (enableDevEndpoints) {
    router.post('/dev/invoices/generate', handler.generateDev);
  }

  return router;
}
