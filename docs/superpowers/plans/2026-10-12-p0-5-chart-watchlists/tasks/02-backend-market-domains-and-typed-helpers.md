> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 2.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 2: Backend market domains (real + mock) and typed helpers

This task follows P0-4 Task 2's backend pattern: the `Backend` interface gains four domains, `api/wails.ts` implements them over the P0-3 bindings, and `api/mock/` implements them in memory for Vitest, `npm run dev:mock` and Playwright. The rest of this plan never sees a binding. It calls thin typed helpers that read the backend registered with `setBackend` (`<MarketDataBridge>` registers it once, tests register a mock).

**Files:**
- Modify (P0-4 files): `frontend/src/api/backend.ts` (DTO types, `Backend` domains, `EventMap` entries), `frontend/src/api/wails.ts` (domains over the bindings), `frontend/src/api/mock/index.ts` (`MockState` market fields, wrapped domains), `frontend/src/api/mock/fixtures.ts` (`base()` gets the market seed)
- Create: `frontend/src/api/mock/market.ts`, `frontend/src/api/current.ts`, `frontend/src/api/call.ts`, `frontend/src/api/instruments.ts`, `frontend/src/api/chart.ts`, `frontend/src/api/quotes.ts`, `frontend/src/api/watchlists.ts`
- Create: `frontend/src/chart/types.ts` (the `Bar` type lives in the chart domain; `api/chart.ts` maps DTOs into it)
- Test: `frontend/src/api/market.test.ts`, `frontend/src/api/wails.market.test.ts`

**Interfaces:**
- Consumes: P0-3 bindings `Chart.{Candles,Refresh,OpenLive,CloseLive}`, `Instruments.{Search,ByTokens,ByIdentifier}`, `Quotes.Watch`, `Watchlists.{List,Create,Rename,Delete,SetActive,Add,Remove,Reorder,ReorderLists}`, models `service.CandlesRequest`/`service.RefreshRequest` (Wails `createFrom`), events `tick.batch`, `candle.forming`, `candle.closed`, `backfill.progress` (see Consumes). P0-4 Task 2 `Backend`, `EventMap`, `EventName`, `TickDTO`, `Ms`, `createWailsBackend` (+ its local `cast`), `createMockBackend` (+ its `wrap`, `emit`), `MockState`, `MockBackend`, `demoSeed`, `personalSeed`, `parseBackendError`. Task 1 `Instrument`, `isInScope`, `EXCHANGE_TAGS`, `ExchangeTag`, `KiteInterval`, fixtures.
- Produces:
  - `backend.ts`: `InstrumentDTO`, `CandleDTO`, `CandleEventDTO`, `CandlesRequest`, `RefreshRequest`, `CandlesResult`, `BackfillState`, `BackfillProgress`, `QuoteConsumer`, `WatchResult`, `WatchlistItem`, `Watchlist`, `WatchlistsState`, `RemovedItem`; `Backend.chart { candles(req); refresh(req); openLive(token, interval); closeLive(id) }`, `Backend.instruments { search(q, tags, limit); byTokens(tokens); byIdentifier(id) }`, `Backend.quotes { watch(consumer, tokens) }`, `Backend.watchlists { list(); create(name); rename(id, name); delete(id); setActive(id); add(id, token, position); remove(id, token); reorder(id, tokens); reorderLists(ids) }`; `EventMap['candle.forming' | 'candle.closed' | 'backfill.progress']`
  - `mock/market.ts`: `interface MarketMockState { instruments; candles; watchlists; quotes; liveBudget; backfillTokens }`, `marketSeedState()`, `createMarketDomains(state, emit)`, `watchlistItemOf(inst)`, `quoteFor(token, ltp, close, tsMs?)`, `MOCK_MARKET_NOW`, `MOCK_BASE_PRICE`, `MOCK_LIVE_BUDGET = 200`
  - `current.ts`: `setBackend(b: Backend): void`, `currentBackend(): Backend`
  - `call.ts`: `class BindingError extends Error { code: string; stage: string }`, `call<T>(p: Promise<T>): Promise<T>`
  - `chart/types.ts`: `interface Bar { time /*ms UTC candle start*/; open; high; low; close; volume }`, `LegendData { bar; previousClose: number | null }` and the other chart types
  - `instruments.ts`: `toInstrument(d)`, `searchInstruments(q, tags = EXCHANGE_TAGS, limit = 50)` (empty query → `[]` without a call), `instrumentsByTokens(tokens)`, `instrumentByIdentifier(id)`
  - `chart.ts`: `interface CandlesPage { bars: Bar[]; complete; backfillJobId; needsDataPlan; offline }`, `interface CandleEvent { token; interval; bar: Bar; closed: boolean }`, `fetchHistory(token, interval, toMs, tradingDays)`, `refreshRange(token, interval, fromMs, toMs)`, `openLive(token, interval)`, `closeLive(id)`, `onCandle(cb)`, `onBackfillProgress(cb)`, `toBar(c)`
  - `quotes.ts`: `QUOTE_CONSUMERS`, `watchQuotes(consumer, tokens): Promise<WatchResult>`, `onTickBatch(cb)`
  - `watchlists.ts`: `watchlistsApi.{list, create, rename, delete, setActive, add, remove, reorder, reorderLists}` (each resolves to `WatchlistsState`, except `remove` → `RemovedItem`)

