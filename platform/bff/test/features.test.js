import { test, before, after, beforeEach } from 'node:test';
import assert from 'node:assert/strict';
import { createBillingClient } from '../src/integrations/index.js';
import { createAccountsHandler } from '../src/features/accounts/handler.js';
import { createAccountsService } from '../src/features/accounts/service.js';
import { createTariffsHandler } from '../src/features/tariffs/handler.js';
import { createTariffsService } from '../src/features/tariffs/service.js';
import { createPlansHandler } from '../src/features/plans/handler.js';
import { createPlansService } from '../src/features/plans/service.js';
import { createSubscriptionsHandler } from '../src/features/subscriptions/handler.js';
import { createSubscriptionsService } from '../src/features/subscriptions/service.js';
import { createInvoicesHandler } from '../src/features/invoices/handler.js';
import { createInvoicesService } from '../src/features/invoices/service.js';
import { createWebhooksHandler } from '../src/features/webhooks/handler.js';
import { createWebhooksService } from '../src/features/webhooks/service.js';
import { startFakeBilling, goOk, goFail } from '../test-support/fakeBilling.js';
import { invoke } from '../test-support/invoke.js';

let fake;
let accounts;
let tariffs;
let plans;
let subscriptions;
let invoices;
let webhooks;

before(async () => {
  fake = await startFakeBilling();
  const billing = createBillingClient({ baseUrl: fake.url, timeoutMs: 300 });
  accounts = createAccountsHandler({ service: createAccountsService({ billing }) });
  tariffs = createTariffsHandler({ service: createTariffsService({ billing }) });
  plans = createPlansHandler({ service: createPlansService({ billing }) });
  subscriptions = createSubscriptionsHandler({ service: createSubscriptionsService({ billing }) });
  invoices = createInvoicesHandler({ service: createInvoicesService({ billing }) });
  webhooks = createWebhooksHandler({ service: createWebhooksService({ billing }) });
});
after(() => fake.close());
beforeEach(() => {
  fake.calls.length = 0;
});

const noUpstreamCalls = () => assert.equal(fake.calls.length, 0, 'billing service must not be called');

// ------------------------------------------------------------------ accounts

test('POST /accounts unpacks the body and sends only known fields upstream', async () => {
  fake.on('POST', '/api/v1/accounts', ({ body }) =>
    goOk({ id: 11, status: 'PENDING_VERIFICATION', ...body, created_at: '2026-09-19T00:00:00Z', internal_note: 'x' }, 201)
  );

  const res = await invoke(accounts.create, {
    body: { external_id: ' user_1 ', currency: 'etb', timezone: 'UTC', net_terms: 0, is_admin: true },
    requestId: 'rid-acc',
  });

  assert.equal(res.status, 201);
  assert.equal(res.body.success, true);
  assert.equal(res.body.data.id, 11);
  assert.equal(res.body.data.currency, 'ETB');
  assert.ok(!('internal_note' in res.body.data), 'unlisted upstream fields are not exposed');

  const sent = fake.calls[0];
  assert.deepEqual(sent.body, { external_id: 'user_1', currency: 'ETB', timezone: 'UTC', net_terms: 0 });
  assert.equal(sent.headers['x-request-id'], 'rid-acc');
});

test('POST /accounts with an invalid body is rejected before calling Go', async () => {
  const res = await invoke(accounts.create, { body: { currency: 'ETBX', net_terms: -1 } });

  assert.equal(res.status, 400);
  assert.equal(res.body.code, 'VALIDATION_ERROR');
  assert.equal(res.body.request_id, 'req-test-1');
  assert.deepEqual(res.body.details, [
    { field: 'currency', message: 'must be a three-letter currency code' },
    { field: 'net_terms', message: 'must be at least 0' },
  ]);
  noUpstreamCalls();
});

