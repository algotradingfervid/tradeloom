import { useCallback, useEffect } from 'react';
import { create } from 'zustand';
import type { Backend, BookSnapshot, HoldingsSnapshot, KiteOrder, PositionsSnapshot, RefreshScope } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import type { ListStatus } from '../components/ListBody';
import { ordersCopy, portfolioCopy } from '../i18n/copy';
import { isTerminalStatus, orderStatusLabel } from '../portfolio/calc';
import { registerReset } from './reset';
import { useSessionStore } from './session';
import { pushToast } from './toasts';

interface PortfolioData { holdings: HoldingsSnapshot; positions: PositionsSnapshot; orders: BookSnapshot }
export type PortfolioKind = keyof PortfolioData;

export interface Resource<T> { status: 'idle' | 'ready' | 'error'; data: T; error: string | null; loading: boolean }

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

const idle = <T,>(data: T): Resource<T> => ({ status: 'idle', data, error: null, loading: false });
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
      return { [kind]: { status: 'ready', data, error: null, loading: false } } as Partial<PortfolioStore>;
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
  for (const k of Object.keys(backgroundFailing) as PortfolioKind[]) backgroundFailing[k] = false;
  usePortfolioStore.setState(initial());
});

const FETCH: { [K in PortfolioKind]: (b: Backend) => Promise<PortfolioData[K]> } = {
  holdings: (b) => b.portfolio.holdings(),
  positions: (b) => b.portfolio.positions(),
  orders: (b) => b.portfolio.orderBook(),
};
const SCOPE: Record<PortfolioKind, RefreshScope> = { holdings: 'holdings', positions: 'positions', orders: 'orders' };
const latestRequest: Record<PortfolioKind, number> = { holdings: 0, positions: 0, orders: 0 };
/** A background re-read is failing for this kind: one notice per failure streak, not one per poll. */
const backgroundFailing: Record<PortfolioKind, boolean> = { holdings: false, positions: false, orders: false };

/**
 * Reads the Go cache (no Kite call); a slower, older response never overwrites a newer one. `background` (polling):
 * no busy indicator, and a failure while data is shown keeps that data and raises a non-blocking notice instead of
 * the error state (Task 10 M8).
 */
export async function loadPortfolio<K extends PortfolioKind>(kind: K, backend: Backend, opts: { background?: boolean } = {}): Promise<void> {
  const mine = ++latestRequest[kind];
  const g = guard();
  const store = usePortfolioStore.getState;
  const shownAsOf = store()[kind].data.asOfMs;
  if (!opts.background) store().patch(kind, { loading: true });
  // Dropped results: after a reset the store is already initial; after a user switch only the spinner is cleared.
  const dropped = () => {
    if (g.epoch === epoch) store().patch(kind, { loading: false });
  };
  try {
    const data = await FETCH[kind](backend);
    if (mine !== latestRequest[kind]) return;
    if (!stillCurrent(g)) return dropped();
    store().applySnapshot(kind, data);
    store().patch(kind, { loading: false });
    backgroundFailing[kind] = false;
  } catch (e) {
    if (mine !== latestRequest[kind]) return;
    if (!stillCurrent(g)) return dropped();
    const cur = store()[kind];
    // A "*.changed" snapshot that arrived while this read was in flight is newer than anything the read could
    // return: keep showing it rather than replacing it with the read's error.
    if (cur.status === 'ready' && cur.data.asOfMs > shownAsOf) store().patch(kind, { loading: false });
    else if (opts.background && cur.status === 'ready') {
      store().patch(kind, { loading: false });
      if (!backgroundFailing[kind]) {
        backgroundFailing[kind] = true;
        pushToast({ level: 'warning', message: portfolioCopy.backgroundReadFailed(portfolioCopy.names[kind], parseBackendError(e).reason) });
      }
    } else store().patch(kind, { status: 'error', error: parseBackendError(e).reason, loading: false });
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
  useEffect(() => {
    void reload();
  }, [reload, version]);
  useEffect(() => {
    if (!opts.pollMs) return;
    const id = window.setInterval(() => void loadPortfolio(kind, backend, { background: true }), opts.pollMs);
    return () => window.clearInterval(id);
  }, [kind, backend, opts.pollMs]);
  return { resource, reload, refresh };
}

export function listStatusOf(r: Resource<unknown>, isEmpty: boolean): ListStatus {
  if (r.status === 'error') return 'error';
  if (r.status === 'idle') return 'loading';
  return isEmpty ? 'empty' : 'ready';
}
