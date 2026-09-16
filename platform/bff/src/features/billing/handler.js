import { createUpstreamProxy } from './service.js';

/**
 * Returns middleware that proxies the request to the billing service.
 * Kept as a handler so future per-route logic (auth, validation)
 * can wrap or replace this without changing routes.js structure.
 */
export function handleProxy() {
  return createUpstreamProxy();
}