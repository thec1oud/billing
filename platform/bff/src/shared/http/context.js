export function requestContext(req) {
  return {
    requestId: req.requestId,
    authorization: req.get?.('authorization') ?? req.headers?.authorization,
  };
}