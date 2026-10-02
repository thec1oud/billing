import http from 'node:http';

/** Response builders mirroring the Go service's `response.Write`. */
export const goOk = (data, status = 200) => ({ status, body: { success: true, data } });
export const goFail = (status, code, message) => ({
  status,
  body: { success: false, error: { code, message } },
});

/**
 * A stand-in for the Go billing service. Records every request and answers
 * from handlers registered per "METHOD /path".
 */
export async function startFakeBilling() {
  const calls = [];
  const routes = new Map();

  const server = http.createServer(async (req, res) => {
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const raw = Buffer.concat(chunks).toString();
    const call = {
      method: req.method,
      path: req.url,
      headers: req.headers,
      raw,
      body: raw ? JSON.parse(raw) : undefined,
    };
    calls.push(call);

    const handler = routes.get(`${req.method} ${req.url}`);
    if (!handler) {
      const miss = goFail(404, 'NOT_FOUND', `no fake route for ${req.method} ${req.url}`);
      res.writeHead(miss.status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(miss.body));
      return;
    }

    const out = await handler(call);
    if (out.hang) return; // never answer (timeout tests)
    if (out.destroy) return req.socket.destroy();
    res.writeHead(out.status, { 'Content-Type': 'application/json' });
    res.end(out.text !== undefined ? out.text : JSON.stringify(out.body));
  });

  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address();

  return {
    url: `http://127.0.0.1:${port}`,
    calls,
    on(method, path, handler) {
      routes.set(`${method} ${path}`, typeof handler === 'function' ? handler : () => handler);
    },
    async close() {
      server.closeAllConnections?.();
      await new Promise((resolve) => server.close(resolve));
    },
  };
}
