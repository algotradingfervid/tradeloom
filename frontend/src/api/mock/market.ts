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

/** An instrument-master row as the mock stores it: `alsoOnNse` is derived per Search result, as Go does. */
export type MockInstrument = Omit<InstrumentDTO, 'alsoOnNse'>;

export interface MarketMockState {
  instruments: MockInstrument[];
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

export function watchlistItemOf(i: MockInstrument): WatchlistItem {
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
  const haystack = (i: MockInstrument) => `${i.tradingsymbol} ${i.name} ${i.displayName} ${ALIASES[i.token] ?? ''}`.toLowerCase();
  // Go fills alsoOnNse on Search results only (a BSE row whose company is also listed on NSE); ByTokens and
  // ByIdentifier always report false (internal/service/chart.go InstrumentDTO).
  const dto = (i: MockInstrument, alsoOnNse = false): InstrumentDTO => ({ ...i, alsoOnNse });
  const listedOnNse = (i: MockInstrument) =>
    i.exchange === 'BSE' && !i.isIndex && state.instruments.some((o) => o.exchange === 'NSE' && o.tradingsymbol === i.tradingsymbol);
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
          const done: BackfillProgress = { jobId, token: req.token, interval: req.interval, done: 1, failed: 0, total: 1, etaMs: 0, state: 'done' };
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
          .map((x) => dto(x.i, listedOnNse(x.i)));
      },
      byTokens: (tokens: number[]): InstrumentDTO[] =>
        tokens.flatMap((t) => state.instruments.filter((i) => i.token === t).map((i) => dto(i))),
      byIdentifier: (id: string): InstrumentDTO => {
        const hit = state.instruments.find((i) => `${i.exchange}:${i.tradingsymbol}` === id);
        if (!hit) throw refuse('not_found', `${id} not found`);
        return dto(hit);
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
