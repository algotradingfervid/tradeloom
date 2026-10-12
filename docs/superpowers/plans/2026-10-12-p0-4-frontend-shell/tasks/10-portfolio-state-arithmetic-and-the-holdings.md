> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 10.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 10: Portfolio state, arithmetic and the Holdings view

**Files:**
- Create: `frontend/src/portfolio/calc.ts`, `frontend/src/store/portfolio.ts`, `frontend/src/components/Values.tsx`, `frontend/src/views/holdings/HoldingsView.tsx`
- Modify: `frontend/src/api/events.ts` (`order.update`, `orders.changed`, `positions.changed`, `holdings.changed`), `frontend/src/app/routes.tsx` (`holdings`), `frontend/src/components/list.css`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/portfolio/calc.test.ts`, `frontend/src/views/holdings/HoldingsView.test.tsx`

**Interfaces:**
- Consumes: Task 2 `Backend.portfolio.{orderBook,positions,holdings,refresh}`, `HoldingsSnapshot`/`HoldingRow`/`HoldingsSummary`, `PositionsSnapshot`/`PositionRow`, `BookSnapshot`/`OrderRow`, `KiteOrder`, `TickDTO`, `useTicksStore`, `registerReset`, `parseBackendError`; Task 4 `pushToast`; Task 5 `ListPage`, `ListBody`, `ListStatus`, `DataTable`, `Column`, `filterRows`; Task 1 formatters; canonical fact A4 (Go snapshots, `*.changed` events, live LTP overlay).
- Produces:
  - `calc.ts` (Go already computed every row in paise; the UI converts to rupees and overlays the newest tick): `rupees(paise)`, `toPaise(rupees)`, `interface LiveHolding { row: HoldingRow; ltp; invested; current; pnl; netChangePct: number | null; dayChange; dayChangePct: number | null }` (rupees), `holdingRow(row, liveLtp?)`, `interface HoldingsTotals { empty; currentValue; investment; pnl; pnlPct: number | null; dayChange }`, `summarizeHoldings(summary, rows)`, `positionPnl(row, liveLtp?)`, `positionNetChangePct(row, ltp)`, `orderStatusLabel(status)`, `isTerminalStatus(status)`.
  - `store/portfolio.ts`: `type PortfolioKind = 'holdings' | 'positions' | 'orders'`, `interface Resource<T> { status: 'idle' | 'ready' | 'error'; data: T; error: string | null; loading: boolean }`, `usePortfolioStore` (`holdings: Resource<HoldingsSnapshot>`, `positions: Resource<PositionsSnapshot>`, `orders: Resource<BookSnapshot>`, `versions`, `patch`, `applySnapshot(kind, data)`, `applyOrderUpdate(o: KiteOrder)`), `loadPortfolio(kind, backend)` (cache read), `refreshPortfolio(kind, backend)` (`Portfolio.Refresh(scope)` then a cache read), `usePortfolioResource(kind, { pollMs? }) → { resource, reload, refresh }`, `listStatusOf(resource, isEmpty): ListStatus`.
  - `Values.tsx`: `Signed({ value: number | null; kind?: 'money' | 'percent' | 'number' })`, `InstrumentName({ name: string; exchange: string })`.
  - `HoldingsView()` at `/holdings`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/portfolio/calc.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { holdingA, holdingB, holdingsOf, positionClosed, positionOpen } from '../api/mock/fixtures';
import { holdingRow, isTerminalStatus, orderStatusLabel, positionNetChangePct, positionPnl, rupees, summarizeHoldings } from './calc';

describe('portfolio arithmetic (paise from Go → rupees in the view)', () => {
  it('converts paise to rupees', () => {
    expect(rupees(151_025)).toBe(1510.25);
  });
  it('FR-ORD-003 AC2: 10 @ 100 with LTP 110 → invested 1,000, current 1,100, P&L 100, net change +10 %', () => {
    expect(holdingRow(holdingA)).toMatchObject({ ltp: 110, invested: 1000, current: 1100, pnl: 100, netChangePct: 10, dayChange: 50 });
  });
  it('FR-HOME-002 AC1: totals start from the Go summary (quantity = settled + T1, summed by Go)', () => {
    const snap = holdingsOf([holdingA, holdingB]);
    const s = summarizeHoldings(snap.summary, snap.rows.map((r) => holdingRow(r)));
    expect(s).toEqual({ empty: false, currentValue: 2050, investment: 2000, pnl: 50, pnlPct: 2.5, dayChange: 0 });
  });
  it('FR-HOME-002 AC2: a live LTP replaces the snapshot price in rows and totals', () => {
    const snap = holdingsOf([holdingA, holdingB]);
    expect(holdingRow(holdingA, 120)).toMatchObject({ ltp: 120, current: 1200, pnl: 200, dayChange: 150 });
    expect(summarizeHoldings(snap.summary, [holdingRow(holdingA, 120), holdingRow(holdingB)]).currentValue).toBe(2150);
  });
  it('no investment → no percentage (never divide by zero)', () => {
    const empty = holdingsOf([]);
    expect(summarizeHoldings(empty.summary, [])).toMatchObject({ empty: true, pnlPct: null });
  });
  it('FR-ORD-002 AC1: position P&L comes from Go; a live LTP adds qty × (LTP − snapshot LTP)', () => {
    expect(positionPnl(positionOpen)).toBe(120);
    expect(positionPnl(positionClosed)).toBe(-45.5);
    expect(positionPnl(positionOpen, 113)).toBe(130);
    expect(positionNetChangePct(positionOpen, 112)).toBeCloseTo(12, 10);
    expect(positionNetChangePct(positionClosed, 101)).toBeNull();
  });
  it('labels Kite order statuses and knows which are final', () => {
    expect(orderStatusLabel('TRIGGER PENDING')).toBe('Trigger pending');
    expect(isTerminalStatus('COMPLETE')).toBe(true);
    expect(isTerminalStatus('OPEN')).toBe(false);
  });
});
```

