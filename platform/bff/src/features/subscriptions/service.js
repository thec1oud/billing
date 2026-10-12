import { pick } from '../../shared/pick.js';

const SUBSCRIPTION_FIELDS = [
  'id',
  'version',
  'account_id',
  'plan_id',
  'plan_version',
  'status',
  'quantity',
  'current_period_start',
  'current_period_end',
  'billing_cycle_anchor',
  'cancel_at_period_end',
  'canceled_at',
  'ended_at',
  'paused_at',
  'resumes_at',
];

export const toSubscriptionDto = (subscription) => pick(subscription, SUBSCRIPTION_FIELDS);

export const toUpstreamSubscription = (input) => ({
  account_id: input.account_id,
  plan_id: input.plan_id,
  plan_version: input.plan_version,
  quantity: input.quantity,
});

export function createSubscriptionsService({ billing }) {
  return {
    async create(input, ctx) {
      const created = await billing.subscriptions.create(toUpstreamSubscription(input), ctx);
      return toSubscriptionDto(created);
    },

    async listForAccount(accountId, ctx) {
      const subscriptions = await billing.subscriptions.listForAccount(accountId, ctx);
      // Go answers `null` when the account has none.
      return (subscriptions ?? []).map(toSubscriptionDto);
    },
  };
}
