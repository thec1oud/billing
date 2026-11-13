import { requestContext } from '../../shared/http/context.js';
import { created } from '../../shared/http/respond.js';
import { parseBody } from '../../shared/http/validate.js';

export function parseCreateTariff(body) {
  return parseBody(body, (f) => ({
    code: f.string('code', { required: true, max: 100 }),
    name: f.string('name', { required: true, max: 255 }),
    description: f.string('description', { max: 1000 }),
    tariff_type_code: f.string('tariff_type_code', { required: true }),
    tier_strategy: f.string('tier_strategy'),
    quantity_unit: f.string('quantity_unit'),
    amount: f.money('amount', { required: true }),
    tiers: f.list('tiers', {
      item: (t) => ({
        up_to: t.integer('up_to', { min: 1 }),
        unit_price: t.money('unit_price', { required: true }),
        flat_fee: t.money('flat_fee', { required: true }),
      }),
    }),
    metadata: f.json('metadata'),
  }));
}

export function createTariffsHandler({ service }) {
  return {
    create: async (req, res) => {
      const input = parseCreateTariff(req.body);
      created(res, await service.create(input, requestContext(req)));
    },
  };
}
