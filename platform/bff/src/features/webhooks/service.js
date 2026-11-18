import { pick } from '../../shared/pick.js';
import { createHmac } from 'node:crypto';

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

export function createWebhooksService({ billing, fakeWebhookHmacSecret }) {
  return {
    /**
     * Stands in for the payment provider calling us back. In a real setup the
     * provider posts to the billing service directly; here the demo UI triggers it.
     */
    async simulateFake(input, ctx) {
      if (!fakeWebhookHmacSecret || Buffer.byteLength(fakeWebhookHmacSecret) < 32) {
        throw new Error('FAKE_WEBHOOK_HMAC_SECRET must contain at least 32 bytes');
      }
      const body = toUpstreamFakeWebhook(input);
      const signature = `sha256=${createHmac('sha256', fakeWebhookHmacSecret)
        .update(JSON.stringify(body))
        .digest('hex')}`;
      // Not wrapped in the Go { success, data } envelope: { status, webhook_id | reason }.
      const result = await billing.webhooks.deliver(
        'fake',
        body,
        ctx,
        { 'X-Fake-Signature': signature }
      );
      return pick(result, ['status', 'webhook_id', 'reason']);
    },
  };
}
