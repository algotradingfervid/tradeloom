import { useCallback, useEffect } from 'react';
import { create } from 'zustand';
import type { Backend, BookSnapshot, HoldingsSnapshot, KiteOrder, PositionsSnapshot, RefreshScope } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import type { ListStatus } from '../components/ListBody';
import { ordersCopy } from '../i18n/copy';
import { isTerminalStatus, orderStatusLabel } from '../portfolio/calc';
import { registerReset } from './reset';
import { useSessionStore } from './session';
import { pushToast } from './toasts';

interface PortfolioData { holdings: HoldingsSnapshot; positions: PositionsSnapshot; orders: BookSnapshot }
export type PortfolioKind = keyof PortfolioData;

/**
 * `backgroundError`: why the latest read (background or foreground) failed while data was shown; the data stays,
 * `status` stays 'ready', and views show a lasting inline notice with Retry (Rulings 103, 105). `status: 'error'`
 * (the alert) only happens with no data held. Cleared by any fresh snapshot, by a failure with no data, and on reset.
 * `backgroundErrorSeq` moves on every such failure so an identical repeated failure is announced again.
 */
export interface Resource<T> {
  status: 'idle' | 'ready' | 'error';
  data: T;
  error: string | null;
  loading: boolean;
  backgroundError: string | null;
  backgroundErrorSeq: number;
}

interface PortfolioStore {
  holdings: Resource<HoldingsSnapshot>;
  positions: Resource<PositionsSnapshot>;
  orders: Resource<BookSnapshot>;
  /** Bumping a kind makes mounted views re-read the Go cache. `order.update` does not bump it (P0-3 Ruling 71). */
  versions: Record<PortfolioKind, number>;
  patch<K extends PortfolioKind>(kind: K, p: Partial<Resource<PortfolioData[K]>>): void;
  /** Applies a snapshot unless a newer one (by `asOfMs`) is already shown. */
  applySnapshot<K extends PortfolioKind>(kind: K, data: PortfolioData[K]): void;
  applyOrderUpdate(o: KiteOrder): void;
}

const idle = <T,>(data: T): Resource<T> => ({ status: 'idle', data, error: null, loading: false, backgroundError: null, backgroundErrorSeq: 0 });
const initial = () => ({
  holdings: idle<HoldingsSnapshot>({
    rows: [],
    summary: { empty: true, currentValuePaise: 0, investmentPaise: 0, pnlPaise: 0, pnlBp: 0, dayChangePaise: 0, outOfScopeCount: 0 },
    asOfMs: 0,
    stale: false,
  }),
  positions: idle<PositionsSnapshot>({ net: [], day: [], netTotalPnlPaise: 0, dayTotalPnlPaise: 0, outOfScopeCount: 0, asOfMs: 0, stale: false }),
  orders: idle<BookSnapshot>({ orders: [], counts: { open: 0, executed: 0, rejected: 0, cancelled: 0 }, outOfScopeCount: 0, asOfMs: 0, stale: false }),
  versions: { holdings: 0, positions: 0, orders: 0 },
});

export const usePortfolioStore = create<PortfolioStore>()((set) => ({
  ...initial(),
  patch: (kind, p) => set((s) => ({ [kind]: { ...s[kind], ...p } }) as Partial<PortfolioStore>),
  applySnapshot: (kind, data) =>
    set((s) => {
      const cur = s[kind];
      if (cur.status === 'ready' && cur.data.asOfMs > data.asOfMs) return s;
      return { [kind]: { ...cur, status: 'ready', data, error: null, loading: false, backgroundError: null } } as Partial<PortfolioStore>;
    }),
  // P0-3 Ruling 71 / Task 10 carry-forward: "order.update" is a notification only. Go publishes the new book,
  // positions and holdings on "*.changed"; those events are the truth, so nothing is re-read or patched here.
  applyOrderUpdate: (o) => {
    if (!isTerminalStatus(o.status)) return;
    const status = o.status.toUpperCase();
    // Kite can deliver the same postback more than once (postback + WebSocket); one toast per order and status.
    const key = `${o.order_id}|${status}`;
    if (announced.has(key)) return;
    announced.add(key);
    pushToast({ level: status === 'REJECTED' ? 'warning' : 'info', message: ordersCopy.updated(o.tradingsymbol, orderStatusLabel(o.status)) });
  },
}));

/** order id + status pairs already toasted (Ruling 96 M5); account data, so cleared with the store. */
const announced = new Set<string>();

/**
 * Bumped by every reset. A read or refresh started before a sign-out (or before the session switched to another
 * user) must not write its result into the store afterwards (Ruling 96 I1).
 */
let epoch = 0;
interface ReadGuard { epoch: number; userId: string | undefined }
const guard = (): ReadGuard => ({ epoch, userId: useSessionStore.getState().session?.userId });
const stillCurrent = (g: ReadGuard): boolean => g.epoch === epoch && g.userId === useSessionStore.getState().session?.userId;

// Account data: cleared on sign-out (default onLogout: true, Ruling 83-F M5).
registerReset(() => {
  epoch += 1;
  announced.clear();
  usePortfolioStore.setState(initial());
}, { name: 'portfolio', inspect: () => ({ ...usePortfolioStore.getState(), epoch, announced }) });

