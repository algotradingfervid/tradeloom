export interface BackendError { code: string; stage: string; reason: string }

const GENERIC = 'Something went wrong';

export function parseBackendError(e: unknown): BackendError {
  const raw = e instanceof Error ? e.message : typeof e === 'string' ? e : '';
  try {
    const j: unknown = JSON.parse(raw);
    if (j && typeof j === 'object' && typeof (j as { reason?: unknown }).reason === 'string') {
      const o = j as { code?: unknown; stage?: unknown; reason: string };
      return { code: String(o.code ?? 'error'), stage: String(o.stage ?? ''), reason: o.reason };
    }
  } catch {
    // not JSON: plain Go error text
  }
  return { code: 'error', stage: '', reason: raw || GENERIC };
}
