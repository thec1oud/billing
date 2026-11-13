import { pick } from '../../shared/pick.js';

const TARIFF_FIELDS = [
  'tariff_id',
  'tariff_code',
  'version',
  'name',
  'description',
  'tariff_type_code',
  'tier_strategy',
  'quantity_unit',
  'amount',
  'tiers',
  'metadata',
  'created_at',
  'is_active',
];

export const toTariffDto = (tariff) => pick(tariff, TARIFF_FIELDS);

export const toUpstreamTariff = (input) => ({
  code: input.code,
  name: input.name,
  description: input.description ?? '',
  tariff_type_code: input.tariff_type_code,
  tier_strategy: input.tier_strategy,
  quantity_unit: input.quantity_unit,
  amount: input.amount,
  tiers: input.tiers,
  // The Go request struct declares metadata as []byte, which encoding/json
  // only accepts as a base64 string, not as a JSON object.
  metadata:
    input.metadata === undefined
      ? undefined
      : Buffer.from(JSON.stringify(input.metadata)).toString('base64'),
});

export function createTariffsService({ billing }) {
  return {
    async create(input, ctx) {
      return toTariffDto(await billing.tariffs.create(toUpstreamTariff(input), ctx));
    },
  };
}