Precondition: P0-3 has committed `frontend/wailsjs/go/api/{Chart,Instruments,Quotes,Watchlists}.d.ts` and P0-4 Task 2 is merged. Check with `ls frontend/wailsjs/go/api/ && grep -n "personalSeed" frontend/src/api/mock/fixtures.ts`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/api/market.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { demoSeed, personalSeed } from './mock/fixtures';
import { createMockBackend, type MockBackend } from './mock/index';
import { setBackend } from './current';
import { BindingError } from './call';
import { instrumentByIdentifier, instrumentsByTokens, searchInstruments } from './instruments';
import { fetchHistory, onBackfillProgress, onCandle, refreshRange } from './chart';
import { watchQuotes } from './quotes';
import { watchlistsApi } from './watchlists';
import { NIFTY_OPTION, RELIANCE, SBIN, TCS } from '../instruments/fixtures';

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

  it('watchlists: name errors keep their codes', async () => {
    await expect(watchlistsApi.create('  ')).rejects.toMatchObject({ code: 'name_invalid' });
    await expect(watchlistsApi.create('watchlist 1')).rejects.toMatchObject({ code: 'name_exists' });
  });
});
```

`frontend/src/api/wails.market.test.ts` (same technique as P0-4's `wails.test.ts`: the generated bindings call `window.go.api.<Struct>.<Method>`):

```ts
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createWailsBackend } from './wails';

type WailsWindow = Window & { go?: unknown };

afterEach(() => {
  delete (window as WailsWindow).go;
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
});
```

- [ ] **Step 2: Run the tests to check they fail**

Run: `cd frontend && npx vitest run src/api/market.test.ts src/api/wails.market.test.ts`
Expected: FAIL with `Failed to resolve import "./current"` (market test) and `TypeError: Cannot read properties of undefined (reading 'search')` (wails test: `b.instruments` does not exist yet).

- [ ] **Step 3: Implement the types and extend the Backend contract**

`frontend/src/chart/types.ts`:

```ts
/** One candle. time = candle start in ms since epoch (UTC); prices in rupees. */
export interface Bar { time: number; open: number; high: number; low: number; close: number; volume: number }

export interface InstrumentSpec { intraday: boolean; hasVolume: boolean; pricePrecision: number; minMove: number }

export interface ChartTheme {
  background: string; text: string; grid: string; crosshair: string; separator: string;
  up: string; down: string; volumeUp: string; volumeDown: string;
}

