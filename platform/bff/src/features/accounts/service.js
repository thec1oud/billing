import { pick } from '../../shared/pick.js';

const ACCOUNT_FIELDS = [
  'id',
  'external_id',
  'status',
  'currency',
  'timezone',
  'locale',
  'net_terms',
  'dunning_profile_id',
  'tax_identifiers',
  'billing_address',
  'compliance_flags',
  'metadata',
  'payment_methods',
  'created_at',
];

export const toAccountDto = (account) => pick(account, ACCOUNT_FIELDS);

// The Go account endpoint rejects unknown JSON fields, so the request is
// rebuilt field by field instead of forwarding whatever the browser sent.
export const toUpstreamAccount = (input) => ({
  external_id: input.external_id,
  currency: input.currency,
  timezone: input.timezone,
  locale: input.locale,
  net_terms: input.net_terms,
  dunning_profile_id: input.dunning_profile_id,
  tax_identifiers: input.tax_identifiers,
  billing_address: input.billing_address,
  compliance_flags: input.compliance_flags,
  metadata: input.metadata,
});

export function createAccountsService({ billing }) {
  return {
    async create(input, ctx) {
      return toAccountDto(await billing.accounts.create(toUpstreamAccount(input), ctx));
    },

    async activate(accountId, ctx) {
      return toAccountDto(await billing.accounts.activate(accountId, ctx));
    },
  };
}
