import { describe, expect, it } from 'vitest';
import { readySession } from '../api/mock/fixtures';
import { gateTarget } from './gate';

describe('session gate', () => {
  it('FR-ACCT-005 AC1: no credentials → setup before any other view', () => {
    expect(gateTarget(readySession({ state: 'no_credentials' }), '/home')).toBe('/setup');
    expect(gateTarget(readySession({ state: 'locked' }), '/')).toBe('/setup');
    expect(gateTarget(readySession({ state: 'no_credentials' }), '/setup')).toBeNull();
  });
  it('logged out (or a login in progress) → login screen', () => {
    expect(gateTarget(readySession({ state: 'logged_out' }), '/holdings')).toBe('/login');
    expect(gateTarget(readySession({ state: 'logging_in' }), '/home')).toBe('/login');
    expect(gateTarget(readySession({ state: 'logging_in' }), '/login')).toBeNull();
  });
  it('FR-SHELL-020 AC1: a fresh login shows the risk disclosure before any other view', () => {
    expect(gateTarget(readySession({ riskAckRequired: true }), '/home')).toBe('/risk');
    expect(gateTarget(readySession({ riskAckRequired: true }), '/risk')).toBeNull();
  });
  it('FR-SHELL-020 AC3 / FR-ACCT-006 AC4: an acknowledged, still-valid session goes straight in', () => {
    expect(gateTarget(readySession(), '/home')).toBeNull();
    expect(gateTarget(readySession(), '/login')).toBe('/');
  });
  it('an expiry while running keeps the user where they are (banner instead)', () => {
    expect(gateTarget(readySession({ state: 'expired' }), '/holdings')).toBeNull();
    expect(gateTarget(readySession({ state: 'expired' }), '/login')).toBeNull();
  });
  it('Settings stay reachable while logged out or logging in, not before setup (merge with Task 14, Ruling 97)', () => {
    expect(gateTarget(readySession({ state: 'logged_out' }), '/settings/kite')).toBeNull();
    expect(gateTarget(readySession({ state: 'logging_in' }), '/settings')).toBeNull();
    expect(gateTarget(readySession({ state: 'no_credentials' }), '/settings/kite')).toBe('/setup');
    expect(gateTarget(readySession({ state: 'locked' }), '/settings/kite')).toBe('/setup');
  });
  it('legal pages and help are reachable before login', () => {
    expect(gateTarget(readySession({ state: 'no_credentials' }), '/legal/terms')).toBeNull();
    expect(gateTarget(readySession({ state: 'logged_out' }), '/help/kite-setup')).toBeNull();
  });
});
