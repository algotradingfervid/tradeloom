import { afterEach, describe, expect, it, vi } from 'vitest';
import { createWailsBackend } from './wails';

type WailsWindow = Window & { go?: unknown; runtime?: unknown };

afterEach(() => {
  delete (window as WailsWindow).go;
  delete (window as WailsWindow).runtime;
});

describe('Wails adapter wiring (generated bindings call window.go.api.<Struct>.<Method>)', () => {
  it('routes calls and arguments to the bound Go methods', async () => {
    const Status = vi.fn().mockResolvedValue({ state: 'ready' });
    const SaveCredentials = vi.fn().mockResolvedValue({ state: 'logged_out' });
    const Set = vi.fn().mockResolvedValue({ name: 'appearance.theme', valueJson: '"dark"' });
    const OrderBook = vi.fn().mockResolvedValue({ orders: [], counts: { open: 0, executed: 0, rejected: 0, cancelled: 0 }, asOfMs: 0, stale: false });
    const Remove = vi.fn().mockResolvedValue({ version: '2026.1', coversUntil: '2026-12-31', stale: false, entries: [] });
    (window as WailsWindow).go = {
      api: { Session: { Status, SaveCredentials }, Settings: { Set }, Portfolio: { OrderBook }, Calendar: { Remove } },
    };
    const backend = createWailsBackend();
    await expect(backend.session.status()).resolves.toEqual({ state: 'ready' });
    await backend.session.saveCredentials({ apiKey: 'kiteapikey9f2c', apiSecret: 's3cret' });
    expect(SaveCredentials).toHaveBeenCalledWith(expect.objectContaining({ apiKey: 'kiteapikey9f2c', apiSecret: 's3cret' }));
    await backend.settings.set('appearance.theme', '"dark"');
    expect(Set).toHaveBeenCalledWith('appearance.theme', '"dark"');
    await expect(backend.portfolio.orderBook()).resolves.toMatchObject({ orders: [] });
    await backend.calendar.remove('NSE', '2026-10-20');
    expect(Remove).toHaveBeenCalledWith('NSE', '2026-10-20');
  });
  it('system.openExternal opens the URL through runtime.BrowserOpenURL (system browser, not the app webview)', async () => {
    const BrowserOpenURL = vi.fn();
    (window as WailsWindow).runtime = { BrowserOpenURL };
    await createWailsBackend().system.openExternal('https://www.tradingview.com/');
    expect(BrowserOpenURL).toHaveBeenCalledWith('https://www.tradingview.com/');
  });
  it('subscribes through runtime.EventsOnMultiple and returns its unsubscribe', () => {
    const off = vi.fn();
    const EventsOnMultiple = vi.fn(() => off);
    (window as WailsWindow).runtime = { EventsOnMultiple };
    const cb = vi.fn();
    const unsubscribe = createWailsBackend().on('feed.state', cb);
    expect(EventsOnMultiple).toHaveBeenCalledWith('feed.state', cb, -1);
    unsubscribe();
    expect(off).toHaveBeenCalledTimes(1);
  });
  it('normalises Go nil slices and maps (JSON null) to empty arrays and objects (Ruling 80)', async () => {
    const counts = { open: 0, executed: 0, rejected: 0, cancelled: 0 };
    const secrets = { mode: 'keyring', backend: 'keyring', keyringAvailable: true, fileExists: false, choices: null };
    (window as WailsWindow).go = {
      api: {
        Session: {
          Status: vi.fn().mockResolvedValue({ state: 'ready', secrets }),
          SecretStorage: vi.fn().mockResolvedValue(secrets),
          Profile: vi.fn().mockResolvedValue({ userId: 'AB1234', exchanges: null, products: null, orderTypes: null }),
        },
        Portfolio: {
          OrderBook: vi.fn().mockResolvedValue({ orders: null, counts, outOfScopeCount: 0, asOfMs: 0, stale: false }),
          Positions: vi.fn().mockResolvedValue({ net: null, day: null, netTotalPnlPaise: 0, dayTotalPnlPaise: 0, outOfScopeCount: 0, asOfMs: 0, stale: false }),
          Holdings: vi.fn().mockResolvedValue({ rows: null, summary: { empty: true }, asOfMs: 0, stale: false }),
        },
        Settings: {
          All: vi.fn().mockResolvedValue(null),
          Shortcuts: vi.fn().mockResolvedValue(null),
          ResetShortcuts: vi.fn().mockResolvedValue(null),
        },
        Data: { InspectBackup: vi.fn().mockResolvedValue({ path: '/b.tar.zst', counts: null }) },
        Calendar: {
          Info: vi.fn().mockResolvedValue({ version: '2026.1', entries: [{ exchange: 'NSE', date: '2026-10-20', type: 'holiday', phases: null, source: 'shipped' }] }),
          Import: vi.fn().mockResolvedValue({ version: '2026.1', entries: null }),
        },
        System: { Info: vi.fn().mockResolvedValue({ version: '0.1.0', licences: null }) },
      },
    };
    const b = createWailsBackend();
    expect((await b.session.status()).secrets.choices).toEqual([]);
    expect((await b.session.secretStorage()).choices).toEqual([]);
    expect(await b.session.profile()).toMatchObject({ exchanges: [], products: [], orderTypes: [] });
    expect((await b.portfolio.orderBook()).orders).toEqual([]);
    expect(await b.portfolio.positions()).toMatchObject({ net: [], day: [] });
    expect((await b.portfolio.holdings()).rows).toEqual([]);
    expect(await b.settings.all()).toEqual([]);
    expect(await b.settings.shortcuts()).toEqual([]);
    expect(await b.settings.resetShortcuts()).toEqual([]);
    expect((await b.data.inspectBackup('/b.tar.zst')).counts).toEqual({});
    expect((await b.calendar.info()).entries[0].phases).toEqual([]);
    expect((await b.calendar.import('/c.csv')).entries).toEqual([]);
    expect((await b.system.info()).licences).toEqual([]);
  });
  it('normalises null arrays in event payloads before the callback sees them', () => {
    const handlers = new Map<string, (p: unknown) => void>();
    (window as WailsWindow).runtime = {
      EventsOnMultiple: vi.fn((name: string, fn: (p: unknown) => void) => {
        handlers.set(name, fn);
        return () => undefined;
      }),
    };
    const b = createWailsBackend();
    const ticks = vi.fn();
    const book = vi.fn();
    const session = vi.fn();
    b.on('tick.batch', ticks);
    b.on('orders.changed', book);
    b.on('session.state', session);
    handlers.get('tick.batch')?.(null);
    handlers.get('orders.changed')?.({ orders: null, counts: {}, asOfMs: 1, stale: false, outOfScopeCount: 0 });
    handlers.get('session.state')?.({ state: 'ready', secrets: { mode: 'keyring', choices: null } });
    expect(ticks).toHaveBeenCalledWith([]);
    expect(book.mock.calls[0][0].orders).toEqual([]);
    expect(session.mock.calls[0][0].secrets.choices).toEqual([]);
  });
});