export interface StudyPoint { time: number; value: number } // NaN value = gap (warm-up / session reset)
export interface StudySpec { id: string; kind: 'line' | 'histogram'; pane: 'overlay' | 'separate'; color: string; points: readonly StudyPoint[] }
export interface PriceBandSpec { id: string; from: number; to: number; color: string }
export interface Handle { readonly id: string }
/** Legend for one bar; previousClose is the close of the bar before it (null for the first loaded bar). */
export interface LegendData { bar: Bar; previousClose: number | null }
export interface RangeInfo { barsBefore: number }
export interface TimeRange { from: number; to: number } // ms
export type UpdateOutcome = 'appended' | 'replaced' | 'historical' | 'ignored';
```

Append to `frontend/src/api/backend.ts` (after P0-4's model types, before `EventMap`):

```ts
// ---- P0-5 market domains (canonical contract §C/§D; JSON of the service.* models) ----
export interface InstrumentDTO {
  token: number;
  exchange: string;
  segment: string;
  tag: string;
  tradingsymbol: string;
  name: string;
  displayName: string;
  instrumentType: string;
  expiryMs: Ms; // 0 = none
  tickSizePaise: number;
  lotSize: number;
  isIndex: boolean;
  isFuture: boolean;
  underlying: string;
}
export interface CandleDTO { t: Ms; o: number; h: number; l: number; c: number; v: number; oi: number; complete: boolean }
export interface CandleEventDTO { token: number; interval: string; candle: CandleDTO; partial: boolean }
export interface CandlesRequest { token: number; interval: string; fromMs: Ms; toMs: Ms; tradingDays: number }
export interface RefreshRequest { token: number; interval: string; fromMs: Ms; toMs: Ms }
export interface CandlesResult { candles: CandleDTO[]; complete: boolean; backfillJobId: string; needsDataPlan: boolean; offline: boolean }
export type BackfillState = 'running' | 'done' | 'failed';
export interface BackfillProgress {
  jobId: string; token: number; interval: string; done: number; failed: number; total: number; etaNs: number; state: BackfillState;
}
export type QuoteConsumer = 'chart' | 'watchlist' | 'widget';
export interface WatchResult { live: number[]; polled: number[]; snapshot: TickDTO[] }
export interface WatchlistItem { token: number; exchange: string; symbol: string; displayName: string; name: string; isIndex: boolean; missing: boolean }
export interface Watchlist { id: string; name: string; items: WatchlistItem[] }
export interface WatchlistsState { lists: Watchlist[]; activeId: string }
export interface RemovedItem { listId: string; token: number; position: number }
```

In the same file add three entries to `interface EventMap` (P0-4 already declares `'tick.batch': TickDTO[]`, which stays as is):

```ts
  'candle.forming': CandleEventDTO[];
  'candle.closed': CandleEventDTO;
  'backfill.progress': BackfillProgress[];
```

and four members to `interface Backend`, before `on<K extends EventName>(…)`:

```ts
  chart: {
    candles(req: CandlesRequest): Promise<CandlesResult>;
    refresh(req: RefreshRequest): Promise<CandlesResult>;
    openLive(token: number, interval: string): Promise<string>;
    closeLive(id: string): Promise<void>;
  };
  instruments: {
    search(q: string, tags: string[], limit: number): Promise<InstrumentDTO[]>;
    byTokens(tokens: number[]): Promise<InstrumentDTO[]>;
    byIdentifier(id: string): Promise<InstrumentDTO>;
  };
  quotes: { watch(consumer: QuoteConsumer, tokens: number[]): Promise<WatchResult> };
  watchlists: {
    list(): Promise<WatchlistsState>;
    create(name: string): Promise<WatchlistsState>;
    rename(id: string, name: string): Promise<WatchlistsState>;
    delete(id: string): Promise<WatchlistsState>;
    setActive(id: string): Promise<WatchlistsState>;
    add(id: string, token: number, position: number): Promise<WatchlistsState>;
    remove(id: string, token: number): Promise<RemovedItem>;
    reorder(id: string, tokens: number[]): Promise<WatchlistsState>;
    reorderLists(ids: string[]): Promise<WatchlistsState>;
  };
```

In `frontend/src/api/wails.ts` add these imports next to P0-4's binding imports, and add `service` to the existing `../../wailsjs/go/models` import (`import { service } from '../../wailsjs/go/models';` if the file has no other models import):

```ts
import * as Chart from '../../wailsjs/go/api/Chart';
import * as Instruments from '../../wailsjs/go/api/Instruments';
import * as Quotes from '../../wailsjs/go/api/Quotes';
import * as Watchlists from '../../wailsjs/go/api/Watchlists';
```

and add these members to the object returned by `createWailsBackend()`, before `on:` (`cast` is P0-4's local helper in that file):

```ts
    chart: {
      candles: (req) => cast(Chart.Candles(service.CandlesRequest.createFrom(req))),
      refresh: (req) => cast(Chart.Refresh(service.RefreshRequest.createFrom(req))),
      openLive: (token, interval) => Chart.OpenLive(token, interval),
      closeLive: (id) => Chart.CloseLive(id),
    },
    instruments: {
      search: (q, tags, limit) => cast(Instruments.Search(q, tags, limit)),
      byTokens: (tokens) => cast(Instruments.ByTokens(tokens)),
      byIdentifier: (id) => cast(Instruments.ByIdentifier(id)),
    },
    quotes: { watch: (consumer, tokens) => cast(Quotes.Watch(consumer, tokens)) },
    watchlists: {
      list: () => cast(Watchlists.List()),
      create: (name) => cast(Watchlists.Create(name)),
      rename: (id, name) => cast(Watchlists.Rename(id, name)),
      delete: (id) => cast(Watchlists.Delete(id)),
      setActive: (id) => cast(Watchlists.SetActive(id)),
      add: (id, token, position) => cast(Watchlists.Add(id, token, position)),
      remove: (id, token) => cast(Watchlists.Remove(id, token)),
      reorder: (id, tokens) => cast(Watchlists.Reorder(id, tokens)),
      reorderLists: (ids) => cast(Watchlists.ReorderLists(ids)),
    },
