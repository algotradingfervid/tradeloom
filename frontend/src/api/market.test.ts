import { beforeEach, describe, expect, it, vi } from 'vitest';
import { demoSeed, personalSeed } from './mock/fixtures';
import { createMockBackend, type MockBackend } from './mock/index';
import { setBackend } from './current';
import { BindingError } from './call';
import { instrumentByIdentifier, instrumentsByTokens, searchInstruments } from './instruments';
import { fetchHistory, onBackfillProgress, onCandle, refreshRange } from './chart';
import { watchQuotes } from './quotes';
import { watchlistsApi } from './watchlists';
import { NIFTY_OPTION, RELIANCE, RELIANCE_BSE, SBIN, TCS } from '../instruments/fixtures';

let backend: MockBackend;
const use = (b: MockBackend) => { backend = b; setBackend(b); };

beforeEach(() => use(createMockBackend(demoSeed())));

describe('market helpers over the Backend', () => {
  it('search passes tags and limit through; an empty query makes no call', async () => {
    expect((await searchInstruments('rel')).map((i) => i.tradingsymbol)).toContain('RELIANCE');
    expect(backend.callsTo('instruments.search')[0].args).toEqual(['rel', ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'], 50]);
    expect(await searchInstruments('   ')).toEqual([]);
    expect(backend.callsTo('instruments.search')).toHaveLength(1);
  });

  it('drops out-of-scope rows even if the backend sends one (NFR-15 defence in depth)', async () => {
    backend.state.instruments.push({ ...NIFTY_OPTION });
    expect(await instrumentsByTokens([RELIANCE.token, NIFTY_OPTION.token])).toEqual([RELIANCE]);
  });

  it('turns a refusal into a BindingError with its code', async () => {
    await expect(instrumentByIdentifier('NSE:NOPE')).rejects.toMatchObject({ code: 'not_found' });
    await expect(instrumentByIdentifier('NSE:NOPE')).rejects.toBeInstanceOf(BindingError);
  });

  it('fetchHistory asks for tradingDays ending at toMs and maps candles to Bars', async () => {
    const page = await fetchHistory(RELIANCE.token, 'day', 0, 750);
    expect(backend.callsTo('chart.candles')[0].args[0]).toEqual({ token: RELIANCE.token, interval: 'day', fromMs: 0, toMs: 0, tradingDays: 750 });
    expect(page.bars.length).toBeGreaterThanOrEqual(740);
    expect(Object.keys(page.bars[0])).toEqual(['time', 'open', 'high', 'low', 'close', 'volume']);
    expect(page).toMatchObject({ complete: true, backfillJobId: '', needsDataPlan: false, offline: false });
  });

  it('Personal plan: needsDataPlan is a normal result, not a rejection', async () => {
    use(createMockBackend(personalSeed()));
    await expect(fetchHistory(RELIANCE.token, '60minute', 0, 120)).resolves.toMatchObject({ bars: [], needsDataPlan: true });
  });

  it('a missing range returns a backfill job id, then backfill.progress reports it done', async () => {
    backend.state.backfillTokens = [SBIN.token];
    const seen: string[] = [];
    const off = onBackfillProgress((ps) => ps.forEach((p) => seen.push(`${p.jobId}:${p.state}`)));
    const first = await fetchHistory(SBIN.token, '5minute', 0, 15);
    expect(first.bars).toEqual([]);
    expect(first.backfillJobId).not.toBe('');
    await vi.waitFor(() => expect(seen).toEqual([`${first.backfillJobId}:done`]));
    expect((await fetchHistory(SBIN.token, '5minute', 0, 15)).bars.length).toBeGreaterThan(0);
    off();
  });

  it('refreshRange sends the range unchanged', async () => {
    await refreshRange(RELIANCE.token, '5minute', 1000, 2000);
    expect(backend.callsTo('chart.refresh')[0].args[0]).toEqual({ token: RELIANCE.token, interval: '5minute', fromMs: 1000, toMs: 2000 });
  });

  it('onCandle flattens the coalesced candle.forming array and the single candle.closed event', () => {
    const seen: unknown[] = [];
    const off = onCandle((e) => seen.push(e));
    const c = (close: number, complete: boolean) => ({ t: 1, o: 1, h: 2, l: 1, c: close, v: 3, oi: 0, complete });
    backend.emit('candle.forming', [
      { token: 5, interval: '5minute', candle: c(1, false), partial: false },
      { token: 6, interval: '5minute', candle: c(9, false), partial: false },
    ]);
    backend.emit('candle.closed', { token: 5, interval: '5minute', candle: c(2, true), partial: false });
    expect(seen).toEqual([
      { token: 5, interval: '5minute', closed: false, bar: { time: 1, open: 1, high: 2, low: 1, close: 1, volume: 3 } },
      { token: 6, interval: '5minute', closed: false, bar: { time: 1, open: 1, high: 2, low: 1, close: 9, volume: 3 } },
      { token: 5, interval: '5minute', closed: true, bar: { time: 1, open: 1, high: 2, low: 1, close: 2, volume: 3 } },
    ]);
    off();
    expect(backend.listenerCount('candle.forming')).toBe(0);
    expect(backend.listenerCount('candle.closed')).toBe(0);
  });

  it('watchQuotes: budget overflow is polled; snapshot is the in-memory last quote per token', async () => {
    backend.state.liveBudget = 1;
    const r = await watchQuotes('watchlist', [256265, RELIANCE.token]);
    expect(r.live).toEqual([256265]);
    expect(r.polled).toEqual([RELIANCE.token]);
    expect(r.snapshot.map((t) => [t.token, t.ltp])).toEqual([[256265, 22500], [RELIANCE.token, 1290]]);
    expect(r.snapshot.every((t) => t.close > 0 && t.polled === false)).toBe(true);
  });

  it('watchlists: append, already_in_list, remove position, last list deleted → "Watchlist 1"', async () => {
    await watchlistsApi.add('wl-1', SBIN.token);
    const s = await watchlistsApi.add('wl-1', TCS.token);
    expect(s.lists[0].items.map((i) => i.symbol)).toEqual(['SBIN', 'TCS']);
    await expect(watchlistsApi.add('wl-1', SBIN.token)).rejects.toMatchObject({ code: 'already_in_list' });
    expect(await watchlistsApi.remove('wl-1', SBIN.token)).toEqual({ listId: 'wl-1', token: SBIN.token, position: 0 });
    const after = await watchlistsApi.delete('wl-1');
    expect(after.lists.map((l) => [l.name, l.items.length])).toEqual([['Watchlist 1', 0]]);
    expect(after.activeId).toBe(after.lists[0].id);
  });

  // Deviation: the generated service.InstrumentDTO has alsoOnNse, which Go fills on Search results only.
  it('mock search marks a BSE row also listed on NSE; byTokens never does', async () => {
    const rows = await backend.instruments.search('reliance', ['NSE', 'BSE'], 50);
    expect(rows.map((r) => [r.exchange, r.alsoOnNse])).toEqual([['NSE', false], ['BSE', true]]);
    expect((await backend.instruments.byTokens([RELIANCE_BSE.token]))[0].alsoOnNse).toBe(false);
  });

  it('watchlists: name errors keep their codes', async () => {
    await expect(watchlistsApi.create('  ')).rejects.toMatchObject({ code: 'name_invalid' });
    await expect(watchlistsApi.create('watchlist 1')).rejects.toMatchObject({ code: 'name_exists' });
  });
});
