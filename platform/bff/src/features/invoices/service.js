import { pick } from '../../shared/pick.js';

const INVOICE_FIELDS = [
  'invoice_id',
  'account_id',
  'invoice_number',
  'status',
  'currency',
  'subtotal',
  'tax',
  'discount',
  'total',
  'amount_paid',
  'amount_due',
  'due_at',
  'finalized_at',
  'paid_at',
];

const LINE_ITEM_FIELDS = [
  'line_item_id',
  'item_id',
  'description',
  'quantity_value',
  'quantity_unit',
  'unit_amount',
  'total_amount',
  'metadata',
  'subscription_id',
];

export const toInvoiceDto = (invoice) => ({
  ...pick(invoice, INVOICE_FIELDS),
  // Go serialises a missing slice as null; the UI maps over this unguarded.
  line_items: (invoice?.line_items ?? []).map((item) => pick(item, LINE_ITEM_FIELDS)),
});

// Only what the browser needs to continue checkout. The upstream response also
// carries the provider's raw payload, which stays on the server.
export const toPaymentDto = (result) =>
  pick(result, ['status', 'internal_tx_id', 'provider_reference', 'checkout_url']);

export function createInvoicesService({ billing }) {
  return {
    async pay(invoiceId, input, ctx) {
      const result = await billing.invoices.pay(
        invoiceId,
        {
          provider_code: input.provider_code,
          idempotency_key: input.idempotency_key,
        },
        ctx,
      );
      return toPaymentDto(result);
    },

    async generateDev(input, ctx) {
      const invoice = await billing.invoices.generateDev(
        { account_id: input.account_id, plan_id: input.plan_id },
        ctx,
      );
      return toInvoiceDto(invoice);
    },
  };
}
