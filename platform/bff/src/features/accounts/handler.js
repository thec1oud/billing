import { requestContext } from '../../shared/http/context.js';
import { created, ok } from '../../shared/http/respond.js';
import { parseBody, parseId } from '../../shared/http/validate.js';

export function parseCreateAccount(body) {
  return parseBody(body, (f) => ({
    external_id: f.string('external_id', { max: 255 }),
    currency: f.currency('currency', { required: true }),
    timezone: f.string('timezone', { max: 64 }),
    locale: f.string('locale', { max: 35 }),
    net_terms: f.integer('net_terms', { min: 0, max: 32767 }),
    dunning_profile_id: f.integer('dunning_profile_id', { min: 1 }),
    tax_identifiers: f.json('tax_identifiers'),
    billing_address: f.json('billing_address'),
    compliance_flags: f.json('compliance_flags'),
    metadata: f.json('metadata'),
  }));
}

export function createAccountsHandler({ service }) {
  return {
    create: async (req, res) => {
      const input = parseCreateAccount(req.body);
      created(res, await service.create(input, requestContext(req)));
    },

    activate: async (req, res) => {
      const accountId = parseId(req.params.accountId, 'accountId');
      ok(res, await service.activate(accountId, requestContext(req)));
    },
  };
}
