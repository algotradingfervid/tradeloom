import type {
  BackfillProgress, CandleDTO, CandlesRequest, CandlesResult, EventMap, EventName, InstrumentDTO, QuoteConsumer,
  RefreshRequest, RemovedItem, TickDTO, WatchlistItem, WatchlistsState, WatchResult,
} from '../backend';
import { ALL_IN_SCOPE, NEWCO } from '../../instruments/fixtures';
import type { MockState } from './index';
import { QUOTE_CONSUMERS } from '../quotes';

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
const DAY = 86_400_000;
/** Kite's per-request historical limits, as Go's maxRefreshSpan (internal/service/chart.go). */
const MAX_REFRESH_SPAN: Record<string, number> = {
  minute: 60 * DAY, '3minute': 100 * DAY, '5minute': 100 * DAY, '10minute': 100 * DAY, '15minute': 100 * DAY,
  '30minute': 100 * DAY, '60minute': 400 * DAY, day: 2000 * DAY,
};

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
  /** Ids returned by Chart.OpenLive and not yet closed (tests assert no live subscription leaks). */
  openLiveIds: string[];
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
    openLiveIds: [],
  };
}

const copy = <T,>(v: T): T => JSON.parse(JSON.stringify(v)) as T;
/** A Go refusal: the JSON {code, stage, reason} that P0-4's parseBackendError reads. Stages are Go's. */
const refuse = (stage: string, code: string, reason: string): Error => new Error(JSON.stringify({ code, stage, reason }));
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

/** Candles of the same grid with start in [fromMs, toMs): Go's readRange of a refreshed window. */
function generateRange(state: MarketMockState, token: number, interval: string, fromMs: number, toMs: number): CandleDTO[] {
  const step = STEP[interval];
  const perDay = interval === 'day' ? 1 : Math.floor((375 * MIN) / step); // as generate(): `days` buys perDay steps
  const days = Math.ceil((toMs - fromMs) / step / perDay) + 1;
  return generate(state, token, interval, toMs, days).filter((c) => c.t >= fromMs && c.t < toMs);
}

