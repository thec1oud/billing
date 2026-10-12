import { requestContext } from '../../shared/http/context.js';
import { created, ok } from '../../shared/http/respond.js';
import { parseBody, parseId } from '../../shared/http/validate.js';

export function parsePayInvoice(body) {
  return parseBody(body, (f) => ({
    provider_code: f.string('provider_code', { required: true, max: 64 }),
    idempotency_key: f.string('idempotency_key', { required: true, max: 255 }),
  }));
}

export function parseGenerateDevInvoice(body) {
  return parseBody(body, (f) => ({
    account_id: f.integer('account_id', { required: true, min: 1 }),
    plan_id: f.integer('plan_id', { required: true, min: 1 }),
  }));
}

export function createInvoicesHandler({ service }) {
  return {
    pay: async (req, res) => {
      const invoiceId = parseId(req.params.invoiceId, 'invoiceId');
      const input = parsePayInvoice(req.body);
      ok(res, await service.pay(invoiceId, input, requestContext(req)));
    },

    generateDev: async (req, res) => {
      const input = parseGenerateDevInvoice(req.body);
      created(res, await service.generateDev(input, requestContext(req)));
    },
  };
}