test('POST /accounts without a body is a validation error', async () => {
  const res = await invoke(accounts.create, { body: undefined });
  assert.equal(res.status, 400);
  assert.deepEqual(res.body.details, [{ field: 'body', message: 'must be a JSON object' }]);
  noUpstreamCalls();
});

test('POST /accounts relays a Go 409 (duplicate external id) with its code', async () => {
  fake.on('POST', '/api/v1/accounts', goFail(409, 'conflict', 'account external id already exists'));
  const res = await invoke(accounts.create, { body: { currency: 'ETB', external_id: 'dup' } });
  assert.equal(res.status, 409);
  assert.equal(res.body.code, 'conflict');
  assert.equal(res.body.message, 'account external id already exists');
});

test('POST /accounts/:id/activate validates the id and calls the right upstream path', async () => {
  const bad = await invoke(accounts.activate, { params: { accountId: 'abc' } });
  assert.equal(bad.status, 400);
  assert.deepEqual(bad.body.details, [{ field: 'accountId', message: 'must be a positive integer' }]);
  noUpstreamCalls();

  fake.on('POST', '/api/v1/accounts/7/activate', goOk({ id: 7, status: 'ACTIVE', currency: 'ETB' }));
  const res = await invoke(accounts.activate, { params: { accountId: '7' } });
  assert.equal(res.status, 200);
  assert.equal(res.body.data.status, 'ACTIVE');
});

test('activate relays Go 404 for an unknown account', async () => {
  fake.on('POST', '/api/v1/accounts/999/activate', goFail(404, 'not_found', 'account not found'));
  const res = await invoke(accounts.activate, { params: { accountId: '999' } });
  assert.equal(res.status, 404);
  assert.equal(res.body.code, 'not_found');
});

// ------------------------------------------------------------------ tariffs

test('POST /tariffs builds the Go request (money, tiers, base64 metadata, default description)', async () => {
  fake.on('POST', '/api/v1/tariffs', goOk({ tariff_id: 3, tariff_code: 'starter_tariff', version: 1, is_active: true }, 201));

  const res = await invoke(tariffs.create, {
    body: {
      code: 'starter_tariff',
      name: 'Starter pricing',
      tariff_type_code: 'FLAT_FEE',
      amount: { amount_minor: 1000, currency: 'etb' },
      metadata: { tier: 'gold' },
      tiers: [{ up_to: 10, unit_price: { amount_minor: 5, currency: 'ETB' }, flat_fee: { amount_minor: 0, currency: 'ETB' } }],
    },
  });

  assert.equal(res.status, 201);
  assert.equal(res.body.data.tariff_id, 3);

  const sent = fake.calls[0].body;
  assert.equal(sent.description, '');
  assert.deepEqual(sent.amount, { amount_minor: 1000, currency: 'ETB' });
  assert.equal(sent.tiers[0].up_to, 10);
  // Go decodes this field into []byte, so it must arrive base64-encoded.
  assert.deepEqual(JSON.parse(Buffer.from(sent.metadata, 'base64').toString()), { tier: 'gold' });
});

test('POST /tariffs omits metadata entirely when none is given', async () => {
  fake.on('POST', '/api/v1/tariffs', goOk({ tariff_id: 4 }, 201));
  await invoke(tariffs.create, {
    body: { code: 'c', name: 'n', tariff_type_code: 'FLAT_FEE', amount: { amount_minor: 1, currency: 'ETB' } },
  });
  assert.ok(!('metadata' in fake.calls[0].body));
});

test('POST /tariffs reports nested money/tier problems with paths', async () => {
  const res = await invoke(tariffs.create, {
    body: {
      code: 'c',
      name: 'n',
      tariff_type_code: 'TIERED_USAGE',
      amount: { amount_minor: 1.5, currency: 'ETB' },
      tiers: [{ unit_price: { amount_minor: 1, currency: 'ETB' } }],
    },
  });
  assert.equal(res.status, 400);
  assert.deepEqual(res.body.details, [
    { field: 'amount.amount_minor', message: 'must be an integer' },
    { field: 'tiers[0].flat_fee', message: 'is required' },
  ]);
  noUpstreamCalls();
});

