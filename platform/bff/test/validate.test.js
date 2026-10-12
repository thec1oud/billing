import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseBody, parseId } from '../src/shared/http/validate.js';
import { ValidationError } from '../src/shared/http/errors.js';

const issuesOf = (fn) => {
  try {
    fn();
  } catch (err) {
    assert.ok(err instanceof ValidationError, `expected ValidationError, got ${err}`);
    return err.details;
  }
  assert.fail('expected a ValidationError');
};

test('parseId accepts positive integers only', () => {
  assert.equal(parseId('42', 'accountId'), 42);
  for (const bad of ['0', '-1', '1.5', 'abc', '12abc', '', ' 7', undefined, '9007199254740993']) {
    assert.deepEqual(
      issuesOf(() => parseId(bad, 'accountId')),
      [{ field: 'accountId', message: 'must be a positive integer' }],
      `input ${JSON.stringify(bad)}`
    );
  }
});

test('parseBody rejects a missing or non-object body', () => {
  for (const bad of [undefined, null, 'x', 5, []]) {
    assert.deepEqual(
      issuesOf(() => parseBody(bad, () => ({}))),
      [{ field: 'body', message: 'must be a JSON object' }]
    );
  }
});

test('parseBody reports every problem at once, with field paths', () => {
  const issues = issuesOf(() =>
    parseBody(
      { name: '', count: 1.5, price: { amount_minor: -1, currency: 'ET' }, items: [{}, 'x'] },
      (f) => ({
        name: f.string('name', { required: true }),
        count: f.integer('count', { required: true }),
        price: f.money('price', { required: true }),
        items: f.list('items', {
          item: (i) => ({ id: i.integer('id', { required: true }) }),
        }),
      })
    )
  );
  assert.deepEqual(issues, [
    { field: 'name', message: 'is required' },
    { field: 'count', message: 'must be an integer' },
    { field: 'price.amount_minor', message: 'must be at least 0' },
    { field: 'price.currency', message: 'must be a three-letter currency code' },
    { field: 'items[0].id', message: 'is required' },
    { field: 'items[1]', message: 'must be an object' },
  ]);
});

test('parseBody only returns fields the parser reads, and normalises values', () => {
  const out = parseBody(
    {
      currency: ' etb ',
      external_id: '  user_1 ',
      injected: 'drop me',
      note: null,
      when: '2026-03-01T10:00:00+03:00',
    },
    (f) => ({
      currency: f.currency('currency', { required: true }),
      external_id: f.string('external_id'),
      note: f.string('note'),
      when: f.timestamp('when'),
    })
  );
  assert.deepEqual(out, {
    currency: 'ETB',
    external_id: 'user_1',
    note: undefined,
    when: '2026-03-01T07:00:00.000Z',
  });
  assert.ok(!('injected' in out));
});

test('timestamp requires an ISO date-time with an offset', () => {
  for (const bad of ['2026-03-01', 'yesterday', '2026-13-45T00:00:00Z', 5]) {
    assert.equal(
      issuesOf(() => parseBody({ t: bad }, (f) => f.timestamp('t', { required: true })))[0].field,
      't'
    );
  }
});