export function createMarketDomains(state: MockState, emit: Emit) {
  let liveSeq = 0;
  let jobSeq = 0;
  /** Per consumer: the wanted tokens and the subset Go put on the 5 s poll (internal/service/quotes.go). */
  const watched = new Map<QuoteConsumer, { wanted: Set<number>; polled: Set<number> }>();
  const haystack = (i: MockInstrument) => `${i.tradingsymbol} ${i.name} ${i.displayName} ${ALIASES[i.token] ?? ''}`.toLowerCase();
  // Go fills alsoOnNse on Search results only (a BSE row whose company is also listed on NSE); ByTokens and
  // ByIdentifier always report false (internal/service/chart.go InstrumentDTO).
  const dto = (i: MockInstrument, alsoOnNse = false): InstrumentDTO => ({ ...i, alsoOnNse });
  const listedOnNse = (i: MockInstrument) =>
    i.exchange === 'BSE' && !i.isIndex && state.instruments.some((o) => o.exchange === 'NSE' && o.tradingsymbol === i.tradingsymbol);
  const instrument = (token: number) => state.instruments.find((i) => i.token === token);
  const checkInterval = (interval: string) => {
    if (!(interval in STEP)) throw refuse('chart', 'invalid_interval', `Unsupported timeframe ${interval}.`);
  };

  // --- watchlists: internal/service/watchlists.go
  const WL = 'watchlists';
  const notFound = () => refuse(WL, 'not_found', 'That watchlist no longer exists.');
  const staleOrder = () => refuse(WL, 'stale_order', 'The list changed; reload and try again.');
  const indexOf = (id: string) => state.watchlists.lists.findIndex((x) => x.id === id);
  const listOf = (id: string) => {
    const l = state.watchlists.lists[indexOf(id)];
    if (!l) throw notFound();
    return l;
  };
  const checkName = (name: string, selfId = ''): string => {
    const t = name.trim();
    if (t === '') throw refuse(WL, 'invalid_name', 'Enter a watchlist name.');
    if ([...t].length > 40) throw refuse(WL, 'invalid_name', 'Watchlist names can be up to 40 characters.');
    if (state.watchlists.lists.some((l) => l.id !== selfId && l.name.toLowerCase() === t.toLowerCase())) {
      throw refuse(WL, 'duplicate_name', 'A watchlist with this name exists');
    }
    return t;
  };
  const nextDefaultName = (): string => {
    const used = new Set(state.watchlists.lists.map((l) => l.name.toLowerCase()));
    let n = 1;
    while (used.has(`watchlist ${n}`)) n += 1;
    return `Watchlist ${n}`;
  };
  const newListId = (): string => {
    let n = state.watchlists.lists.length;
    do { n += 1; } while (state.watchlists.lists.some((l) => l.id === `wl-${n}`));
    return `wl-${n}`;
  };
  /** Go builds every item from the instrument master on each read; a token no longer in it is `missing`. */
  const view = (): WatchlistsState => ({
    activeId: state.watchlists.activeId,
    lists: state.watchlists.lists.map((l) => ({
      id: l.id, name: l.name,
      items: l.items.map((it) => {
        const inst = instrument(it.token);
        return inst
          ? watchlistItemOf(inst)
          : { token: it.token, exchange: '', symbol: '', displayName: '', name: '', isIndex: false, missing: true };
      }),
    })),
  });
  /** Go's mutate(): apply, then publish watchlists.changed with the new state (refusals publish nothing). */
  const mutate = (fn: () => void): WatchlistsState => {
    fn();
    const next = view();
    emit('watchlists.changed', next);
    return copy(next);
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
      /** Go RefreshCandles: interval, range, Kite span, then session checks; returns the whole [from, to) window. */
      refresh: (req: RefreshRequest): CandlesResult => {
        checkInterval(req.interval);
        const to = req.toMs > 0 ? req.toMs : MOCK_MARKET_NOW;
        if (to <= req.fromMs) throw refuse('chart', 'invalid_range', 'The start of the range must be before its end.');
        if (to - req.fromMs > MAX_REFRESH_SPAN[req.interval]) {
          throw refuse('chart', 'invalid_range', `Refresh a smaller range: Kite serves at most ${MAX_REFRESH_SPAN[req.interval] / DAY} days of ${req.interval} candles per request.`);
        }
        if (state.session.state !== 'ready') throw refuse('chart', 'offline', 'Log in to Kite to refresh candles.');
        if (state.session.dataPlan === 'none') throw refuse('chart', 'no_data_plan', 'This API key has no market-data permission; cached candles only.');
        const candles = generateRange(state, req.token, req.interval, req.fromMs, to);
        return { candles, complete: true, backfillJobId: '', needsDataPlan: false, offline: false };
      },
      openLive: (token: number, interval: string): string => {
        checkInterval(interval);
        if (state.session.dataPlan === 'none') {
          throw refuse('chart', 'no_data_plan', 'This API key has no market-data permission; live candles are unavailable.');
        }
        if (state.instruments.length > 0 && !instrument(token)) {
          throw refuse('chart', 'not_found', 'This instrument is not in the instrument list.');
        }
        const id = `live-${token}-${interval}-${++liveSeq}`;
        state.openLiveIds.push(id);
        return id;
      },
      closeLive: (id: string): void => {
        if (!state.openLiveIds.includes(id)) throw refuse('chart', 'not_found', `Unknown live subscription ${id}.`);
        state.openLiveIds = state.openLiveIds.filter((x) => x !== id);
      },
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
        if (!hit) throw refuse('instruments', 'not_found', `No instrument ${id} in the instrument list.`);
        return dto(hit);
      },
    },
    quotes: {
      /**
       * Go WatchQuotes: replaces the consumer's set (tokens deduped, order kept). Only newly wanted tokens are
       * subscribed; one that cannot be (no data plan, over budget) is polled, and stays polled while it is wanted.
       * Differences: the budget is per token here (Go refuses a whole over-budget batch), and the 30 s linger is
       * not modelled (a dropped token is released at once, so re-adding it subscribes it again, as in Go).
       */
      watch: (consumer: QuoteConsumer, tokens: number[]): WatchResult => {
        if (!QUOTE_CONSUMERS.includes(consumer)) throw refuse('quotes', 'invalid_consumer', `Unknown quote consumer ${consumer}.`);
        const ordered = [...new Set(tokens)];
        const prev = watched.get(consumer) ?? { wanted: new Set<number>(), polled: new Set<number>() };
        const polled = new Set([...prev.polled].filter((t) => ordered.includes(t)));
        const liveElsewhere = new Set<number>();
        for (const [c, w] of watched) {
          if (c !== consumer) for (const t of w.wanted) if (!w.polled.has(t)) liveElsewhere.add(t);
        }
        const live = new Set([...liveElsewhere, ...ordered.filter((t) => prev.wanted.has(t) && !polled.has(t))]);
        for (const t of ordered) {
          if (prev.wanted.has(t) || live.has(t)) continue;
          if (state.session.dataPlan !== 'none' && live.size < state.liveBudget) live.add(t);
          else polled.add(t);
        }
        watched.set(consumer, { wanted: new Set(ordered), polled });
        return {
          live: ordered.filter((t) => !polled.has(t)),
          polled: ordered.filter((t) => polled.has(t)),
          snapshot: ordered.flatMap((t) => (state.quotes[t] ? [copy(state.quotes[t])] : [])),
        };
      },
    },
    watchlists: {
      list: (): WatchlistsState => copy(view()),
      create: (name: string): WatchlistsState =>
        mutate(() => {
          const t = checkName(name.trim() === '' ? nextDefaultName() : name);
          const id = newListId();
          state.watchlists.lists.push({ id, name: t, items: [] });
          state.watchlists.activeId = id;
        }),
      rename: (id: string, name: string): WatchlistsState =>
        mutate(() => {
          const l = listOf(id);
          l.name = checkName(name, id);
        }),
      delete: (id: string): WatchlistsState =>
        mutate(() => {
          const idx = indexOf(id);
          if (idx < 0) throw notFound();
          const remaining = state.watchlists.lists.filter((l) => l.id !== id);
          if (remaining.length === 0) {
            const nid = newListId();
            state.watchlists.lists = [{ id: nid, name: 'Watchlist 1', items: [] }];
            state.watchlists.activeId = nid;
            return;
          }
          state.watchlists.lists = remaining;
          if (state.watchlists.activeId === id) state.watchlists.activeId = remaining[Math.min(idx, remaining.length - 1)].id;
        }),
      setActive: (id: string): WatchlistsState =>
        mutate(() => {
          listOf(id);
          state.watchlists.activeId = id;
        }),
      add: (id: string, token: number, position: number): WatchlistsState =>
        mutate(() => {
          const l = listOf(id);
          const inst = instrument(token);
          if (!inst) throw refuse(WL, 'unknown_instrument', 'That instrument is not in the instrument list.');
          if (l.items.some((i) => i.token === token)) throw refuse(WL, 'already_in_list', `Already in ${l.name}`);
          const at = position < 0 || position > l.items.length ? l.items.length : position;
          l.items.splice(at, 0, watchlistItemOf(inst));
        }),
      remove: (id: string, token: number): RemovedItem => {
        let position = -1;
        mutate(() => {
          const l = listOf(id);
          position = l.items.findIndex((i) => i.token === token);
          if (position < 0) throw refuse(WL, 'not_found', 'That instrument is not in the list.');
          l.items.splice(position, 1);
        });
        return { listId: id, token, position };
      },
      reorder: (id: string, tokens: number[]): WatchlistsState =>
        mutate(() => {
          const l = listOf(id);
          const cur = new Set(l.items.map((i) => i.token));
          const seen = new Set<number>();
          for (const t of tokens) {
            if (!cur.has(t) || seen.has(t)) throw staleOrder();
            seen.add(t);
          }
          if (seen.size !== cur.size) throw staleOrder();
          l.items = tokens.map((t) => l.items.find((i) => i.token === t)!);
        }),
      reorderLists: (ids: string[]): WatchlistsState =>
        mutate(() => {
          const seen = new Set<string>();
          for (const id of ids) {
            if (indexOf(id) < 0 || seen.has(id)) throw refuse(WL, 'stale_order', 'The lists changed; reload and try again.');
            seen.add(id);
          }
          if (seen.size !== state.watchlists.lists.length) throw refuse(WL, 'stale_order', 'The lists changed; reload and try again.');
          state.watchlists.lists = ids.map((id) => state.watchlists.lists[indexOf(id)]);
        }),
    },
  };
}
