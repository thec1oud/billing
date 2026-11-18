import { createRemoteJWKSet, jwtVerify } from 'jose';

export function createAuthMiddleware({ jwksUrl, issuer, audience }) {
  if (!jwksUrl || !issuer || !audience) {
    throw new Error('AUTH_JWKS_URL, AUTH_ISSUER, and AUTH_AUDIENCE are required');
  }

  const parsedJwksUrl = new URL(jwksUrl);
  const loopback = ['localhost', '127.0.0.1', '[::1]'].includes(parsedJwksUrl.hostname);
  if (parsedJwksUrl.protocol !== 'https:' && !(parsedJwksUrl.protocol === 'http:' && loopback)) {
    throw new Error('AUTH_JWKS_URL must use HTTPS (HTTP is allowed only for localhost)');
  }
  const jwks = createRemoteJWKSet(parsedJwksUrl);
  return async function authenticate(req, res, next) {
    const match = /^Bearer\s+(\S+)$/i.exec(req.get('authorization') || '');
    if (!match) return unauthorized(req, res);

    let payload;
    try {
      ({ payload } = await jwtVerify(match[1], jwks, {
        issuer,
        audience,
        algorithms: ['RS256'],
      }));
    } catch {
      return unauthorized(req, res);
    }
    if (typeof payload.sub !== 'string' || payload.sub === '' || typeof payload.exp !== 'number') {
      return unauthorized(req, res);
    }
    req.auth = { subject: payload.sub, name: typeof payload.name === 'string' ? payload.name : '' };
    return next();
  };
}

function unauthorized(req, res) {
  res.set('WWW-Authenticate', 'Bearer');
  return res.status(401).json({
    code: 'UNAUTHENTICATED',
    message: 'A valid bearer access token is required',
    request_id: req.requestId,
  });
}
