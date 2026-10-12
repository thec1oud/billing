import { pick } from '../../shared/pick.js';

export const toUpstreamFakeWebhook = (input) => ({
  event_id: input.event_id,
  event: input.event,
  tx_ref: input.tx_ref,
  reference: input.reference,
  status: input.status,
  amount_minor: input.amount_minor,
  currency: input.currency,
  failure_reason: input.failure_reason,
  timestamp: input.timestamp,
});

export function createWebhooksService({ billing }) {
  return {
    /**
     * Stands in for the payment provider calling us back. In a real setup the
     * provider posts to the billing service directly; here the demo UI triggers it.
     */
    async simulateFake(input, ctx) {
      // Not wrapped in the Go { success, data } envelope: { status, webhook_id | reason }.
      const result = await billing.webhooks.deliver('fake', toUpstreamFakeWebhook(input), ctx);
      return pick(result, ['status', 'webhook_id', 'reason']);
    },
  };
}