Create `frontend/src/views/holdings/HoldingsView.test.tsx`:

```tsx
import { act, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed, emptySeed, holdingA, holdingB, holdingsOf, kiteOrder, tickFixture } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { renderApp } from '../../test/render';

const cells = (row: HTMLElement) => within(row).getAllByRole('cell').slice(1).map((c) => c.textContent);

describe('FR-ORD-003 Holdings view', () => {
  it('AC2: shows invested, current value, P&L and net change per row; T1 quantity is included (Go sums it)', async () => {
    renderApp({ route: '/holdings' });
    expect(await screen.findByRole('heading', { name: 'Holdings' })).toBeInTheDocument();
    expect(cells(await screen.findByRole('row', { name: 'RELIANCE' }))).toEqual(
      ['10', '100.00', '110.00', '₹1,000.00', '₹1,100.00', '+₹100.00', '+10.00%', '+₹50.00'],
    );
    expect(cells(screen.getByRole('row', { name: 'TCS' }))[0]).toBe('5');
  });
  it('shows a totals row', async () => {
    renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    const totals = screen.getAllByRole('rowgroup')[2];
    expect(totals).toHaveTextContent('₹2,000.00');
    expect(totals).toHaveTextContent('₹2,050.00');
    expect(totals).toHaveTextContent('+₹50.00');
    expect(totals).toHaveTextContent('+2.50%');
  });
  it('updates LTP-driven values live from ticks', async () => {
    const { backend } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    act(() => backend.emit('tick.batch', [tickFixture(holdingA.token, 120, 105)]));
    expect(cells(screen.getByRole('row', { name: 'RELIANCE' }))[4]).toBe('₹1,200.00');
  });
  it('applies a holdings.changed snapshot pushed by Go without a re-read', async () => {
    const { backend } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    const reads = backend.callsTo('portfolio.holdings').length;
    act(() => backend.emit('holdings.changed', holdingsOf([holdingB], Date.now())));
    expect(screen.queryByRole('row', { name: 'RELIANCE' })).toBeNull();
    expect(screen.getByRole('row', { name: 'TCS' })).toBeInTheDocument();
    expect(backend.callsTo('portfolio.holdings')).toHaveLength(reads);
  });
  it('Refresh asks Go to re-fetch from Kite (Portfolio.Refresh("holdings"))', async () => {
    const { backend, user } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    await user.click(screen.getByRole('button', { name: 'Refresh holdings' }));
    expect(backend.callsTo('portfolio.refresh').map((c) => c.args)).toEqual([['holdings']]);
  });
  it('AC1 + FR-SHELL-014 AC1: no holdings → "No holdings" with a single call to action', async () => {
    renderApp({ route: '/holdings', backend: createMockBackend(emptySeed()) });
    const message = await screen.findByText('No holdings');
    const state = message.closest('.list-state') as HTMLElement;
    expect(within(state).getAllByRole('button')).toHaveLength(1);
    expect(state).not.toHaveTextContent(/positions/i);
  });
  it('FR-SHELL-014 AC3: a failed load shows the error with Retry, and Retry recovers', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('portfolio.holdings', 'Network unavailable');
    const { user } = renderApp({ route: '/holdings', backend });
    expect(await screen.findByRole('alert')).toHaveTextContent('Network unavailable');
    expect(screen.queryByText('No holdings')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Retry' }));
    expect(await screen.findByRole('row', { name: 'RELIANCE' })).toBeInTheDocument();
  });
  it('searches by symbol or exchange', async () => {
    const { user } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    await user.type(screen.getByRole('searchbox', { name: 'Search holdings' }), 'tcs');
    expect(screen.queryByRole('row', { name: 'RELIANCE' })).toBeNull();
    expect(screen.getByRole('row', { name: 'TCS' })).toBeInTheDocument();
  });
  it('re-reads holdings when a CNC order completes (FR-HOME-002 refresh rule) and announces the fill', async () => {
    const { backend } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    const before = backend.callsTo('portfolio.holdings').length;
    act(() => backend.emit('order.update', kiteOrder({ status: 'COMPLETE', filled_quantity: 10, average_price: 111 })));
    await act(async () => {});
    expect(backend.callsTo('portfolio.holdings').length).toBe(before + 1);
    expect(screen.getByText('Order RELIANCE: Complete')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/portfolio src/views/holdings`
