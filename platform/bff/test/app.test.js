import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createApp } from '../src/app.js';

test('development routes are unavailable by default', async () => {
  const server = createApp().listen(0);
  try {
    await new Promise((resolve, reject) => {
      server.once('listening', resolve);
      server.once('error', reject);
    });

    const { port } = server.address();
    for (const path of ['/api/v1/dev/invoices/generate', '/api/v1/webhooks/fake']) {
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
  const result = spawnSync(
    process.execPath,
    ['--input-type=module', '-e', "import('./src/config/index.js')"],
    {
      cwd: new URL('..', import.meta.url),
      env: { ...process.env, NODE_ENV: 'production', ENABLE_DEV_ENDPOINTS: 'true' },
      encoding: 'utf8',
    }
  );
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /ENABLE_DEV_ENDPOINTS cannot be enabled/);
});
