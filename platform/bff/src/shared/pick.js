/** Copy only the listed keys (and only those that are present). */
export function pick(source, keys) {
  const out = {};
  if (source === null || typeof source !== 'object') return out;
  for (const key of keys) {
    if (source[key] !== undefined) out[key] = source[key];
  }
  return out;
}
