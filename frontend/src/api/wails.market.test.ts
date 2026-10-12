import { afterEach, describe, expect, it, vi } from 'vitest';
import { createWailsBackend } from './wails';

type WailsWindow = Window & { go?: unknown; runtime?: unknown };

afterEach(() => {
  delete (window as WailsWindow).go;
  delete (window as WailsWindow).runtime;
});

describe('Wails adapter — market domains', () => {
  it('routes positional arguments to the bound Go methods', async () => {
    const Search = vi.fn().mockResolvedValue([]);
    const Add = vi.fn().mockResolvedValue({ lists: [], activeId: '' });
    const OpenLive = vi.fn().mockResolvedValue('live-1');
    const Watch = vi.fn().mockResolvedValue({ live: [1], polled: [], snapshot: [] });
    (window as WailsWindow).go = { api: { Instruments: { Search }, Watchlists: { Add }, Chart: { OpenLive }, Quotes: { Watch } } };
    const b = createWailsBackend();
    await b.instruments.search('rel', ['NSE'], 50);
    expect(Search).toHaveBeenCalledWith('rel', ['NSE'], 50);
    await b.watchlists.add('wl-1', 738561, -1);
    expect(Add).toHaveBeenCalledWith('wl-1', 738561, -1);
    await expect(b.chart.openLive(256265, '60minute')).resolves.toBe('live-1');
    await expect(b.quotes.watch('watchlist', [1])).resolves.toEqual({ live: [1], polled: [], snapshot: [] });
  });
  // Deviation (Ruling 80): Go nil slices arrive as JSON null; the adapter hands callers [] instead.
  it('normalises Go nil slices in market responses to empty arrays', async () => {
    const result = { candles: null, complete: true, backfillJobId: '', needsDataPlan: false, offline: false };
    (window as WailsWindow).go = {
      api: {
        Chart: { Candles: vi.fn().mockResolvedValue(result), Refresh: vi.fn().mockResolvedValue(result) },
        Instruments: { Search: vi.fn().mockResolvedValue(null), ByTokens: vi.fn().mockResolvedValue(null) },
        Quotes: { Watch: vi.fn().mockResolvedValue({ live: null, polled: null, snapshot: null }) },
        Watchlists: {
          List: vi.fn().mockResolvedValue({ lists: null, activeId: '' }),
          Create: vi.fn().mockResolvedValue({ lists: [{ id: 'wl-1', name: 'Watchlist 1', items: null }], activeId: 'wl-1' }),
        },
      },
    };
    const b = createWailsBackend();
    const req = { token: 1, interval: 'day', fromMs: 0, toMs: 0, tradingDays: 750 };
    expect((await b.chart.candles(req)).candles).toEqual([]);
    expect((await b.chart.refresh({ token: 1, interval: 'day', fromMs: 0, toMs: 1 })).candles).toEqual([]);
    expect(await b.instruments.search('rel', [], 50)).toEqual([]);
    expect(await b.instruments.byTokens([1])).toEqual([]);
    expect(await b.quotes.watch('chart', [1])).toEqual({ live: [], polled: [], snapshot: [] });
    expect((await b.watchlists.list()).lists).toEqual([]);
    expect((await b.watchlists.create('Watchlist 1')).lists[0].items).toEqual([]);
  });
  it('normalises null candle.forming / backfill.progress batches and watchlists.changed items', () => {
    const handlers = new Map<string, (p: unknown) => void>();
    (window as WailsWindow).runtime = {
      EventsOnMultiple: vi.fn((name: string, fn: (p: unknown) => void) => {
        handlers.set(name, fn);
        return () => undefined;
      }),
    };
    const b = createWailsBackend();
    const forming = vi.fn();
    const progress = vi.fn();
    const lists = vi.fn();
    b.on('candle.forming', forming);
    b.on('backfill.progress', progress);
    b.on('watchlists.changed', lists);
    handlers.get('candle.forming')?.(null);
    handlers.get('backfill.progress')?.(null);
    handlers.get('watchlists.changed')?.({ lists: [{ id: 'wl-1', name: 'Watchlist 1', items: null }], activeId: 'wl-1' });
    expect(forming).toHaveBeenCalledWith([]);
    expect(progress).toHaveBeenCalledWith([]);
    expect(lists).toHaveBeenCalledWith({ lists: [{ id: 'wl-1', name: 'Watchlist 1', items: [] }], activeId: 'wl-1' });
  });
});