```

- [ ] **Step 4: Implement the mock domains and the typed helpers**

`frontend/src/api/mock/market.ts` (deterministic in-memory market for Vitest, `npm run dev:mock` and Playwright):

```ts
import type {
  BackfillProgress, CandleDTO, CandlesRequest, CandlesResult, EventMap, EventName, InstrumentDTO, QuoteConsumer,
  RefreshRequest, RemovedItem, TickDTO, WatchlistItem, WatchlistsState, WatchResult,
} from '../backend';
import { ALL_IN_SCOPE, NEWCO } from '../../instruments/fixtures';
import type { MockState } from './index';

const MIN = 60_000;
const STEP: Record<string, number> = {
  minute: MIN, '3minute': 3 * MIN, '5minute': 5 * MIN, '10minute': 10 * MIN, '15minute': 15 * MIN,
  '30minute': 30 * MIN, '60minute': 60 * MIN, day: 1440 * MIN,
};
/** 09 Oct 2026 11:30 IST: the mock market's "now". */
export const MOCK_MARKET_NOW = Date.UTC(2026, 9, 9, 6, 0);
const HISTORY_START = Date.UTC(2024, 0, 1);
const ALIASES: Record<number, string> = { 260105: 'banknifty' };
export const MOCK_BASE_PRICE: Record<number, number> = { 256265: 22500, 260105: 48000, 265: 74000, 274441: 55000, 738561: 1290 };
/** Small on purpose so e2e can exercise the "polled" hint. */
export const MOCK_LIVE_BUDGET = 200;
const CONSUMERS: readonly QuoteConsumer[] = ['chart', 'watchlist', 'widget'];

export interface MarketMockState {
  instruments: InstrumentDTO[];
  /** Explicit candle series keyed `${token}:${interval}`; any other (token, interval) is generated. */
  candles: Record<string, CandleDTO[]>;
  watchlists: WatchlistsState;
  /** In-memory last quote per token: what Quotes.Watch returns as `snapshot`. */
  quotes: Record<number, TickDTO>;
  liveBudget: number;
  /** Tokens whose next Candles call reports a backfill job first (exercises the wait path). */
  backfillTokens: number[];
}

export function quoteFor(token: number, ltp: number, close: number, tsMs = MOCK_MARKET_NOW): TickDTO {
  return { token, ltp, close, change: ltp - close, changePct: close > 0 ? ((ltp - close) / close) * 100 : 0, volume: 0, tsMs, polled: false };
}

export function watchlistItemOf(i: InstrumentDTO): WatchlistItem {
  return { token: i.token, exchange: i.exchange, symbol: i.tradingsymbol, displayName: i.displayName, name: i.name, isIndex: i.isIndex, missing: false };
}

export function marketSeedState(): MarketMockState {
  const quotes: Record<number, TickDTO> = {};
  for (const [t, p] of Object.entries(MOCK_BASE_PRICE)) quotes[Number(t)] = quoteFor(Number(t), p, p * 0.995);
  return {
    instruments: ALL_IN_SCOPE.map((i) => ({ ...i })),
    candles: {},
    watchlists: { lists: [{ id: 'wl-1', name: 'Watchlist 1', items: [] }], activeId: 'wl-1' },
    quotes,
    liveBudget: MOCK_LIVE_BUDGET,
    backfillTokens: [],
  };
}

const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) as T;
const refuse = (code: string, reason: string): Error => new Error(JSON.stringify({ code, stage: 'market', reason }));
type Emit = <K extends EventName>(name: K, payload: EventMap[K]) => void;

function priceAt(token: number, t: number, step: number): number {
  const base = MOCK_BASE_PRICE[token] ?? 1000;
  return base * (1 + 0.02 * Math.sin((t / step) * 0.05 + token));
}

