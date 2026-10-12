import { describe, expect, it, vi } from 'vitest';
import { useAppStore } from '../store/app';
import { useSessionStore } from '../store/session';
import { useSettingsStore } from '../store/settings';
import { useStatusStore } from '../store/status';
import { registerReset } from '../store/reset';
import { useTicksStore } from '../store/ticks';
import { bootstrap, startEventBridge } from './events';
import { demoSeed, FIXED_NOW, freshSeed, readySession, tickFixture } from './mock/fixtures';
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