test('POST /tariffs relays a domain rule violation from Go as a 400', async () => {
  fake.on('POST', '/api/v1/tariffs', goFail(400, 'invalid_request', 'validate tariff: invalid tariff type "NOPE"'));
  const res = await invoke(tariffs.create, {
    body: { code: 'c', name: 'n', tariff_type_code: 'NOPE', amount: { amount_minor: 1, currency: 'ETB' } },
  });
  assert.equal(res.status, 400);
  assert.equal(res.body.code, 'invalid_request');
});

// ------------------------------------------------------------------ plans

test('GET /plans turns a null upstream list into [] and shapes durations', async () => {
  fake.on('GET', '/api/v1/plans', { status: 200, body: { success: true, data: null } });
  const empty = await invoke(plans.list);
  assert.deepEqual(empty.body, { success: true, data: [] });

  fake.on(
    'GET',
    '/api/v1/plans',
    goOk([
      { plan_id: 1, plan_code: 'a', version: 1, legacy_price_policy_code: 'KEEP_FOREVER', durations: null, secret: 1 },
      { plan_id: 2, plan_code: 'b', version: 2, durations: [{ plan_duration_id: 5, plan_id: 2, tariff_id: 3, duration: 1, is_active: true, extra: 'x' }] },
    ])
  );
  const res = await invoke(plans.list);
  assert.equal(res.status, 200);
  assert.deepEqual(res.body.data[0].durations, []);
  assert.ok(!('secret' in res.body.data[0]));
  assert.deepEqual(res.body.data[1].durations, [{ plan_duration_id: 5, plan_id: 2, tariff_id: 3, duration: 1, is_active: true }]);
});

test('POST /plans sends durations in Go shape, including a year-long nanosecond duration', async () => {
  const YEAR_NS = 365 * 24 * 60 * 60 * 1_000_000_000;
  fake.on('POST', '/api/v1/plans', goOk({ plan_id: 8, plan_code: 'pro', version: 1 }, 201));

  const res = await invoke(plans.create, {
    body: {
      plan_code: 'pro',
      legacy_price_policy_code: 'KEEP_FOREVER',
      durations: [{ tariff_id: 3, duration: YEAR_NS, is_active: false }],
    },
  });
  assert.equal(res.status, 201);
  assert.equal(res.body.data.plan_id, 8);
  assert.deepEqual(fake.calls[0].body, {
    plan_code: 'pro',
    legacy_price_policy_code: 'KEEP_FOREVER',
    durations: [{ tariff_id: 3, duration: YEAR_NS }],
  });
  assert.equal(fake.calls[0].raw.includes('31536000000000000'), true, 'duration serialises as a plain integer');
});

test('POST /plans requires at least one duration', async () => {
  const res = await invoke(plans.create, { body: { plan_code: 'x', legacy_price_policy_code: 'KEEP_FOREVER', durations: [] } });
  assert.equal(res.status, 400);
  assert.deepEqual(res.body.details, [{ field: 'durations', message: 'must contain at least 1 item(s)' }]);
  noUpstreamCalls();
});

// ------------------------------------------------------------------ subscriptions

test('POST /subscriptions forwards only account/plan/version/quantity', async () => {
  fake.on('POST', '/api/v1/subscriptions', ({ body }) =>
    goOk({ id: 21, version: 1, status: 'ACTIVE', ...body, current_period_start: '2026-09-19T00:00:00Z' }, 201)
  );
  const res = await invoke(subscriptions.create, {
    body: {
      account_id: 7,
      plan_id: 2,
      plan_version: 1,
      current_period_start: '2020-01-01T00:00:00Z',
      current_period_end: '2020-02-01T00:00:00Z',
      billing_cycle_anchor: '2020-01-01T00:00:00Z',
    },
  });
  assert.equal(res.status, 201);
  assert.equal(res.body.data.id, 21);
  assert.deepEqual(fake.calls[0].body, { account_id: 7, plan_id: 2, plan_version: 1 });
});

