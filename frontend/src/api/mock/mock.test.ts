import { describe, expect, it, vi } from 'vitest';
import { parseBackendError } from '../errors';
import { demoSeed, freshSeed, personalSeed, seedByName } from './fixtures';
import { createMockBackend } from './index';

describe('mock backend', () => {
  it('records calls and returns copies of its state', async () => {
    const m = createMockBackend(demoSeed());
    const h = await m.portfolio.holdings();
    h.rows[0].qty = 999;
    expect(m.state.holdings.rows[0].qty).not.toBe(999);
    expect(m.callsTo('portfolio.holdings')).toHaveLength(1);
  });
  it('fails the next call of a method once', async () => {
    const m = createMockBackend(demoSeed());
    m.failNext('portfolio.orderBook', 'network down');
    await expect(m.portfolio.orderBook()).rejects.toThrow('network down');
    await expect(m.portfolio.orderBook()).resolves.toMatchObject({ counts: { open: 1, executed: 1, rejected: 1, cancelled: 1 } });
  });
  it('emits events to subscribers and supports unsubscribe', () => {
    const m = createMockBackend(demoSeed());
    const cb = vi.fn();
    const off = m.on('feed.state', cb);
    // service.FeedStateDTO: Unix-ms times, 0 = never (P0-3 Ruling 67/71).
    const feed = { state: 'offline' as const, attempt: 0, nextRetryMs: 0, lastTickMs: 0, connections: 0, tokens: 0 };
    m.emit('feed.state', feed);
    off();
    m.emit('feed.state', { ...feed, state: 'live' });
    expect(cb).toHaveBeenCalledTimes(1);
    expect(m.listenerCount('feed.state')).toBe(0);
  });
  it('refuses a conflicting rebind naming the other action (FR-SHELL-019 AC2 contract)', async () => {
    const m = createMockBackend(demoSeed());
    const err = await m.settings.rebind('kill.switch', 'Alt+1').catch((e: unknown) => parseBackendError(e));
    expect(err).toEqual({ code: 'conflict', stage: 'shortcuts', reason: 'Go to Home' });
  });
  it('hides kite.* keys from Settings.All and refuses to set them', async () => {
    const m = createMockBackend(demoSeed());
    m.state.settings['kite.redirect_port'] = '7420';
    expect((await m.settings.all()).map((v) => v.name)).not.toContain('kite.redirect_port');
    const err = await m.settings.set('kite.redirect_port', '8080').catch((e: unknown) => parseBackendError(e));
    expect(err).toMatchObject({ code: 'use_credentials_flow' });
  });
  it('moves through setup → paste login → ready (risk disclosure still required)', async () => {
    const m = createMockBackend(freshSeed());
    await m.session.saveCredentials({ apiKey: 'kiteapikey9f2c', apiSecret: 'secretvalue1' });
    expect(m.state.session.state).toBe('logged_out');
    expect(m.state.session.credentials.apiKeyMasked).toBe('••••9f2c');
    const status = await m.session.completeLogin('http://127.0.0.1:7420/kite/callback?request_token=abc123XYZ&status=success');
    expect(status).toMatchObject({ state: 'ready', riskAckRequired: true });
  });
  it('never keeps the API secret in its state (only a masked key)', async () => {
    const m = createMockBackend(freshSeed());
    await m.session.saveCredentials({ apiKey: 'kiteapikey9f2c', apiSecret: 'secretvalue1' });
    expect(JSON.stringify(m.state)).not.toContain('secretvalue1');
  });
  it('saveCredentials applies Go\'s format checks (normalised, 6–64 letters and digits, key ≠ secret)', async () => {
    const m = createMockBackend(freshSeed());
    const refuse = (apiKey: string, apiSecret: string) => m.session.saveCredentials({ apiKey, apiSecret }).catch((e: unknown) => parseBackendError(e));
    expect(await refuse('abc', 'secretvalue1')).toMatchObject({ code: 'invalid_credentials', reason: expect.stringContaining('The API key should be 6–64 letters and digits') });
    expect(await refuse('kiteapikey9f2c', 'bad-secret')).toMatchObject({ code: 'invalid_credentials', reason: expect.stringContaining('The API secret should be 6–64 letters and digits') });
    expect(await refuse('kiteapikey9f2c', ' "kiteapikey9f2c" ')).toMatchObject({ code: 'invalid_credentials', reason: expect.stringContaining('identical') });
    expect(m.state.session.state).toBe('no_credentials');
    await m.session.saveCredentials({ apiKey: ' kiteapikey9f2c\n', apiSecret: "'secretvalue1'" });
    expect(m.state.session.credentials.apiKeyMasked).toBe('••••9f2c');
  });
  it('the personal seed is a ready session without a data plan', () => {
    expect(personalSeed().session).toMatchObject({ state: 'ready', dataPlan: 'none' });
    expect(seedByName('personal').session.dataPlan).toBe('none');
  });
});

describe('failAlways / clearFailures (Task 19 e2e ?fail=)', () => {
  it('fails every call until cleared, then answers again', async () => {
    const m = createMockBackend(demoSeed());
    m.failAlways('system.info', 'down');
    await expect(m.system.info()).rejects.toThrow('down');
    await expect(m.system.info()).rejects.toThrow('down');
    m.clearFailures();
    await expect(m.system.info()).resolves.toMatchObject({ version: expect.any(String) });
  });
});
