export interface BackendError { code: string; stage: string; reason: string }

const GENERIC = 'Something went wrong';

const isRecord = (v: unknown): v is Record<string, unknown> => typeof v === 'object' && v !== null && !Array.isArray(v);
const text = (v: unknown): string => (typeof v === 'string' ? v : '');

/** The refusal form `{code, stage, reason}` (ARCHITECTURE §8.4); a missing or empty reason becomes the generic message. */
function fromRefusal(o: Record<string, unknown>): BackendError {
  return { code: text(o.code) || 'error', stage: text(o.stage), reason: text(o.reason) || GENERIC };
}

/**
 * Accepts what a rejected binding promise may carry: the JSON string, an Error whose message is that JSON or plain
 * Go error text, an already-parsed refusal object, or anything else (→ the generic message).
 */
export function parseBackendError(e: unknown): BackendError {
  if (isRecord(e) && !(e instanceof Error) && ('code' in e || 'reason' in e)) return fromRefusal(e);
  const raw = e instanceof Error ? e.message : typeof e === 'string' ? e : isRecord(e) ? text(e.message) : '';
  try {
    const j: unknown = JSON.parse(raw);
    if (isRecord(j) && ('code' in j || 'reason' in j)) return fromRefusal(j);
  } catch {
    // not JSON: plain Go error text
  }
  return { code: 'error', stage: '', reason: raw.trim() || GENERIC };
}