Expected: FAIL — `Failed to resolve import "./calc"`; the view tests time out (no `/holdings` route).

- [ ] **Step 3: Implement the arithmetic, the store, value components and the view**

Create `frontend/src/portfolio/calc.ts`:

```ts
import type { HoldingRow, HoldingsSummary, PositionRow } from '../api/backend';

/** Go sends money as integer paise (AD-04); the view layer works in rupees. */
export const rupees = (paise: number): number => paise / 100;
export const toPaise = (r: number): number => Math.round(r * 100);

export interface LiveHolding {
  row: HoldingRow;
  ltp: number;
  invested: number;
  current: number;
  pnl: number;
  netChangePct: number | null;
  dayChange: number;
  dayChangePct: number | null;
}

/** One holding at the newest LTP (`tick.batch`), falling back to the LTP Go priced the snapshot at. */
export function holdingRow(row: HoldingRow, liveLtp?: number): LiveHolding {
  const ltpPaise = liveLtp === undefined ? row.ltpPaise : toPaise(liveLtp);
  const currentPaise = row.qty * ltpPaise;
  return {
    row,
    ltp: rupees(ltpPaise),
    invested: rupees(row.investedPaise),
    current: rupees(currentPaise),
    pnl: rupees(currentPaise - row.investedPaise),
    netChangePct: row.avgPaise > 0 ? ((ltpPaise - row.avgPaise) / row.avgPaise) * 100 : null,
    dayChange: rupees(row.qty * (ltpPaise - row.prevClosePaise)),
    dayChangePct: row.prevClosePaise > 0 ? ((ltpPaise - row.prevClosePaise) / row.prevClosePaise) * 100 : null,
  };
}

export interface HoldingsTotals { empty: boolean; currentValue: number; investment: number; pnl: number; pnlPct: number | null; dayChange: number }

/**
 * FR-HOME-002 card definitions, computed by Go (`orders.Summarize`); the UI only adds the live-LTP difference of each row
 * (current = Σ qty×LTP, investment = Σ qty×avg, day's change = Σ qty×(LTP − prev close)).
 */
export function summarizeHoldings(summary: HoldingsSummary, rows: LiveHolding[]): HoldingsTotals {
  let currentPaise = summary.currentValuePaise;
  let dayPaise = summary.dayChangePaise;
  for (const r of rows) {
    const live = toPaise(r.current);
    currentPaise += live - r.row.currentPaise;
    dayPaise += live - r.row.currentPaise;
  }
  const pnlPaise = currentPaise - summary.investmentPaise;
  return {
    empty: summary.empty,
    currentValue: rupees(currentPaise),
    investment: rupees(summary.investmentPaise),
    pnl: rupees(pnlPaise),
    pnlPct: summary.investmentPaise > 0 ? (pnlPaise / summary.investmentPaise) * 100 : null,
    dayChange: rupees(dayPaise),
  };
}

/**
 * Go's position P&L (`pnlPaise`) is priced at `ltpPaise`; a newer tick moves it by
 * qty × multiplier × (LTP − ltpPaise), the same linear adjustment orders.BuildPositions uses
 * (P0-3 Task 12). `multiplier` is Kite's; Go already maps 0 → 1.
 */
export function positionPnl(row: PositionRow, liveLtp?: number): number {
  if (liveLtp === undefined) return rupees(row.pnlPaise);
  const mult = row.multiplier > 0 ? row.multiplier : 1;
  return rupees(row.pnlPaise + row.qty * mult * (toPaise(liveLtp) - row.ltpPaise));
}

export function positionNetChangePct(row: PositionRow, ltp: number): number | null {
  return row.qty !== 0 && row.avgPaise > 0 ? ((toPaise(ltp) - row.avgPaise) / row.avgPaise) * 100 : null;
}

export function orderStatusLabel(status: string): string {
  const s = status.toLowerCase();
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/** Kite statuses after which an order no longer changes (Go's `orders.SectionOf` non-open sections). */
export function isTerminalStatus(status: string): boolean {
  const s = status.toUpperCase();
  return s === 'COMPLETE' || s === 'REJECTED' || s === 'CANCELLED';
}
```