const FETCH: { [K in PortfolioKind]: (b: Backend) => Promise<PortfolioData[K]> } = {
  holdings: (b) => b.portfolio.holdings(),
  positions: (b) => b.portfolio.positions(),
  orders: (b) => b.portfolio.orderBook(),
};
const SCOPE: Record<PortfolioKind, RefreshScope> = { holdings: 'holdings', positions: 'positions', orders: 'orders' };
const latestRequest: Record<PortfolioKind, number> = { holdings: 0, positions: 0, orders: 0 };

/**
 * Reads the Go cache (no Kite call); a slower, older response never overwrites a newer one. `background` (polling and
 * the notice's Retry): never touches `loading` (no busy indicator) and is skipped while a foreground read or Refresh
 * is in flight (that read is fresher and owns the spinner). Any failure while data is shown keeps it and records
 * `backgroundError`; only a failure with no data held is the error state (Task 10 M8, Rulings 103, 105).
 */
export async function loadPortfolio<K extends PortfolioKind>(kind: K, backend: Backend, opts: { background?: boolean } = {}): Promise<void> {
  const g = guard();
  const store = usePortfolioStore.getState;
  const shownAsOf = store()[kind].data.asOfMs;
  if (opts.background && store()[kind].loading) return;
  const mine = ++latestRequest[kind];
  if (!opts.background) store().patch(kind, { loading: true });
  // Dropped results: after a reset the store is already initial; after a user switch only the spinner is cleared.
  const dropped = () => {
    if (g.epoch === epoch && !opts.background) store().patch(kind, { loading: false });
  };
  try {
    const data = await FETCH[kind](backend);
    if (mine !== latestRequest[kind]) return;
    if (!stillCurrent(g)) return dropped();
    const wasLoading = store()[kind].loading;
    store().applySnapshot(kind, data);
    // A successful read clears the notice even when a newer snapshot was already shown; a background read leaves
    // the busy state (e.g. a Refresh waiting on Kite) as it was.
    store().patch(kind, { loading: opts.background ? wasLoading : false, backgroundError: null });
  } catch (e) {
    if (mine !== latestRequest[kind]) return;
    if (!stillCurrent(g)) return dropped();
    const cur = store()[kind];
    // A "*.changed" snapshot that arrived while this read was in flight is newer than anything the read could
    // return: keep showing it rather than replacing it with the read's error.
    const done = opts.background ? {} : { loading: false };
    const reason = parseBackendError(e).reason;
    if (cur.status === 'ready' && cur.data.asOfMs > shownAsOf) store().patch(kind, done);
    else if (cur.status === 'ready') {
      store().patch(kind, { ...done, backgroundError: reason, backgroundErrorSeq: cur.backgroundErrorSeq + 1 });
    } else store().patch(kind, { ...done, status: 'error', error: reason, backgroundError: null });
  }
}

/** The Refresh button: Go re-fetches from Kite (`Portfolio.Refresh(scope)`), then the cache is re-read. */
export async function refreshPortfolio(kind: PortfolioKind, backend: Backend): Promise<void> {
  const g = guard();
  usePortfolioStore.getState().patch(kind, { loading: true });
  try {
    await backend.portfolio.refresh(SCOPE[kind]);
  } catch (e) {
    if (!stillCurrent(g)) {
      if (g.epoch === epoch) usePortfolioStore.getState().patch(kind, { loading: false });
      return;
    }
    usePortfolioStore.getState().patch(kind, { loading: false });
    pushToast({ level: 'error', message: parseBackendError(e).reason });
    return;
  }
  if (!stillCurrent(g)) {
    if (g.epoch === epoch) usePortfolioStore.getState().patch(kind, { loading: false });
    return;
  }
  await loadPortfolio(kind, backend);
}

export function usePortfolioResource<K extends PortfolioKind>(kind: K, opts: { pollMs?: number } = {}) {
  const backend = useBackend();
  const resource = usePortfolioStore((s) => s[kind]) as Resource<PortfolioData[K]>;
  const version = usePortfolioStore((s) => s.versions[kind]);
  const reload = useCallback(() => loadPortfolio(kind, backend), [kind, backend]);
  const refresh = useCallback(() => refreshPortfolio(kind, backend), [kind, backend]);
  /** The background-error notice's Retry: a failure keeps the shown data and updates the notice (Ruling 103). */
  const retryInBackground = useCallback(() => loadPortfolio(kind, backend, { background: true }), [kind, backend]);
  useEffect(() => {
    void reload();
  }, [reload, version]);
  useEffect(() => {
    if (!opts.pollMs) return;
    const id = window.setInterval(() => void loadPortfolio(kind, backend, { background: true }), opts.pollMs);
    return () => window.clearInterval(id);
  }, [kind, backend, opts.pollMs]);
  return { resource, reload, refresh, retryInBackground };
}

export function listStatusOf(r: Resource<unknown>, isEmpty: boolean): ListStatus {
  if (r.status === 'error') return 'error';
  if (r.status === 'idle') return 'loading';
  return isEmpty ? 'empty' : 'ready';
}
