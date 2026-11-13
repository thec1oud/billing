import { pick } from '../../shared/pick.js';

const PLAN_FIELDS = [
  'plan_id',
  'plan_code',
  'version',
  'effective_from',
  'effective_until',
  'legacy_price_policy_code',
  'migration_path',
  'metadata',
  'created_at',
];

const DURATION_FIELDS = [
  'plan_duration_id',
  'plan_id',
  'tariff_id',
  'duration',
  'is_active',
  'created_at',
];

export const toPlanDto = (plan) => ({
  ...pick(plan, PLAN_FIELDS),
  // Go marshals an empty slice as null / omits it; the UI expects an array.
  durations: (plan?.durations ?? []).map((d) => pick(d, DURATION_FIELDS)),
});

export const toUpstreamPlan = (input) => ({
  plan_code: input.plan_code,
  legacy_price_policy_code: input.legacy_price_policy_code,
  effective_from: input.effective_from,
  effective_until: input.effective_until,
  migration_path: input.migration_path,
  metadata: input.metadata,
  durations: input.durations.map((d) => ({
    tariff_id: d.tariff_id,
    duration: d.duration,
  })),
});

export function createPlansService({ billing }) {
  return {
    async list(ctx) {
      const plans = await billing.plans.list(ctx);
      return (plans ?? []).map(toPlanDto);
    },

    async create(input, ctx) {
      return toPlanDto(await billing.plans.create(toUpstreamPlan(input), ctx));
    },
  };
}