Create `frontend/src/store/portfolio.ts`:

```ts
import { useCallback, useEffect } from 'react';
import { create } from 'zustand';
import type { Backend, BookSnapshot, HoldingsSnapshot, KiteOrder, PositionsSnapshot, RefreshScope } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import type { ListStatus } from '../components/ListBody';
import { ordersCopy } from '../i18n/copy';
import { isTerminalStatus, orderStatusLabel } from '../portfolio/calc';
import { registerReset } from './reset';
import { pushToast } from './toasts';

interface PortfolioData { holdings: HoldingsSnapshot; positions: PositionsSnapshot; orders: BookSnapshot }
export type PortfolioKind = keyof PortfolioData;

export interface Resource<T> { status: 'idle' | 'ready' | 'error'; data: T; error: string | null; loading: boolean }

interface PortfolioStore {
  holdings: Resource<HoldingsSnapshot>;
  positions: Resource<PositionsSnapshot>;
  orders: Resource<BookSnapshot>;
  /** Bumped when an order fill makes a cached list stale; views re-read the Go cache on change. */
  versions: Record<PortfolioKind, number>;
  patch<K extends PortfolioKind>(kind: K, p: Partial<Resource<PortfolioData[K]>>): void;
  /** Applies a snapshot unless a newer one (by `asOfMs`) is already shown. */
  applySnapshot<K extends PortfolioKind>(kind: K, data: PortfolioData[K]): void;
  applyOrderUpdate(o: KiteOrder): void;
}

const idle = <T,>(data: T): Resource<T> => ({ status: 'idle', data, error: null, loading: false });
const initial = () => ({
  holdings: idle<HoldingsSnapshot>({
    rows: [], summary: { empty: true, currentValuePaise: 0, investmentPaise: 0, pnlPaise: 0, pnlBp: 0, dayChangePaise: 0 }, asOfMs: 0, stale: false,
  }),
  positions: idle<PositionsSnapshot>({ net: [], day: [], netTotalPnlPaise: 0, dayTotalPnlPaise: 0, asOfMs: 0, stale: false }),
  orders: idle<BookSnapshot>({ orders: [], counts: { open: 0, executed: 0, rejected: 0, cancelled: 0 }, asOfMs: 0, stale: false }),
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
  // Go publishes the new book on "orders.changed"; "order.update" only announces terminal states and marks
  // holdings (CNC fills) and positions (any fill) for a re-read of the Go cache.
  applyOrderUpdate: (o) => {
    if (!isTerminalStatus(o.status)) return;
    const status = o.status.toUpperCase();
    pushToast({ level: status === 'REJECTED' ? 'warning' : 'info', message: ordersCopy.updated(o.tradingsymbol, orderStatusLabel(o.status)) });
    const filled = status === 'COMPLETE';
    set((s) => ({
      versions: {
        holdings: s.versions.holdings + (filled && o.product === 'CNC' ? 1 : 0),
        positions: s.versions.positions + (filled ? 1 : 0),
        orders: s.versions.orders,
      },
    }));
  },
}));

registerReset(() => usePortfolioStore.setState(initial()));

const FETCH: { [K in PortfolioKind]: (b: Backend) => Promise<PortfolioData[K]> } = {
  holdings: (b) => b.portfolio.holdings(),
  positions: (b) => b.portfolio.positions(),
  orders: (b) => b.portfolio.orderBook(),
};
const SCOPE: Record<PortfolioKind, RefreshScope> = { holdings: 'holdings', positions: 'positions', orders: 'orders' };
const latestRequest: Record<PortfolioKind, number> = { holdings: 0, positions: 0, orders: 0 };

/** Reads the Go cache (no Kite call); a slower, older response never overwrites a newer one. */
export async function loadPortfolio<K extends PortfolioKind>(kind: K, backend: Backend): Promise<void> {
  const mine = ++latestRequest[kind];
  usePortfolioStore.getState().patch(kind, { loading: true });
  try {
    const data = await FETCH[kind](backend);
    if (mine !== latestRequest[kind]) return;
    usePortfolioStore.getState().applySnapshot(kind, data);
    usePortfolioStore.getState().patch(kind, { loading: false });
  } catch (e) {
    if (mine !== latestRequest[kind]) return;
    usePortfolioStore.getState().patch(kind, { status: 'error', error: parseBackendError(e).reason, loading: false });
  }
}

/** The Refresh button: Go re-fetches from Kite (`Portfolio.Refresh(scope)`), then the cache is re-read. */
export async function refreshPortfolio(kind: PortfolioKind, backend: Backend): Promise<void> {
  usePortfolioStore.getState().patch(kind, { loading: true });
  try {
    await backend.portfolio.refresh(SCOPE[kind]);
  } catch (e) {
    usePortfolioStore.getState().patch(kind, { loading: false });
    pushToast({ level: 'error', message: parseBackendError(e).reason });
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
    const id = window.setInterval(() => void reload(), opts.pollMs);
    return () => window.clearInterval(id);
  }, [reload, opts.pollMs]);
  return { resource, reload, refresh };
}

export function listStatusOf(r: Resource<unknown>, isEmpty: boolean): ListStatus {
  if (r.status === 'error') return 'error';
  if (r.status === 'idle') return 'loading';
  return isEmpty ? 'empty' : 'ready';
}
```

