import { describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { useSettingsStore } from '../store/settings';
import { useStatusStore } from '../store/status';
import { registerReset } from '../store/reset';
import { useTicksStore } from '../store/ticks';
import { loadPortfolio, usePortfolioStore } from '../store/portfolio';
import type { HoldingsSnapshot } from './backend';
import { bootstrap, startEventBridge } from './events';
import { demoSeed, FIXED_NOW, freshSeed, holdingA, holdingsOf, readySession, tickFixture } from './mock/fixtures';
import { createMockBackend } from './mock/index';

describe('event bridge', () => {
  it('dispatches each backend event into its store and unsubscribes on stop', () => {
    const backend = createMockBackend(demoSeed());
    const navigate = vi.fn();
    const stop = startEventBridge(backend, { navigate });
    backend.emit('session.state', readySession({ state: 'expired' }));
    backend.emit('feed.state', { state: 'reconnecting', attempt: 2, nextRetryMs: FIXED_NOW + 5000, lastTickMs: 0, connections: 0, tokens: 3 });
    backend.emit('market.status', { exchange: 'NSE', phase: 'closed', dayType: 'holiday', label: 'Holiday', tooltip: 'Exchange holiday', atMs: FIXED_NOW });
    backend.emit('clock.offset', { millis: 3200, source: 'ticks', samples: 12, warn: true, skewed: false, updatedAtMs: FIXED_NOW });
    backend.emit('tick.batch', [tickFixture(1, 10, 9)]);
    backend.emit('settings.changed', { key: 'appearance.theme', value: 'dark' });
    backend.emit('app.navigate', { route: '/holdings' });
    expect(useSessionStore.getState().session?.state).toBe('expired');
    expect(useStatusStore.getState().feed).toMatchObject({ state: 'reconnecting', attempt: 2, nextRetryMs: FIXED_NOW + 5000 });
    expect(useStatusStore.getState().market?.label).toBe('Holiday');
    expect(useStatusStore.getState().clock?.millis).toBe(3200);
    expect(useTicksStore.getState().latest[1].ltp).toBe(10);
    expect(useSettingsStore.getState().values['appearance.theme']).toBe('dark');
    expect(navigate).toHaveBeenCalledWith('/holdings');
    stop();
    for (const name of ['session.state', 'feed.state', 'market.status', 'clock.offset', 'tick.batch', 'settings.changed', 'app.navigate'] as const) {
      expect(backend.listenerCount(name)).toBe(0);
    }
  });
  it('bootstrap loads session, status, settings and app info', async () => {
    const seed = demoSeed();
    seed.settings = { 'appearance.theme': '"dark"' };
    await bootstrap(createMockBackend(seed));
    expect(useSessionStore.getState().session?.state).toBe('ready');
    expect(useStatusStore.getState().feed.state).toBe('live');
    expect(useSettingsStore.getState()).toMatchObject({ loaded: true, values: { 'appearance.theme': 'dark' } });
    expect(useAppStore.getState().info?.version).toBe('0.1.0');
  });
  // Final review I1: Settings.All never lists the credential keys (kite.*), and Go filters them from the event too;
  // the bridge ignores them as well so a credential can never land in the settings store.
  it('ignores settings.changed for credential keys (kite.*)', () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    backend.emit('settings.changed', { key: 'kite.api_key', value: 'plantedApiKey0123' });
    backend.emit('settings.changed', { key: 'kite.redirect_port', value: 7421 });
    backend.emit('settings.changed', { key: 'appearance.theme', value: 'dark' });
    const { values } = useSettingsStore.getState();
    expect(Object.keys(values).filter((k) => k.startsWith('kite.'))).toEqual([]);
    expect(JSON.stringify(values)).not.toContain('plantedApiKey0123');
    expect(values['appearance.theme']).toBe('dark');
    stop();
  });
});

describe('bootstrap vs events (Ruling 83-F I1)', () => {
  it('a feed.state live emitted before system.status() resolves stays live', async () => {
    const backend = createMockBackend(freshSeed()); // System.Status reports the feed stopped
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    const done = bootstrap(backend);
    backend.emit('feed.state', { state: 'live', attempt: 0, nextRetryMs: 0, lastTickMs: FIXED_NOW, connections: 1, tokens: 3 });
    await done;
    expect(useStatusStore.getState().feed.state).toBe('live');
    expect(useStatusStore.getState().market?.label).toBe('Open'); // slices without an event still load
    stop();
  });
  it('a session.state emitted during bootstrap wins over the older Status() result', async () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    const done = bootstrap(backend);
    backend.emit('session.state', readySession({ state: 'expired' }));
    await done;
    expect(useSessionStore.getState().session?.state).toBe('expired');
    stop();
  });
  it('settings hydrate merges: keys changed since subscribing keep their newer value', async () => {
    const seed = demoSeed();
    seed.settings = { 'appearance.theme': '"light"', 'appearance.sidebar_collapsed': 'true' };
    const backend = createMockBackend(seed);
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    const done = bootstrap(backend);
    backend.emit('settings.changed', { key: 'appearance.theme', value: 'dark' });
    await done;
    expect(useSettingsStore.getState()).toMatchObject({
      loaded: true, values: { 'appearance.theme': 'dark', 'appearance.sidebar_collapsed': true },
    });
    stop();
  });
});