test('POST /subscriptions relays Go domain errors (inactive account, version mismatch)', async () => {
  fake.on('POST', '/api/v1/subscriptions', goFail(400, 'invalid_request', 'account 7 is not active'));
  const res = await invoke(subscriptions.create, { body: { account_id: 7, plan_id: 2, plan_version: 1 } });
  assert.equal(res.status, 400);
  assert.equal(res.body.message, 'account 7 is not active');
});

test('POST /subscriptions validates required ids', async () => {
  const res = await invoke(subscriptions.create, { body: { account_id: '7', plan_id: 0 } });
  assert.equal(res.status, 400);
  assert.deepEqual(
    res.body.details.map((d) => d.field),
    ['account_id', 'plan_id', 'plan_version']
  );
  noUpstreamCalls();
});

test('GET /accounts/:id/subscriptions returns [] when Go answers null', async () => {
  fake.on('GET', '/api/v1/accounts/7/subscriptions', { status: 200, body: { success: true, data: null } });
  const res = await invoke(subscriptions.listForAccount, { params: { accountId: '7' } });
  assert.deepEqual(res.body, { success: true, data: [] });
});

// ------------------------------------------------------------------ invoices

test('POST /invoices/:id/pay sends provider + idempotency key and hides the raw provider payload', async () => {
  fake.on('POST', '/api/v1/invoices/12/pay', goOk({
    status: 'PENDING',
    internal_tx_id: 'req_1',
    provider_reference: 'fake_ref_abc',
    checkout_url: 'https://checkout.fake-provider.com/pay/fake_ref_abc',
    raw_response: { tx_ref: 'req_1', reference: 'fake_ref_abc' },
  }));

  const res = await invoke(invoices.pay, {
    params: { invoiceId: '12' },
    body: { provider_code: 'fake', idempotency_key: 'req_1', ignored: true },
  });

  assert.equal(res.status, 200);
  assert.deepEqual(res.body.data, {
    status: 'PENDING',
    internal_tx_id: 'req_1',
    provider_reference: 'fake_ref_abc',
    checkout_url: 'https://checkout.fake-provider.com/pay/fake_ref_abc',
  });
  assert.deepEqual(fake.calls[0].body, { provider_code: 'fake', idempotency_key: 'req_1' });
});

test('pay requires an idempotency key and a valid invoice id', async () => {
  const noKey = await invoke(invoices.pay, { params: { invoiceId: '12' }, body: { provider_code: 'fake' } });
  assert.equal(noKey.status, 400);
  assert.deepEqual(noKey.body.details, [{ field: 'idempotency_key', message: 'is required' }]);

  const badId = await invoke(invoices.pay, { params: { invoiceId: '0' }, body: { provider_code: 'fake', idempotency_key: 'k' } });
  assert.equal(badId.status, 400);
  assert.equal(badId.body.details[0].field, 'invoiceId');
  noUpstreamCalls();
});

test('pay relays Go business errors (invoice not OPEN, payment failed)', async () => {
  fake.on('POST', '/api/v1/invoices/12/pay', goFail(400, 'INVALID_STATUS', 'Invoice must be in OPEN status to be paid'));
  const res = await invoke(invoices.pay, { params: { invoiceId: '12' }, body: { provider_code: 'fake', idempotency_key: 'k' } });
  assert.equal(res.status, 400);
  assert.equal(res.body.code, 'INVALID_STATUS');
});

