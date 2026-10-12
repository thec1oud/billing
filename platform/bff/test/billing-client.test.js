import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { createBillingClient, BillingApiError } from '../src/integrations/index.js';
import { startFakeBilling, goOk, goFail } from '../test-support/fakeBilling.js';

let fake;
let billing;

before(async () => {
  fake = await startFakeBilling();
  billing = createBillingClient({ baseUrl: `${fake.url}/`, timeoutMs: 300 });
});
after(() => fake.close());

const failure = async (promise) => {
  try {
    await promise;
  } catch (err) {
    assert.ok(err instanceof BillingApiError, `expected BillingApiError, got ${err}`);
    return err;
  }
  assert.fail('expected the call to fail');
};

test('unwraps the { success, data } envelope and sends JSON + request id', async () => {
  fake.on('POST', '/api/v1/accounts', goOk({ id: 9 }, 201));
  const data = await billing.accounts.create({ currency: 'ETB' }, { requestId: 'rid-1' });

  assert.deepEqual(data, { id: 9 });
  const call = fake.calls.at(-1);
  assert.equal(call.path, '/api/v1/accounts'); // trailing slash on baseUrl is tolerated
  assert.equal(call.headers['x-request-id'], 'rid-1');
  assert.equal(call.headers['content-type'], 'application/json');
  assert.deepEqual(call.body, { currency: 'ETB' });
});

test('bodyless POST sends no content-type or body', async () => {
  fake.on('POST', '/api/v1/accounts/5/activate', goOk({ id: 5, status: 'ACTIVE' }));
  await billing.accounts.activate(5);
  const call = fake.calls.at(-1);
  assert.equal(call.raw, '');
  assert.equal(call.headers['content-type'], undefined);
});

test('returns bare (non-enveloped) JSON as-is, as the webhook endpoint does', async () => {
  fake.on('POST', '/api/v1/webhooks/fake', { status: 200, body: { status: 'processed', webhook_id: 'w1' } });
  assert.deepEqual(await billing.webhooks.deliver('fake', {}), { status: 'processed', webhook_id: 'w1' });
});

test('enveloped response without data resolves to null', async () => {
  fake.on('GET', '/api/v1/plans', { status: 200, body: { success: true } });
  assert.equal(await billing.plans.list(), null);
});

test('upstream 4xx keeps status, code and message', async () => {
  fake.on('POST', '/api/v1/subscriptions', goFail(409, 'conflict', 'account is closed'));
  const err = await failure(billing.subscriptions.create({}));
  assert.deepEqual([err.kind, err.status, err.code, err.message], ['http', 409, 'conflict', 'account is closed']);
});

test('upstream error without the envelope still yields a usable error', async () => {
  fake.on('POST', '/api/v1/webhooks/fake', { status: 500, body: { status: 'error' } });
  const err = await failure(billing.webhooks.deliver('fake', {}));
  assert.deepEqual([err.kind, err.status, err.code], ['http', 500, null]);
  assert.match(err.message, /HTTP 500/);
});

test('non-JSON error page is handled', async () => {
  fake.on('GET', '/api/v1/plans', { status: 502, text: '<html>bad gateway</html>' });
  const err = await failure(billing.plans.list());
  assert.deepEqual([err.kind, err.status], ['http', 502]);
});

test('2xx with a non-JSON body is an invalid_response', async () => {
  fake.on('GET', '/api/v1/plans', { status: 200, text: 'not json' });
  assert.equal((await failure(billing.plans.list())).kind, 'invalid_response');
});

test('no answer within the timeout is kind=timeout', async () => {
  fake.on('GET', '/api/v1/plans', { hang: true });
  assert.equal((await failure(billing.plans.list())).kind, 'timeout');
});

test('connection failure is kind=network', async () => {
  const dead = createBillingClient({ baseUrl: 'http://127.0.0.1:1', timeoutMs: 300 });
  assert.equal((await failure(dead.plans.list())).kind, 'network');
});

test('connection dropped mid-request is kind=network', async () => {
  fake.on('GET', '/api/v1/plans', { destroy: true });
  assert.equal((await failure(billing.plans.list())).kind, 'network');
});