Create `frontend/src/components/Values.tsx`:

```tsx
import { formatPercent, formatSigned, formatSignedMoney, toneOf } from '../i18n/format';

export function Signed({ value, kind = 'number' }: { value: number | null; kind?: 'money' | 'percent' | 'number' }) {
  const text = kind === 'money' ? formatSignedMoney(value) : kind === 'percent' ? formatPercent(value) : formatSigned(value);
  return <span className={`tone-${toneOf(value)}`}>{text}</span>;
}

export function InstrumentName({ name, exchange }: { name: string; exchange: string }) {
  return (
    <>
      <span className="instrument__name">{name}</span>
      <span className="chip">{exchange}</span>
    </>
  );
}
```

Append to `frontend/src/components/list.css`:

```css
.chip { display: inline-block; margin-left: var(--space-2); padding: 0 var(--space-1); border: 1px solid var(--border); border-radius: var(--radius-1); font-size: var(--fs-xs); color: var(--text-muted); vertical-align: middle; }
.data-table tfoot td.num, .data-table td.num { text-align: right; }
```

Append to `frontend/src/i18n/copy.ts` (`ordersCopy` gets its remaining strings in Task 11):

```ts
export const holdingsCopy = {
  title: 'Holdings',
  description: 'Long-term equity holdings in your Kite account, valued at live prices.',
  empty: 'No holdings',
  refresh: 'Refresh',
  loading: 'Loading holdings',
  total: 'Total',
  cols: { symbol: 'Symbol', qty: 'Qty', avg: 'Avg', ltp: 'LTP', invested: 'Invested', current: 'Current value', pnl: 'P&L', net: 'Net change', day: 'Day change' },
} as const;

export const ordersCopy = {
  updated: (symbol: string, status: string) => `Order ${symbol}: ${status}`,
} as const;
```

Create `frontend/src/views/holdings/HoldingsView.tsx`:

