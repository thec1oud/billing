import { test } from 'node:test';
import assert from 'node:assert/strict';
import express from 'express';
import http from 'node:http';
import { exportJWK, generateKeyPair, SignJWT } from 'jose';
import { createAuthMiddleware } from '../src/shared/middleware/auth.js';

test('OIDC middleware accepts valid JWTs and rejects missing, expired, or wrong-audience tokens', async () => {
  const { privateKey, publicKey } = await generateKeyPair('RS256');
  const jwk = await exportJWK(publicKey);
  Object.assign(jwk, { kid: 'auth-test', use: 'sig', alg: 'RS256' });
  const jwksServer = http.createServer((_req, res) => {
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify({ keys: [jwk] }));
  });
  const app = express();
  const port = await new Promise((resolve) => {
    jwksServer.listen(0, '127.0.0.1', () => resolve(jwksServer.address().port));
  });
  const issuer = 'https://issuer.example/';
  app.get(
    '/private',
    createAuthMiddleware({
      jwksUrl: `http://127.0.0.1:${port}/jwks`,
      issuer,
      audience: 'billing-api',
    }),
    (req, res) => res.json(req.auth)
  );
  const server = app.listen(0, '127.0.0.1');
  await new Promise((resolve) => server.once('listening', resolve));

  async function token(audience = 'billing-api', expires = '5m') {
    return new SignJWT({ name: 'Test User' })
      .setProtectedHeader({ alg: 'RS256', kid: 'auth-test' })
      .setIssuer(issuer)
      .setAudience(audience)
      .setSubject('verified-user')
      .setIssuedAt()
      .setExpirationTime(expires)
      .sign(privateKey);
  }

  try {
    const url = `http://127.0.0.1:${server.address().port}/private`;
    const missing = await fetch(url);
    assert.equal(missing.status, 401);

    const valid = await fetch(url, { headers: { Authorization: `Bearer ${await token()}` } });
    assert.equal(valid.status, 200);
    assert.deepEqual(await valid.json(), { subject: 'verified-user', name: 'Test User' });

    const wrongAudience = await fetch(url, {
      headers: { Authorization: `Bearer ${await token('another-api')}` },
    });
    assert.equal(wrongAudience.status, 401);

    const expired = await fetch(url, {
      headers: { Authorization: `Bearer ${await token('billing-api', '-1s')}` },
    });
    assert.equal(expired.status, 401);
  } finally {
    await new Promise((resolve, reject) => server.close((error) => (error ? reject(error) : resolve())));
    await new Promise((resolve, reject) => jwksServer.close((error) => (error ? reject(error) : resolve())));
  }
});
