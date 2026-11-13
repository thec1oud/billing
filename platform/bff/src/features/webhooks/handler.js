import { requestContext } from '../../shared/http/context.js';
import { ok } from '../../shared/http/respond.js';
import { parseBody } from '../../shared/http/validate.js';

export function parseFakeWebhook(body) {
  return parseBody(body, (f) => ({
    event_id: f.string('event_id', { required: true, max: 255 }),
    event: f.string('event', { required: true, max: 100 }),
    tx_ref: f.string('tx_ref', { required: true, max: 255 }),
    reference: f.string('reference', { required: true, max: 255 }),
    status: f.string('status', { required: true, max: 50 }),
    amount_minor: f.integer('amount_minor', {
      required: true,
      min: 0,
      max: Number.MAX_SAFE_INTEGER,
    }),
    currency: f.currency('currency', { required: true }),
    failure_reason: f.string('failure_reason', { max: 500 }),
    timestamp: f.integer('timestamp', { min: 0 }), // unix seconds
  }));
}

export function createWebhooksHandler({ service }) {
  return {
    simulateFake: async (req, res) => {
      const input = parseFakeWebhook(req.body);
      ok(res, await service.simulateFake(input, requestContext(req)));
    },
  };
}