```tsx
import { useMemo, useState } from 'react';
import { DataTable, type Column } from '../../components/DataTable';
import { filterRows } from '../../components/filterRows';
import { ListBody } from '../../components/ListBody';
import { ListPage } from '../../components/ListPage';
import { InstrumentName, Signed } from '../../components/Values';
import { holdingsCopy as copy } from '../../i18n/copy';
import { formatMoney, formatNumber, formatPrice } from '../../i18n/format';
import { holdingRow, rupees, summarizeHoldings, type LiveHolding } from '../../portfolio/calc';
import { listStatusOf, usePortfolioResource } from '../../store/portfolio';
import { useTicksStore } from '../../store/ticks';

const columns: Column<LiveHolding>[] = [
  { id: 'symbol', header: copy.cols.symbol, width: '24%', cell: (r) => <InstrumentName name={r.row.symbol} exchange={r.row.exchange} />, title: (r) => r.row.symbol },
  { id: 'qty', header: copy.cols.qty, align: 'end', cell: (r) => formatNumber(r.row.qty, 0) },
  { id: 'avg', header: copy.cols.avg, align: 'end', cell: (r) => formatPrice(rupees(r.row.avgPaise)) },
  { id: 'ltp', header: copy.cols.ltp, align: 'end', cell: (r) => formatPrice(r.ltp) },
  { id: 'invested', header: copy.cols.invested, align: 'end', cell: (r) => formatMoney(r.invested) },
  { id: 'current', header: copy.cols.current, align: 'end', cell: (r) => formatMoney(r.current) },
  { id: 'pnl', header: copy.cols.pnl, align: 'end', cell: (r) => <Signed value={r.pnl} kind="money" /> },
  { id: 'net', header: copy.cols.net, align: 'end', cell: (r) => <Signed value={r.netChangePct} kind="percent" /> },
  { id: 'day', header: copy.cols.day, align: 'end', cell: (r) => <Signed value={r.dayChange} kind="money" /> },
];

export function HoldingsView() {
  const { resource, reload, refresh } = usePortfolioResource('holdings');
  const latest = useTicksStore((s) => s.latest);
  const [query, setQuery] = useState('');
  const rows = useMemo(() => resource.data.rows.map((h) => holdingRow(h, latest[h.token]?.ltp)), [resource.data, latest]);
  const visible = filterRows(rows, query, (r) => [r.row.symbol, r.row.exchange]);
  const totals = summarizeHoldings(resource.data.summary, rows);

  return (
    <ListPage title={copy.title} description={copy.description} onRefresh={() => void refresh()} refreshing={resource.loading} search={{ value: query, onChange: setQuery }}>
      <ListBody
        status={listStatusOf(resource, rows.length === 0)}
        error={resource.error}
        onRetry={() => void reload()}
        empty={{ message: copy.empty, action: { label: copy.refresh, onSelect: () => void refresh() } }}
        loadingLabel={copy.loading}
      >
        <DataTable
          caption={copy.title}
          columns={columns}
          rows={visible}
          rowKey={(r) => String(r.row.token)}
          rowLabel={(r) => r.row.symbol}
          footer={
            <tr>
              <td>{copy.total}</td>
              <td /><td /><td />
              <td className="num">{formatMoney(totals.investment)}</td>
              <td className="num">{formatMoney(totals.currentValue)}</td>
              <td className="num"><Signed value={totals.pnl} kind="money" /></td>
              <td className="num"><Signed value={totals.pnlPct} kind="percent" /></td>
              <td className="num"><Signed value={totals.dayChange} kind="money" /></td>
            </tr>
          }
        />
      </ListBody>
    </ListPage>
  );
}
```

In `frontend/src/api/events.ts` add `import { usePortfolioStore } from '../store/portfolio';` and append to the `offs` array:

```ts
    backend.on('order.update', (o) => usePortfolioStore.getState().applyOrderUpdate(o)),
    backend.on('orders.changed', (b) => usePortfolioStore.getState().applySnapshot('orders', b)),
    backend.on('positions.changed', (p) => usePortfolioStore.getState().applySnapshot('positions', p)),
    backend.on('holdings.changed', (h) => usePortfolioStore.getState().applySnapshot('holdings', h)),
```

In `frontend/src/app/routes.tsx` add `import { HoldingsView } from '../views/holdings/HoldingsView';` and append `{ path: 'holdings', element: <HoldingsView /> },` to `shellChildren`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (32 test files), `tsc` exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src
git commit -m "feat(frontend): portfolio store and arithmetic, read-only Holdings view (P0-4 Task 10)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
