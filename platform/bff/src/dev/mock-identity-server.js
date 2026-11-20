import express from 'express';
import { exportJWK, generateKeyPair, SignJWT } from 'jose';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export async function createMockIdentityApp({
  issuer,
  audience,
} = {}) {
  if (!issuer || !audience) {
    throw new Error('Mock identity issuer and audience are required');
  }

  const { privateKey, publicKey } = await generateKeyPair('RS256', {
    modulusLength: 2048,
  });
  const keyId = 'local-development-key';
  const publicJwk = await exportJWK(publicKey);
  publicJwk.kid = keyId;
  publicJwk.alg = 'RS256';
  publicJwk.use = 'sig';

  const app = express();
  app.use(express.json({ limit: '4kb' }));

  app.get('/health', (_req, res) => {
    res.status(200).json({ status: 'ok', service: 'mock-identity' });
  });

  app.get('/.well-known/jwks.json', (_req, res) => {
    res.set('Cache-Control', 'public, max-age=60');
    res.json({ keys: [publicJwk] });
  });

  app.post('/token', async (req, res) => {
    const subject = typeof req.body?.subject === 'string' ? req.body.subject.trim() : '';
    const name = typeof req.body?.name === 'string' ? req.body.name.trim() : subject;
    if (!subject || subject.length > 128 || name.length > 128) {
      return res.status(400).json({
        code: 'INVALID_IDENTITY',
        message: 'A subject between 1 and 128 characters is required',
      });
    }

    const accessToken = await new SignJWT({ name })
      .setProtectedHeader({ alg: 'RS256', kid: keyId })
      .setIssuer(issuer)
      .setAudience(audience)
      .setSubject(subject)
      .setIssuedAt()
      .setExpirationTime('1h')
      .sign(privateKey);

    res.set('Cache-Control', 'no-store');
    return res.json({
      access_token: accessToken,
      token_type: 'Bearer',
      expires_in: 3600,
    });
  });

  return app;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const port = Number(process.env.PORT || 8090);
  const issuer = process.env.MOCK_IDENTITY_ISSUER || 'http://mock-identity:8090';
  const audience = process.env.AUTH_AUDIENCE || 'billing-api';
  const app = await createMockIdentityApp({ issuer, audience });
  app.listen(port, '0.0.0.0', () => {
    console.log(JSON.stringify({ msg: 'Mock identity started', port, issuer, audience }));
  });
}