describe('bootstrap failures and cancellation (Ruling 83-F I2, M6)', () => {
  it('records the failed slices, leaves settings unloaded and still applies the rest', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('settings.all', 'database is locked');
    const result = await bootstrap(backend);
    expect(result.failed).toEqual(['settings']);
    expect(useSettingsStore.getState().loaded).toBe(false);
    expect(useAppStore.getState().bootError).toEqual({ failed: ['settings'], reason: 'database is locked' });
    expect(useSessionStore.getState().session?.state).toBe('ready');
    await bootstrap(backend);
    expect(useAppStore.getState().bootError).toBeNull();
    expect(useSettingsStore.getState().loaded).toBe(true);
  });
  it('an aborted bootstrap applies nothing', async () => {
    const backend = createMockBackend(demoSeed());
    const ctl = new AbortController();
    const done = bootstrap(backend, { signal: ctl.signal });
    ctl.abort();
    await done;
    expect(useSessionStore.getState().session).toBeNull();
    expect(useSettingsStore.getState().loaded).toBe(false);
    expect(useAppStore.getState().info).toBeNull();
    expect(useAppStore.getState().bootError).toBeNull();
  });
});

describe('account data is cleared on sign-out (Ruling 83-F M5)', () => {
  it('runs account resets once on the transition to logged_out or no_credentials, never ticks', () => {
    const accountReset = vi.fn();
    registerReset(accountReset);
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    backend.emit('tick.batch', [tickFixture(1, 10, 9)]);
    backend.emit('session.state', readySession());
    backend.emit('settings.changed', { key: 'appearance.theme', value: 'dark' });
    expect(accountReset).not.toHaveBeenCalled();
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    expect(accountReset).toHaveBeenCalledTimes(1);
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    backend.emit('session.state', readySession({ state: 'no_credentials' }));
    expect(accountReset).toHaveBeenCalledTimes(1);
    backend.emit('session.state', readySession());
    backend.emit('session.state', readySession({ state: 'no_credentials' }));
    expect(accountReset).toHaveBeenCalledTimes(2);
    expect(useTicksStore.getState().latest[1].ltp).toBe(10);
    expect(useSettingsStore.getState().values).toEqual({ 'appearance.theme': 'dark' });
    expect(useSessionStore.getState().session?.state).toBe('no_credentials');
    stop();
  });
});

describe('a different user signing in clears account data (Ruling 98)', () => {
  async function switchUser(viaLoggingIn: boolean) {
    const accountReset = vi.fn();
    registerReset(accountReset);
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    backend.emit('session.state', readySession({ userId: 'AA1111', sessionId: 'sess-a' }));
    backend.emit('holdings.changed', holdingsOf([holdingA], FIXED_NOW));
    expect(usePortfolioStore.getState().holdings.data.rows).toHaveLength(1);
    let resolve!: (h: HoldingsSnapshot) => void;
    backend.portfolio.holdings = () => new Promise<HoldingsSnapshot>((r) => { resolve = r; });
    const read = loadPortfolio('holdings', backend);
    backend.emit('session.state', readySession({ state: 'expired', userId: 'AA1111', sessionId: 'sess-a' }));
    if (viaLoggingIn) backend.emit('session.state', readySession({ state: 'logging_in', userId: '', sessionId: '' }));
    backend.emit('session.state', readySession({ userId: 'BB2222', sessionId: 'sess-b', riskAckRequired: true }));
    expect(accountReset).toHaveBeenCalledTimes(1);
    resolve(holdingsOf([holdingA], FIXED_NOW + 1));
    await read;
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'idle', loading: false, data: { rows: [], asOfMs: 0 } });
    expect(useSessionStore.getState().session?.userId).toBe('BB2222');
    // B's first snapshot (asOfMs 0 or later) is accepted: nothing of A's is left to out-rank it.
    backend.emit('holdings.changed', holdingsOf([], 0));
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'ready', data: { rows: [] } });
    stop();
  }

  it('expired(A) → logging_in → ready(B): A\'s stores are cleared and A\'s in-flight read is dropped', () => switchUser(true));
  it('paste login expired(A) → ready(B), with no logging_in in between', () => switchUser(false));

  it('the same user signing in again keeps account data; the first session after start never resets', () => {
    const accountReset = vi.fn();
    registerReset(accountReset);
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    backend.emit('session.state', readySession());
    backend.emit('session.state', readySession({ state: 'expired' }));
    backend.emit('session.state', readySession({ sessionId: 'sess-2' }));
    expect(accountReset).not.toHaveBeenCalled();
    stop();
  });

  it('a user known only from bootstrap (no earlier event) still counts as the previous user', async () => {
    const accountReset = vi.fn();
    registerReset(accountReset);
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: vi.fn() });
    await bootstrap(backend);
    backend.emit('session.state', readySession({ userId: 'BB2222', sessionId: 'sess-b' }));
    expect(accountReset).toHaveBeenCalledTimes(1);
    stop();
  });
});
