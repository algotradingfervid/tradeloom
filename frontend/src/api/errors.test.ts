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

describe('parseBackendError robustness (Ruling 83-F M8)', () => {
  it('accepts an already-parsed refusal object', () => {
    expect(parseBackendError({ code: 'invalid_port', stage: 'session', reason: 'Port must be 1024–65535' }))
      .toEqual({ code: 'invalid_port', stage: 'session', reason: 'Port must be 1024–65535' });
  });
  it('uses the generic message when the JSON form has no usable reason', () => {
    expect(parseBackendError('{"code":"internal","stage":"data"}')).toEqual({ code: 'internal', stage: 'data', reason: 'Something went wrong' });
    expect(parseBackendError({ code: 'internal', reason: 42 })).toEqual({ code: 'internal', stage: '', reason: 'Something went wrong' });
    expect(parseBackendError(new Error('{"code":"bad_archive","reason":""}'))).toEqual({ code: 'bad_archive', stage: '', reason: 'Something went wrong' });
  });
  it('falls back to the generic message for null, numbers and objects without a code', () => {
    expect(parseBackendError(null)).toEqual({ code: 'error', stage: '', reason: 'Something went wrong' });
    expect(parseBackendError(42)).toEqual({ code: 'error', stage: '', reason: 'Something went wrong' });
    expect(parseBackendError({ message: 'socket closed' })).toEqual({ code: 'error', stage: '', reason: 'socket closed' });
  });
});
