> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 11.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 11: Positions and Order Book views (read-only) + 1280 px long-name layout check

**Files:**
- Create: `frontend/src/components/Tabs.tsx`, `frontend/src/views/positions/PositionsView.tsx`, `frontend/src/views/orders/OrderBookView.tsx`, `frontend/e2e/portfolio-layout.spec.ts`
- Modify: `frontend/src/app/routes.tsx` (`positions`, `orders`), `frontend/src/components/list.css`, `frontend/src/i18n/copy.ts`
- Test: `frontend/src/views/positions/PositionsView.test.tsx`, `frontend/src/views/orders/OrderBookView.test.tsx`

**Interfaces:**
- Consumes: Task 10 `usePortfolioResource` (`reload`, `refresh`), `listStatusOf`, `positionPnl`, `positionNetChangePct`, `orderStatusLabel`, `rupees`, `Signed`, `InstrumentName`, `ordersCopy.updated`; Task 5 list components; Task 2 `useStatusStore`, `useTicksStore`, `PositionRow`, `OrderRow` (Go `section`, `side`, `source`, paise prices), `OrderSection`, `bookOf`, `tickFixture`.
- Produces: `Tabs({ label: string; tabs: { id: string; label: string }[]; value: string; onChange(id: string): void })` (`role="tablist"`, arrow keys); `PositionsView()` at `/positions`; `OrderBookView()` at `/orders`; `ORDER_POLL_MS = 30_000`. Row actions (convert, exit, cancel, modify) and the Paper tab are **not** rendered in P0 (FR-ORD-002/011, FR-DEP-017 GATE).

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/views/positions/PositionsView.test.tsx`:

```tsx
import { act, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { emptySeed, positionOpen, tickFixture } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { renderApp } from '../../test/render';

describe('FR-ORD-002 Positions view (read-only in P0)', () => {
  it('AC1: Total P&L of +120 and −45.50 is +74.50; open rows come before closed ones', async () => {
    renderApp({ route: '/positions' });
    const open = await screen.findByRole('row', { name: 'INFY' });
    const closed = screen.getByRole('row', { name: 'TATAMOTORS' });
    expect(open.compareDocumentPosition(closed) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(closed).toHaveClass('row--muted');
    expect(screen.getAllByRole('rowgroup')[2]).toHaveTextContent('Total P&L+₹74.50');
  });
  it('shows Product, Symbol, Qty, Avg, LTP, P&L and Net change with live LTP', async () => {
    const { backend } = renderApp({ route: '/positions' });
    const row = await screen.findByRole('row', { name: 'INFY' });
    expect(within(row).getAllByRole('cell').map((c) => c.textContent)).toEqual(['MIS', 'INFYNSE', '10', '100.00', '112.00', '+₹120.00', '+12.00%']);
    act(() => backend.emit('tick.batch', [tickFixture(positionOpen.token, 113, 110)]));
    expect(within(screen.getByRole('row', { name: 'INFY' })).getAllByRole('cell')[5]).toHaveTextContent('+₹130.00');
  });
  it('switches between Net and Day with the arrow keys', async () => {
    const { user } = renderApp({ route: '/positions' });
    const net = await screen.findByRole('tab', { name: 'Net' });
    expect(net).toHaveAttribute('aria-selected', 'true');
    net.focus();
    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: 'Day' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', { name: 'Day' })).toHaveFocus();
  });
  it('offers no row actions in a P0 build (conversion arrives with trading in P4)', async () => {
    renderApp({ route: '/positions' });
    const row = await screen.findByRole('row', { name: 'INFY' });
    expect(within(row).queryAllByRole('button')).toHaveLength(0);
  });
  it('empty state', async () => {
    renderApp({ route: '/positions', backend: createMockBackend(emptySeed()) });
    expect(await screen.findByText('No positions')).toBeInTheDocument();
  });
});
```

Create `frontend/src/views/orders/OrderBookView.test.tsx`:

```tsx
import { act, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { bookOf, demoSeed, FIXED_NOW, ordersFixture } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { renderApp } from '../../test/render';

afterEach(() => vi.useRealTimers());

describe('FR-ORD-001 Order book (read-only in P0)', () => {
  it('groups orders into Open, Executed, Rejected and Cancelled with counts', async () => {
    renderApp({ route: '/orders' });
    for (const name of ['Open (1)', 'Executed (1)', 'Rejected (1)', 'Cancelled (1)']) {
      expect(await screen.findByRole('heading', { name })).toBeInTheDocument();
    }
  });
  it('AC1 (UI part): the Source column says whether Tradeloom placed the order (Go `source` from the TL tag); rejected rows show the reason', async () => {
    renderApp({ route: '/orders' });
    const executed = await screen.findByRole('row', { name: 'INFY 261012000000002' });
    expect(within(executed).getAllByRole('cell').map((c) => c.textContent)).toEqual(
      ['10:10:00', 'Sell', 'INFYNSE', 'MIS', '5/5', '1,510.25', 'Complete', 'Tradeloom'],
    );
    expect(within(screen.getByRole('row', { name: 'RELIANCE 261012000000001' })).getAllByRole('cell')[7]).toHaveTextContent('Kite');
    expect(within(screen.getByRole('row', { name: 'TCS 261012000000003' })).getByText('Insufficient funds')).toBeInTheDocument();
  });
  it('searches by instrument, order ID or source', async () => {
    const { user } = renderApp({ route: '/orders' });
    await screen.findByRole('heading', { name: 'Open (1)' });
    await user.type(screen.getByRole('searchbox', { name: 'Search order book' }), '000003');
    expect(screen.getByRole('heading', { name: 'Rejected (1)' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Open (0)' })).toBeInTheDocument();
  });
  it('applies the orders.changed snapshot Go pushes, without a re-read', async () => {
    const { backend } = renderApp({ route: '/orders' });
    await screen.findByRole('heading', { name: 'Open (1)' });
    const reads = backend.callsTo('portfolio.orderBook').length;
    const sbin = {
      ...ordersFixture[0], orderId: '261012000000009', timeMs: FIXED_NOW, symbol: 'SBIN', token: 779521, qty: 3,
      orderType: 'SL', status: 'TRIGGER PENDING',
    };
    act(() => backend.emit('orders.changed', bookOf([...ordersFixture, sbin], Date.now())));
    expect(screen.getByRole('heading', { name: 'Open (2)' })).toBeInTheDocument();
    expect(screen.getByRole('row', { name: 'SBIN 261012000000009' })).toHaveTextContent('Trigger pending');
    expect(backend.callsTo('portfolio.orderBook')).toHaveLength(reads);
  });
  it('AC2 (UI part): while the feed is down the list is re-read every 30 s', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const seed = demoSeed();
    seed.status.feed = { ...seed.status.feed, state: 'offline' };
    const { backend } = renderApp({ route: '/orders', backend: createMockBackend(seed) });
    await screen.findByRole('heading', { name: 'Open (1)' });
    const before = backend.callsTo('portfolio.orderBook').length;
    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });
    expect(backend.callsTo('portfolio.orderBook').length).toBe(before + 1);
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/views/positions src/views/orders`
Expected: FAIL — the views are not routed (`findByRole` timeouts on the headings).

- [ ] **Step 3: Implement tabs and both views**

Append `positionsCopy` to `frontend/src/i18n/copy.ts` and replace the one-line `ordersCopy` added in Task 10 with the full object below (it keeps `updated`):

```ts
export const positionsCopy = {
  title: 'Positions',
  description: 'Open and closed positions for today, with live P&L.',
  tabs: 'Position type',
  net: 'Net',
  day: 'Day',
  empty: 'No positions',
  loading: 'Loading positions',
  totalPnl: 'Total P&L',
  cols: { product: 'Product', symbol: 'Symbol', qty: 'Qty', avg: 'Avg', ltp: 'LTP', pnl: 'P&L', net: 'Net change' },
} as const;

export const ordersCopy = {
  title: 'Order Book',
  description: 'Every order in your Kite account today: open, executed, rejected and cancelled.',
  searchPlaceholder: 'Search by instrument, order ID or source',
  empty: 'No orders today',
  loading: 'Loading orders',
  sections: { open: 'Open', executed: 'Executed', rejected: 'Rejected', cancelled: 'Cancelled' },
  sectionEmpty: { open: 'No open orders', executed: 'No executed orders', rejected: 'No rejected orders', cancelled: 'No cancelled orders' },
  // Go `orders.OrderRow.source`: "tradeloom" for orders tagged TL…, "external" for orders placed in Kite itself.
  sources: { tradeloom: 'Tradeloom', external: 'Kite' },
  cols: { time: 'Time', type: 'Type', instrument: 'Instrument', product: 'Product', qty: 'Qty', avg: 'Avg price', status: 'Status', source: 'Source' },
  updated: (symbol: string, status: string) => `Order ${symbol}: ${status}`,
} as const;
```

Create `frontend/src/components/Tabs.tsx`:

```tsx
import { useRef, type KeyboardEvent } from 'react';
import './list.css';

export interface TabDef { id: string; label: string }

export function Tabs({ label, tabs, value, onChange }: { label: string; tabs: TabDef[]; value: string; onChange: (id: string) => void }) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const onKeyDown = (e: KeyboardEvent<HTMLButtonElement>, i: number) => {
    if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return;
    e.preventDefault();
    const j = (i + (e.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
    onChange(tabs[j].id);
    refs.current[j]?.focus();
  };
  return (
    <div role="tablist" aria-label={label} className="tabs">
      {tabs.map((t, i) => (
        <button
          key={t.id}
          ref={(el) => { refs.current[i] = el; }}
          type="button"
          role="tab"
          id={`tab-${t.id}`}
          aria-selected={t.id === value}
          tabIndex={t.id === value ? 0 : -1}
          onClick={() => onChange(t.id)}
          onKeyDown={(e) => onKeyDown(e, i)}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}
```

Append to `frontend/src/components/list.css`:

```css
.tabs { display: flex; gap: var(--space-1); border-bottom: 1px solid var(--border); }
.tabs [role="tab"] { padding: var(--space-2) var(--space-3); border: none; border-bottom: 2px solid transparent; background: none; cursor: pointer; }
.tabs [role="tab"][aria-selected="true"] { border-bottom-color: var(--accent); color: var(--accent); font-weight: 600; }
.list-section h2 { font-size: var(--fs-md); }
.order-status__reason { display: block; font-size: var(--fs-xs); color: var(--text-muted); white-space: normal; }
```

Create `frontend/src/views/positions/PositionsView.tsx`:

```tsx
import { useMemo, useState } from 'react';
import type { PositionRow } from '../../api/backend';
import { DataTable, type Column } from '../../components/DataTable';
import { filterRows } from '../../components/filterRows';
import { ListBody } from '../../components/ListBody';
import { ListPage } from '../../components/ListPage';
import { Tabs } from '../../components/Tabs';
import { InstrumentName, Signed } from '../../components/Values';
import { positionsCopy as copy } from '../../i18n/copy';
import { formatNumber, formatPrice } from '../../i18n/format';
import { positionNetChangePct, positionPnl, rupees } from '../../portfolio/calc';
import { listStatusOf, usePortfolioResource } from '../../store/portfolio';
import { useTicksStore } from '../../store/ticks';

interface LivePosition { position: PositionRow; ltp: number; pnl: number; netPct: number | null; open: boolean }

const columns: Column<LivePosition>[] = [
  { id: 'product', header: copy.cols.product, width: '9%', cell: (r) => r.position.product },
  { id: 'symbol', header: copy.cols.symbol, width: '30%', cell: (r) => <InstrumentName name={r.position.symbol} exchange={r.position.exchange} />, title: (r) => r.position.symbol },
  { id: 'qty', header: copy.cols.qty, align: 'end', cell: (r) => formatNumber(r.position.qty, 0) },
  { id: 'avg', header: copy.cols.avg, align: 'end', cell: (r) => formatPrice(rupees(r.position.avgPaise)) },
  { id: 'ltp', header: copy.cols.ltp, align: 'end', cell: (r) => formatPrice(r.ltp) },
  { id: 'pnl', header: copy.cols.pnl, align: 'end', cell: (r) => <Signed value={r.pnl} kind="money" /> },
  { id: 'net', header: copy.cols.net, align: 'end', cell: (r) => <Signed value={r.netPct} kind="percent" /> },
];

export function PositionsView() {
  const { resource, reload, refresh } = usePortfolioResource('positions');
  const latest = useTicksStore((s) => s.latest);
  const [tab, setTab] = useState<'net' | 'day'>('net');
  const [query, setQuery] = useState('');

  const rows = useMemo<LivePosition[]>(
    () =>
      resource.data[tab]
        .map((p) => {
          const live = latest[p.token]?.ltp;
          const ltp = live ?? rupees(p.ltpPaise);
          return { position: p, ltp, pnl: positionPnl(p, live), netPct: positionNetChangePct(p, ltp), open: !p.closed };
        })
        .sort((a, b) => Number(b.open) - Number(a.open)),
    [resource.data, tab, latest],
  );
  const visible = filterRows(rows, query, (r) => [r.position.symbol, r.position.exchange, r.position.product]);
  const total = rows.reduce((acc, r) => acc + r.pnl, 0);

  return (
    <ListPage
      title={copy.title}
      description={copy.description}
      onRefresh={() => void refresh()}
      refreshing={resource.loading}
      search={{ value: query, onChange: setQuery }}
      tabs={<Tabs label={copy.tabs} tabs={[{ id: 'net', label: copy.net }, { id: 'day', label: copy.day }]} value={tab} onChange={(id) => setTab(id as 'net' | 'day')} />}
    >
      <div role="tabpanel" aria-labelledby={`tab-${tab}`}>
        <ListBody status={listStatusOf(resource, rows.length === 0)} error={resource.error} onRetry={() => void reload()} empty={{ message: copy.empty }} loadingLabel={copy.loading}>
          <DataTable
            caption={`${copy.title} — ${tab === 'net' ? copy.net : copy.day}`}
            columns={columns}
            rows={visible}
            rowKey={(r) => r.position.key}
            rowLabel={(r) => r.position.symbol}
            rowClassName={(r) => (r.open ? undefined : 'row--muted')}
            footer={
              <tr>
                <td colSpan={5}>{copy.totalPnl}</td>
                <td className="num"><Signed value={total} kind="money" /></td>
                <td />
              </tr>
            }
          />
        </ListBody>
      </div>
    </ListPage>
  );
}
```

Create `frontend/src/views/orders/OrderBookView.tsx`:

```tsx
import { useMemo, useState } from 'react';
import type { OrderRow, OrderSection } from '../../api/backend';
import { DataTable, type Column } from '../../components/DataTable';
import { filterRows } from '../../components/filterRows';
import { ListBody } from '../../components/ListBody';
import { ListPage } from '../../components/ListPage';
import { InstrumentName } from '../../components/Values';
import { ordersCopy as copy } from '../../i18n/copy';
import { DASH, formatNumber, formatPrice, formatTime } from '../../i18n/format';
import { orderStatusLabel, rupees } from '../../portfolio/calc';
import { listStatusOf, usePortfolioResource } from '../../store/portfolio';
import { useStatusStore } from '../../store/status';

export const ORDER_POLL_MS = 30_000;
const SECTIONS: OrderSection[] = ['open', 'executed', 'rejected', 'cancelled'];

const sourceLabel = (o: OrderRow) => copy.sources[o.source];

const columns: Column<OrderRow>[] = [
  { id: 'time', header: copy.cols.time, width: '9%', cell: (o) => formatTime(o.timeMs, true) },
  { id: 'type', header: copy.cols.type, width: '7%', cell: (o) => <span className={o.side === 'Buy' ? 'tone-up' : 'tone-down'}>{o.side}</span> },
  { id: 'instrument', header: copy.cols.instrument, width: '22%', cell: (o) => <InstrumentName name={o.symbol} exchange={o.exchange} />, title: (o) => o.symbol },
  { id: 'product', header: copy.cols.product, width: '8%', cell: (o) => o.product },
  { id: 'qty', header: copy.cols.qty, align: 'end', width: '9%', cell: (o) => `${formatNumber(o.filledQty, 0)}/${formatNumber(o.qty, 0)}` },
  { id: 'avg', header: copy.cols.avg, align: 'end', width: '11%', cell: (o) => (o.filledQty > 0 ? formatPrice(rupees(o.avgPricePaise)) : DASH) },
  {
    id: 'status', header: copy.cols.status, width: '16%',
    cell: (o) => (
      <>
        {orderStatusLabel(o.status)}
        {o.statusMessage && <span className="order-status__reason">{o.statusMessage}</span>}
      </>
    ),
  },
  { id: 'source', header: copy.cols.source, cell: (o) => sourceLabel(o), title: (o) => (o.tag ? `${sourceLabel(o)} (${o.tag})` : sourceLabel(o)) },
];

export function OrderBookView() {
  // Go re-syncs the book every 30 s and pushes "orders.changed"; while the feed is down the view also re-reads the cache.
  const feedLive = useStatusStore((s) => s.feed.state === 'live');
  const { resource, reload, refresh } = usePortfolioResource('orders', { pollMs: feedLive ? undefined : ORDER_POLL_MS });
  const [query, setQuery] = useState('');
  const all = useMemo(() => [...resource.data.orders].sort((a, b) => b.timeMs - a.timeMs), [resource.data]);
  const visible = filterRows(all, query, (o) => [o.symbol, o.orderId, o.tag, sourceLabel(o)]);

  return (
    <ListPage
      title={copy.title}
      description={copy.description}
      onRefresh={() => void refresh()}
      refreshing={resource.loading}
      search={{ value: query, onChange: setQuery, placeholder: copy.searchPlaceholder }}
    >
      <ListBody status={listStatusOf(resource, all.length === 0)} error={resource.error} onRetry={() => void reload()} empty={{ message: copy.empty }} loadingLabel={copy.loading}>
        {SECTIONS.map((section) => {
          const rows = visible.filter((o) => o.section === section);
          const headingId = `orders-${section}`;
          return (
            <section key={section} className="list-section" aria-labelledby={headingId}>
              <h2 id={headingId}>{`${copy.sections[section]} (${rows.length})`}</h2>
              {rows.length === 0 ? (
                <p className="muted">{copy.sectionEmpty[section]}</p>
              ) : (
                <DataTable caption={copy.sections[section]} columns={columns} rows={rows} rowKey={(o) => o.orderId} rowLabel={(o) => `${o.symbol} ${o.orderId}`} />
              )}
            </section>
          );
        })}
      </ListBody>
    </ListPage>
  );
}
```

In `frontend/src/app/routes.tsx` add `import { OrderBookView } from '../views/orders/OrderBookView';` and `import { PositionsView } from '../views/positions/PositionsView';`, and append `{ path: 'positions', element: <PositionsView /> },` and `{ path: 'orders', element: <OrderBookView /> },` to `shellChildren`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (34 test files), `tsc` exits 0. (The executed-row time is `10:10:00` because the fixture is `FIXED_NOW − 20 min` = 10:10 IST.)

- [ ] **Step 5: Add the review-focus layout test (1280 px, long names) and run it**

Create `frontend/e2e/portfolio-layout.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

const LONG_NAME = 'BHARATIYA INTERNATIONAL INFRASTRUCTURE AND ENGINEERING SERVICES LIMITED';

for (const route of ['/holdings', '/positions', '/orders']) {
  test(`review focus: ${route} at 1280×720 with a ${LONG_NAME.length}-character name truncates without horizontal scroll`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 });
    await page.goto('/?seed=longnames');
    await expect(page.getByRole('button', { name: 'Account menu' })).toBeVisible();
    await page.evaluate((r) => window.__tradeloomMock?.emit('app.navigate', { route: r }), route);
    const cell = page.locator('td .cell-truncate', { hasText: LONG_NAME }).first();
    await expect(cell).toBeVisible();
    expect(await cell.getAttribute('title')).toBe(LONG_NAME);
    expect(await cell.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true);
    const frameOverflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(frameOverflow).toBe(0);
    const bodyOverflow = await page.locator('.list-page__body').evaluate((el) => el.scrollWidth - el.clientWidth);
    expect(bodyOverflow).toBeLessThanOrEqual(0);
  });
}
```

Run: `cd frontend && npx playwright test e2e/portfolio-layout.spec.ts`
Expected: PASS (3 tests × 2 projects: webkit, chromium).

- [ ] **Step 6: Commit**

```bash
git add frontend/src frontend/e2e
git commit -m "feat(frontend): read-only Positions and Order Book with live updates (P0-4 Task 11)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
