import { describe, expect, it } from 'vitest';
import { parseBackendError } from './errors';

describe('parseBackendError (ARCHITECTURE §8.4 error JSON)', () => {
  it('reads code, stage and reason from the JSON form', () => {
    expect(parseBackendError('{"code":"conflict","stage":"shortcuts","reason":"Go to Home"}'))
      .toEqual({ code: 'conflict', stage: 'shortcuts', reason: 'Go to Home' });
  });
  it('accepts Error instances and plain strings', () => {
    expect(parseBackendError(new Error('network down')).reason).toBe('network down');
    expect(parseBackendError('timeout')).toEqual({ code: 'error', stage: '', reason: 'timeout' });
  });
  it('falls back to a generic message for empty input', () => {
    expect(parseBackendError(undefined).reason).toBe('Something went wrong');
  });
});