/** Candles on a fixed grid ending one step before MOCK_MARKET_NOW, newest at or before `toMs` (0 = now). */
function generate(state: MarketMockState, token: number, interval: string, toMs: number, tradingDays: number): CandleDTO[] {
  const explicit = state.candles[`${token}:${interval}`];
  const limit = toMs > 0 ? toMs : MOCK_MARKET_NOW;
  if (explicit) return explicit.filter((c) => c.t <= limit).map((c) => ({ ...c }));
  if (token === NEWCO.token) return [];
  const step = STEP[interval] ?? 5 * MIN;
  const perDay = interval === 'day' ? 1 : Math.floor((375 * MIN) / step);
  const anchor = MOCK_MARKET_NOW - step;
  const first = anchor <= limit ? 0 : Math.ceil((anchor - limit) / step);
  const isIndex = state.instruments.find((i) => i.token === token)?.isIndex ?? false;
  const out: CandleDTO[] = [];
  for (let k = first; k < first + tradingDays * perDay; k++) {
    const t = anchor - k * step;
    if (t < HISTORY_START) break;
    const o = priceAt(token, t, step);
    const c = priceAt(token, t + step, step);
    const v = isIndex ? 0 : 1000 + (Math.abs(Math.round(t / step)) % 50) * 10;
    out.push({ t, o, h: Math.max(o, c) * 1.001, l: Math.min(o, c) * 0.999, c, v, oi: 0, complete: true });
  }
  return out.reverse();
}

