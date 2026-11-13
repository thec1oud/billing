// Success envelope: { success: true, data }. The web client unwraps `data`.

export function ok(res, data) {
  return res.status(200).json({ success: true, data });
}

export function created(res, data) {
  return res.status(201).json({ success: true, data });
}