test('POST /dev/invoices/generate returns the invoice with line_items always an array', async () => {
  fake.on('POST', '/api/v1/dev/invoices/generate', goOk({
    invoice_id: 30,
    account_id: 7,
    status: 'OPEN',
    currency: 'ETB',
    subtotal: { amount_minor: 1000, currency: 'ETB' },
    amount_due: { amount_minor: 1000, currency: 'ETB' },
    line_items: null,
  }, 201));

  const res = await invoke(invoices.generateDev, { body: { account_id: 7, plan_id: 2 } });
  assert.equal(res.status, 201);
  assert.equal(res.body.data.invoice_id, 30);
  assert.deepEqual(res.body.data.line_items, []);
  assert.deepEqual(fake.calls[0].body, { account_id: 7, plan_id: 2 });
});

// ------------------------------------------------------------------ webhooks

const webhookBody = {
  event_id: 'wh_evt_1',
  event: 'charge.success',
  tx_ref: 'req_1',
  reference: 'fake_ref_abc',
  status: 'success',
  amount_minor: 1000,
  currency: 'ETB',
};

test('POST /webhooks/fake unpacks the callback and forwards it to the Go fake provider', async () => {
  fake.on('POST', '/api/v1/webhooks/fake', { status: 200, body: { status: 'processed', webhook_id: 'wh_evt_1' } });
  const res = await invoke(webhooks.simulateFake, { body: { ...webhookBody, junk: 1 } });

  assert.equal(res.status, 200);
  assert.deepEqual(res.body, { success: true, data: { status: 'processed', webhook_id: 'wh_evt_1' } });
  assert.deepEqual(fake.calls[0].body, webhookBody);
});

test('duplicate webhook delivery surfaces as ignored, not as an error', async () => {
  fake.on('POST', '/api/v1/webhooks/fake', { status: 200, body: { status: 'ignored', reason: 'duplicate_event' } });
  const res = await invoke(webhooks.simulateFake, { body: webhookBody });
  assert.equal(res.status, 200);
  assert.deepEqual(res.body.data, { status: 'ignored', reason: 'duplicate_event' });
});

test('webhook validation rejects incomplete callbacks', async () => {
  const res = await invoke(webhooks.simulateFake, { body: { event_id: 'x', amount_minor: -5 } });
  assert.equal(res.status, 400);
  assert.deepEqual(
    res.body.details.map((d) => d.field),
    ['event', 'tx_ref', 'reference', 'status', 'amount_minor', 'currency']
  );
  noUpstreamCalls();
});

test('a Go 500 in the bare webhook format becomes a 502 UPSTREAM_ERROR', async () => {
  fake.on('POST', '/api/v1/webhooks/fake', { status: 500, body: { status: 'error' } });
  const res = await invoke(webhooks.simulateFake, { body: webhookBody });
  assert.equal(res.status, 502);
  assert.equal(res.body.code, 'UPSTREAM_ERROR');
  assert.equal(res.body.request_id, 'req-test-1');
});

// ------------------------------------------------------------------ upstream failures

test('Go 5xx is reported as 502 (not passed through as a 500)', async () => {
  fake.on('GET', '/api/v1/plans', goFail(500, 'internal_error', 'pq: relation "plans" does not exist'));
  const res = await invoke(plans.list);
  assert.equal(res.status, 502);
  assert.equal(res.body.code, 'UPSTREAM_ERROR');
});

test('unreachable billing service is 502 BAD_GATEWAY and a stalled one is 504', async () => {
  const dead = createBillingClient({ baseUrl: 'http://127.0.0.1:1', timeoutMs: 300 });
  const deadPlans = createPlansHandler({ service: createPlansService({ billing: dead }) });
  const down = await invoke(deadPlans.list);
  assert.equal(down.status, 502);
  assert.equal(down.body.code, 'BAD_GATEWAY');

  fake.on('GET', '/api/v1/plans', { hang: true });
  const slow = await invoke(plans.list);
  assert.equal(slow.status, 504);
  assert.equal(slow.body.code, 'GATEWAY_TIMEOUT');
});