export function createMarketDomains(state: MockState, emit: Emit) {
  let liveSeq = 0;
  let jobSeq = 0;
  const haystack = (i: InstrumentDTO) => `${i.tradingsymbol} ${i.name} ${i.displayName} ${ALIASES[i.token] ?? ''}`.toLowerCase();
  const listOf = (id: string) => {
    const l = state.watchlists.lists.find((x) => x.id === id);
    if (!l) throw refuse('not_found', `Watchlist ${id} not found`);
    return l;
  };
  const checkName = (name: string, exceptId?: string): string => {
    const t = name.trim();
    if (t.length === 0 || t.length > 40) throw refuse('name_invalid', 'Use 1 to 40 characters');
    if (state.watchlists.lists.some((l) => l.id !== exceptId && l.name.toLowerCase() === t.toLowerCase())) {
      throw refuse('name_exists', 'A watchlist with this name exists');
    }
    return t;
  };
  const newListId = (): string => {
    let n = state.watchlists.lists.length;
    do { n += 1; } while (state.watchlists.lists.some((l) => l.id === `wl-${n}`));
    return `wl-${n}`;
  };

  return {
    chart: {
      candles: (req: CandlesRequest): CandlesResult => {
        if (state.session.dataPlan === 'none') {
          return { candles: [], complete: false, backfillJobId: '', needsDataPlan: true, offline: false };
        }
        if (state.backfillTokens.includes(req.token)) {
          state.backfillTokens = state.backfillTokens.filter((t) => t !== req.token);
          const jobId = `bf-${++jobSeq}`;
          const done: BackfillProgress = { jobId, token: req.token, interval: req.interval, done: 1, failed: 0, total: 1, etaNs: 0, state: 'done' };
          setTimeout(() => emit('backfill.progress', [done]), 20);
          return { candles: [], complete: false, backfillJobId: jobId, needsDataPlan: false, offline: false };
        }
        return { candles: generate(state, req.token, req.interval, req.toMs, req.tradingDays), complete: true, backfillJobId: '', needsDataPlan: false, offline: false };
      },
      refresh: (req: RefreshRequest): CandlesResult => {
        if (state.session.dataPlan === 'none') throw refuse('no_data_plan', 'Charts need a Kite Connect data plan');
        const candles = generate(state, req.token, req.interval, req.toMs, 30).filter((c) => c.t >= req.fromMs);
        return { candles, complete: true, backfillJobId: '', needsDataPlan: false, offline: false };
      },
      openLive: (token: number, interval: string): string => `live-${token}-${interval}-${++liveSeq}`,
      closeLive: (_id: string): void => undefined,
    },
    instruments: {
      search: (q: string, tags: string[], limit: number): InstrumentDTO[] => {
        const words = q.toLowerCase().split(/\s+/).filter(Boolean);
        if (words.length === 0) return [];
        return state.instruments
          .filter((i) => tags.length === 0 || tags.includes(i.tag))
          .filter((i) => words.every((w) => haystack(i).includes(w)))
          .map((i) => ({ i, rank: i.tradingsymbol.toLowerCase().startsWith(words[0]) ? 0 : 1 }))
          .sort((a, b) => a.rank - b.rank || a.i.tradingsymbol.localeCompare(b.i.tradingsymbol)) // stable: NSE listing before BSE
          .slice(0, limit)
          .map((x) => ({ ...x.i }));
      },
      byTokens: (tokens: number[]): InstrumentDTO[] =>
        tokens.flatMap((t) => state.instruments.filter((i) => i.token === t).map((i) => ({ ...i }))),
      byIdentifier: (id: string): InstrumentDTO => {
        const hit = state.instruments.find((i) => `${i.exchange}:${i.tradingsymbol}` === id);
        if (!hit) throw refuse('not_found', `${id} not found`);
        return { ...hit };
      },
    },
    quotes: {
      watch: (consumer: QuoteConsumer, tokens: number[]): WatchResult => {
        if (!CONSUMERS.includes(consumer)) throw refuse('invalid_consumer', `Unknown consumer ${consumer}`);
        const budget = state.session.dataPlan === 'none' ? 0 : state.liveBudget;
        return {
          live: tokens.slice(0, budget),
          polled: tokens.slice(budget),
          snapshot: tokens.flatMap((t) => (state.quotes[t] ? [copy(state.quotes[t])] : [])),
        };
      },
    },
    watchlists: {
      list: (): WatchlistsState => copy(state.watchlists),
      create: (name: string): WatchlistsState => {
        const t = checkName(name);
        state.watchlists.lists.push({ id: newListId(), name: t, items: [] });
        return copy(state.watchlists);
      },
      rename: (id: string, name: string): WatchlistsState => {
        listOf(id).name = checkName(name, id);
        return copy(state.watchlists);
      },
      delete: (id: string): WatchlistsState => {
        listOf(id);
        state.watchlists.lists = state.watchlists.lists.filter((l) => l.id !== id);
        if (state.watchlists.lists.length === 0) state.watchlists.lists.push({ id: newListId(), name: 'Watchlist 1', items: [] });
        if (!state.watchlists.lists.some((l) => l.id === state.watchlists.activeId)) state.watchlists.activeId = state.watchlists.lists[0].id;
        return copy(state.watchlists);
      },
      setActive: (id: string): WatchlistsState => {
        listOf(id);
        state.watchlists.activeId = id;
        return copy(state.watchlists);
      },
      add: (id: string, token: number, position: number): WatchlistsState => {
        const l = listOf(id);
        if (l.items.some((i) => i.token === token)) throw refuse('already_in_list', `Already in ${l.name}`);
        const inst = state.instruments.find((i) => i.token === token);
        const item: WatchlistItem = inst
          ? watchlistItemOf(inst)
          : { token, exchange: '', symbol: String(token), displayName: '', name: '', isIndex: false, missing: true };
        const at = position < 0 || position > l.items.length ? l.items.length : position;
        l.items.splice(at, 0, item);
        return copy(state.watchlists);
      },
      remove: (id: string, token: number): RemovedItem => {
        const l = listOf(id);
        const position = l.items.findIndex((i) => i.token === token);
        if (position < 0) throw refuse('not_found', `${token} is not in ${l.name}`);
        l.items.splice(position, 1);
        return { listId: id, token, position };
      },
      reorder: (id: string, tokens: number[]): WatchlistsState => {
        const l = listOf(id);
        l.items = tokens.flatMap((t) => l.items.filter((i) => i.token === t));
        return copy(state.watchlists);
      },
      reorderLists: (ids: string[]): WatchlistsState => {
        state.watchlists.lists = ids.flatMap((id) => state.watchlists.lists.filter((l) => l.id === id));
        return copy(state.watchlists);
      },
    },
  };
}
```

In `frontend/src/api/mock/index.ts` (P0-4):
- add `import { createMarketDomains, type MarketMockState } from './market';`
- change `export interface MockState {` to `export interface MockState extends MarketMockState {`
- add `const market = createMarketDomains(state, emit);` on the line before `const backend: MockBackend = {`
- add these members after `system: wrap('system', …),`:

```ts
    chart: wrap('chart', market.chart),
    instruments: wrap('instruments', market.instruments),
    quotes: wrap('quotes', market.quotes),
    watchlists: wrap('watchlists', market.watchlists),
```

In `frontend/src/api/mock/fixtures.ts` (P0-4) add `import { marketSeedState } from './market';` and add `...marketSeedState(),` as the first property of the object literal that `base()` passes to `deepCopy`. Every P0-4 seed (`demoSeed`, `personalSeed`, `freshSeed`, …) builds on `base()`, so all of them get one empty "Watchlist 1", the fixture instrument master and the snapshot quotes. `?seed=personal` therefore gives a ready session with `dataPlan: 'none'` and the same market data.

`frontend/src/api/current.ts`:

```ts
import type { Backend } from './backend';

let current: Backend | null = null;

/** Registered once by <MarketDataBridge> (and directly by tests); the typed market helpers read it. */
export function setBackend(b: Backend): void {
  current = b;
}

export function currentBackend(): Backend {
  if (!current) throw new Error('No backend registered: render <MarketDataBridge /> inside <BackendProvider>');
  return current;
}
```

`frontend/src/api/call.ts`:

```ts
import { parseBackendError } from './errors';

export class BindingError extends Error {
  constructor(readonly code: string, message: string, readonly stage: string = '') {
    super(message);
    this.name = 'BindingError';
  }
}

/** Awaits a Backend promise; a rejection becomes a BindingError carrying P0-3's {code, stage, reason}. */
export async function call<T>(p: Promise<T>): Promise<T> {
  try {
    return await p;
  } catch (e) {
    const { code, stage, reason } = parseBackendError(e);
    throw new BindingError(code, reason, stage);
  }
}
```

`frontend/src/api/instruments.ts`:

```ts
import type { InstrumentDTO } from './backend';
import type { Instrument } from '../instruments/types';
import { EXCHANGE_TAGS, isInScope, type ExchangeTag } from '../instruments/naming';
import { BindingError, call } from './call';
import { currentBackend } from './current';

export function toInstrument(d: InstrumentDTO): Instrument {
  return {
    token: d.token, exchange: d.exchange, segment: d.segment, tag: d.tag, tradingsymbol: d.tradingsymbol, name: d.name,
    displayName: d.displayName, instrumentType: d.instrumentType, expiryMs: d.expiryMs, tickSizePaise: d.tickSizePaise,
    lotSize: d.lotSize, isIndex: d.isIndex, isFuture: d.isFuture, underlying: d.underlying,
  };
}

const inScope = (list: InstrumentDTO[]): Instrument[] => list.map(toInstrument).filter(isInScope);

/** Go returns [] for an empty query, so the round trip is skipped; the UI shows "Type to search". */
export async function searchInstruments(
  query: string, tags: readonly ExchangeTag[] = EXCHANGE_TAGS, limit = 50,
): Promise<Instrument[]> {
  const q = query.trim();
  if (q === '') return [];
  return inScope(await call(currentBackend().instruments.search(q, [...tags], limit)));
}

export async function instrumentsByTokens(tokens: readonly number[]): Promise<Instrument[]> {
  if (tokens.length === 0) return [];
  return inScope(await call(currentBackend().instruments.byTokens([...tokens])));
}

export async function instrumentByIdentifier(id: string): Promise<Instrument> {
  const i = toInstrument(await call(currentBackend().instruments.byIdentifier(id)));
  if (!isInScope(i)) throw new BindingError('out_of_scope', `${id} is not supported`);
  return i;
}
```

`frontend/src/api/chart.ts`:

```ts
import type { BackfillProgress, CandleDTO, CandleEventDTO, CandlesResult } from './backend';
import type { Bar } from '../chart/types';
import type { KiteInterval } from '../chart/timeframes';
import { call } from './call';
import { currentBackend } from './current';

export type { BackfillProgress };
export interface CandlesPage { bars: Bar[]; complete: boolean; backfillJobId: string; needsDataPlan: boolean; offline: boolean }
export interface CandleEvent { token: number; interval: string; bar: Bar; closed: boolean }

export const toBar = (c: CandleDTO): Bar => ({ time: c.t, open: c.o, high: c.h, low: c.l, close: c.c, volume: c.v });
const toPage = (r: CandlesResult): CandlesPage => ({
  bars: (r.candles ?? []).map(toBar), complete: r.complete, backfillJobId: r.backfillJobId ?? '',
  needsDataPlan: r.needsDataPlan, offline: r.offline,
});
const toEvent = (e: CandleEventDTO, closed: boolean): CandleEvent => ({ token: e.token, interval: e.interval, bar: toBar(e.candle), closed });

/** `tradingDays` trading days of candles ending at `toMs` (0 = now); Go resolves fromMs from the calendar. */
export async function fetchHistory(token: number, interval: KiteInterval, toMs: number, tradingDays: number): Promise<CandlesPage> {
  return toPage(await call(currentBackend().chart.candles({ token, interval, fromMs: 0, toMs, tradingDays })));
}

export async function refreshRange(token: number, interval: KiteInterval, fromMs: number, toMs: number): Promise<CandlesPage> {
  return toPage(await call(currentBackend().chart.refresh({ token, interval, fromMs, toMs })));
}

export const openLive = (token: number, interval: KiteInterval): Promise<string> => call(currentBackend().chart.openLive(token, interval));
export const closeLive = (id: string): Promise<void> => call(currentBackend().chart.closeLive(id));

/** candle.forming arrives as a coalesced array, candle.closed as one event; both reach `cb` one candle at a time. */
export function onCandle(cb: (e: CandleEvent) => void): () => void {
  const b = currentBackend();
  const offForming = b.on('candle.forming', (batch) => { for (const e of batch) cb(toEvent(e, false)); });
  const offClosed = b.on('candle.closed', (e) => cb(toEvent(e, true)));
  return () => { offForming(); offClosed(); };
}

export function onBackfillProgress(cb: (progress: BackfillProgress[]) => void): () => void {
  return currentBackend().on('backfill.progress', cb);
}
```

`frontend/src/api/quotes.ts`:

```ts
import type { QuoteConsumer, TickDTO, WatchResult } from './backend';
import { call } from './call';
import { currentBackend } from './current';

export type { QuoteConsumer, WatchResult };
export const QUOTE_CONSUMERS: readonly QuoteConsumer[] = ['chart', 'watchlist', 'widget'];

export async function watchQuotes(consumer: QuoteConsumer, tokens: readonly number[]): Promise<WatchResult> {
  const r = await call(currentBackend().quotes.watch(consumer, [...tokens]));
  return { live: [...(r.live ?? [])], polled: [...(r.polled ?? [])], snapshot: [...(r.snapshot ?? [])] };
}

/** Raw tick.batch access. P0-4's event bridge already feeds useTicksStore; use this only outside that store. */
export function onTickBatch(cb: (ticks: TickDTO[]) => void): () => void {
  return currentBackend().on('tick.batch', cb);
}
```

`frontend/src/api/watchlists.ts`:

```ts
import type { RemovedItem, WatchlistsState } from './backend';
import { call } from './call';
import { currentBackend } from './current';

const w = () => currentBackend().watchlists;

/** Every mutation resolves to Go's new WatchlistsState (Remove resolves to the RemovedItem used by Undo). */
export const watchlistsApi = {
  list: (): Promise<WatchlistsState> => call(w().list()),
  create: (name: string): Promise<WatchlistsState> => call(w().create(name)),
  rename: (id: string, name: string): Promise<WatchlistsState> => call(w().rename(id, name)),
  delete: (id: string): Promise<WatchlistsState> => call(w().delete(id)),
  setActive: (id: string): Promise<WatchlistsState> => call(w().setActive(id)),
  /** position −1 appends. */
  add: (id: string, token: number, position = -1): Promise<WatchlistsState> => call(w().add(id, token, position)),
  remove: (id: string, token: number): Promise<RemovedItem> => call(w().remove(id, token)),
  reorder: (id: string, tokens: readonly number[]): Promise<WatchlistsState> => call(w().reorder(id, [...tokens])),
  reorderLists: (ids: readonly string[]): Promise<WatchlistsState> => call(w().reorderLists([...ids])),
};
```

- [ ] **Step 5: Run the tests to check they pass, then typecheck**

Run: `cd frontend && npx vitest run src/api && npm run typecheck`
Expected: PASS (the 11 market tests, the 1 Wails market test and P0-4's existing `src/api` tests). Typecheck exits 0. A typecheck error naming a `service.*` field means P0-3's generated model differs from the canonical DTO in Consumes: fix P0-3's JSON tag, not this mapping.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api frontend/src/chart/types.ts
git commit -m "$(cat <<'EOF'
feat(frontend): chart/instruments/quotes/watchlists Backend domains (Wails + mock) and typed helpers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---