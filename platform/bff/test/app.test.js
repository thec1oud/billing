import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';

process.env.AUTH_JWKS_URL = 'http://127.0.0.1:1/jwks';
process.env.AUTH_ISSUER = 'https://issuer.example';
process.env.AUTH_AUDIENCE = 'billing-api';
const { createApp } = await import('../src/app.js');

test('development routes are unavailable by default', async () => {
  const server = createApp().listen(0);
  try {
    await new Promise((resolve, reject) => {
      server.once('listening', resolve);
      server.once('error', reject);
    });

    const { port } = server.address();
    const businessRoute = await fetch(`http://127.0.0.1:${port}/api/v1/plans`);
    assert.equal(businessRoute.status, 401, 'business routes require authentication');
    for (const path of ['/api/v1/dev/invoices/generate', '/api/v1/dev/unknown', '/api/v1/webhooks/fake']) {
      const response = await fetch(`http://127.0.0.1:${port}${path}`, { method: 'POST' });
      assert.equal(response.status, 404, `${path} should be unavailable`);
    }
  } finally {
    await new Promise((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
    });
  }
});

test('production refuses ENABLE_DEV_ENDPOINTS=true', () => {
  for (const productionEnv of [
    { NODE_ENV: 'production', APP_ENV: 'development' },
    { NODE_ENV: 'development', APP_ENV: 'production' },
  ]) {
    const result = spawnSync(
      process.execPath,
      ['--input-type=module', '-e', "import('./src/config/index.js')"],
      {
        cwd: new URL('..', import.meta.url),
        env: { ...process.env, ...productionEnv, ENABLE_DEV_ENDPOINTS: 'true' },
        encoding: 'utf8',
      }
    );
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /ENABLE_DEV_ENDPOINTS cannot be enabled/);
  }
});
