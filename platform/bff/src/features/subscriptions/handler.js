import { requestContext } from '../../shared/http/context.js';
import { created, ok } from '../../shared/http/respond.js';
import { parseBody, parseId } from '../../shared/http/validate.js';

// Billing period fields are intentionally not accepted: the billing service
// always derives them itself when a subscription is created.
export function parseCreateSubscription(body) {
  return parseBody(body, (f) => ({
    account_id: f.integer('account_id', { required: true, min: 1 }),
    plan_id: f.integer('plan_id', { required: true, min: 1 }),
    plan_version: f.integer('plan_version', { required: true, min: 1 }),
    quantity: f.integer('quantity', { min: 1, max: 2147483647 }),
  }));
}

export function createSubscriptionsHandler({ service }) {
  return {
    create: async (req, res) => {
      const input = parseCreateSubscription(req.body);
      created(res, await service.create(input, requestContext(req)));
    },

    listForAccount: async (req, res) => {
      const accountId = parseId(req.params.accountId, 'accountId');
      ok(res, await service.listForAccount(accountId, requestContext(req)));
    },
  };
}
