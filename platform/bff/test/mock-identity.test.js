import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createLocalJWKSet, jwtVerify } from 'jose';
import { createMockIdentityApp } from '../src/dev/mock-identity-server.js';

test('mock identity publishes a JWKS and issues a verifiable short-lived token', async (t) => {
  const issuer = 'http://mock-identity:8090';
  const audience = 'billing-api';
  const app = await createMockIdentityApp({ issuer, audience });
  const server = app.listen(0, '127.0.0.1');
  t.after(() => new Promise((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
  }));
  await new Promise((resolve, reject) => {
    server.once('listening', resolve);
    server.once('error', reject);
  });
  const baseUrl = `http://127.0.0.1:${server.address().port}`;

  const jwksResponse = await fetch(`${baseUrl}/.well-known/jwks.json`);
  assert.equal(jwksResponse.status, 200);
  const jwks = createLocalJWKSet(await jwksResponse.json());

  const tokenResponse = await fetch(`${baseUrl}/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ subject: 'demo-user', name: 'Demo User' }),
  });
  assert.equal(tokenResponse.status, 200);
  const { access_token: token } = await tokenResponse.json();
  const { payload } = await jwtVerify(token, jwks, { issuer, audience });
  assert.equal(payload.sub, 'demo-user');
  assert.equal(payload.name, 'Demo User');
  assert.equal(typeof payload.exp, 'number');
});

test('mock identity rejects empty subjects', async (t) => {
  const app = await createMockIdentityApp({
    issuer: 'http://mock-identity:8090',
    audience: 'billing-api',
  });
  const server = app.listen(0, '127.0.0.1');
  t.after(() => new Promise((resolve, reject) => {
    server.close((error) => (error ? reject(error) : resolve()));
  }));
  await new Promise((resolve, reject) => {
    server.once('listening', resolve);
    server.once('error', reject);
  });

  const response = await fetch(`http://127.0.0.1:${server.address().port}/token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ subject: '  ' }),
  });
  assert.equal(response.status, 400);
});
