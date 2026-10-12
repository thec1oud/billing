import { requestContext } from '../../shared/http/context.js';
import { created, ok } from '../../shared/http/respond.js';
import { parseBody } from '../../shared/http/validate.js';

export function parseCreatePlan(body) {
  return parseBody(body, (f) => ({
    plan_code: f.string('plan_code', { required: true, max: 100 }),
    legacy_price_policy_code: f.string('legacy_price_policy_code', { required: true }),
    effective_from: f.timestamp('effective_from'),
    effective_until: f.timestamp('effective_until'),
    migration_path: f.json('migration_path'),
    metadata: f.json('metadata'),
    // `duration` is in nanoseconds (Go time.Duration).
    durations: f.list('durations', {
      required: true,
      min: 1,
      item: (d) => ({
        tariff_id: d.integer('tariff_id', { required: true, min: 1 }),
        duration: d.integer('duration', { required: true, min: 1 }),
      }),
    }),
  }));
}

export function createPlansHandler({ service }) {
  return {
    list: async (req, res) => {
      ok(res, await service.list(requestContext(req)));
    },

    create: async (req, res) => {
      const input = parseCreatePlan(req.body);
      created(res, await service.create(input, requestContext(req)));
    },
  };
}
