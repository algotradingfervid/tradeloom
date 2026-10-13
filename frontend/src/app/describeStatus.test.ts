import { describe, expect, it } from 'vitest';
import type { ClockOffset, FeedState, MarketStatus } from '../api/backend';
import { FIXED_NOW, readySession } from '../api/mock/fixtures';
import { describeStatus } from './describeStatus';

const open: MarketStatus = { exchange: 'NSE', phase: 'continuous', dayType: 'trading', label: 'Open', tooltip: '', atMs: FIXED_NOW };
// service.FeedStateDTO / service.OffsetDTO carry Unix-ms fields (Ruling 80), not RFC3339 strings.
const base = {
  feed: { state: 'live', attempt: 0, nextRetryMs: 0, lastTickMs: FIXED_NOW - 1000, connections: 1, tokens: 3 } as FeedState,
  session: readySession(),
  market: open,
  clock: { millis: 120, source: 'ticks', samples: 40, warn: false, skewed: false, updatedAtMs: FIXED_NOW } as ClockOffset,
  now: FIXED_NOW,
};

describe('FR-SHELL-005 status model', () => {
  it('AC1: connected feed → green "Live"', () => {
    expect(describeStatus(base)).toMatchObject({ label: 'Live', tone: 'up', marketClosed: false, canLogIn: false, canReconnect: true });
  });
  it('AC2: reconnecting → amber, with attempt count and next retry in the details', () => {
    const v = describeStatus({ ...base, feed: { ...base.feed, state: 'reconnecting', attempt: 3, nextRetryMs: Date.UTC(2026, 9, 12, 5, 0, 30) } });
    expect(v).toMatchObject({ label: 'Reconnecting', tone: 'warn' });
    expect(v.lines[0]).toBe('Market data: reconnecting (attempt 3, next retry at 10:30:30)');
  });
  it('carry-forward: during the reconnecting backoff Reconnect now cannot shorten the wait, so it is unavailable', () => {
    const v = describeStatus({ ...base, feed: { ...base.feed, state: 'reconnecting', attempt: 1, nextRetryMs: FIXED_NOW + 5000 } });
    expect(v.canReconnect).toBe(false);
  });
  it('connecting → amber "Connecting"; stopped → grey "Not connected"', () => {
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'connecting' } })).toMatchObject({ label: 'Connecting', tone: 'warn' });
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'stopped' } })).toMatchObject({ label: 'Not connected', tone: 'flat' });
  });
  it('offline → red "Offline"', () => {
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'offline' } })).toMatchObject({ label: 'Offline', tone: 'down', canReconnect: true });
  });
  it('AC3: an expired session shows "Logged out" and offers Log in, whatever feed.state says', () => {
    const v = describeStatus({ ...base, session: readySession({ state: 'expired' }) });
    expect(v).toMatchObject({ state: 'logged_out', label: 'Logged out', tone: 'flat', canLogIn: true, canReconnect: false });
    expect(v.lines[1]).toBe('Kite session expired — log in to continue');
  });
  it('FR-ACCT-007 AC1: a Personal-plan key (dataPlan "none") shows grey "No data plan"', () => {
    expect(describeStatus({ ...base, session: readySession({ dataPlan: 'none' }) })).toMatchObject({ label: 'No data plan', tone: 'flat', canReconnect: false });
  });
  it('AC4: a closed phase (holiday, weekend, after hours) shows "Market closed" whatever the feed state', () => {
    const holiday: MarketStatus = { ...open, phase: 'closed', dayType: 'holiday', label: 'Holiday' };
    expect(describeStatus({ ...base, market: holiday })).toMatchObject({ label: 'Live', marketClosed: true });
    expect(describeStatus({ ...base, feed: { ...base.feed, state: 'offline' }, market: { ...open, phase: 'closed', label: 'Closed' } }).marketClosed).toBe(true);
  });
  it('AC5: details name feed state, session expiry, market phase, clock offset and last tick', () => {
    expect(describeStatus(base).lines).toEqual([
      'Market data: live',
      'Session valid until 06:00 tomorrow',
      'Market: Open',
      'Clock offset +0.1 s (from ticks)',
      'Last tick: 10:29:59',
    ]);
  });
  it('FR-DATA-004 AC4: the offset shows its sign and source', () => {
    expect(describeStatus({ ...base, clock: { ...base.clock, millis: 3200, source: 'http-date' } }).lines[3]).toBe('Clock offset +3.2 s (from Kite server time)');
  });
  it('FR-DATA-001 AC5: an inferred non-trading day label passes through', () => {
    const v = describeStatus({ ...base, market: { ...open, phase: 'closed', dayType: 'inferred-non-trading', label: 'Non-trading day (inferred)' } });
    expect(v.lines[2]).toBe('Market: Non-trading day (inferred)');
    expect(v.marketClosed).toBe(true);
  });
});
