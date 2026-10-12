> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 12.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 12: Home layout with the Holdings summary card (P0-5 fills the chart and watchlist slots)

**Files:**
- Create: `frontend/src/views/home/HoldingsSummaryCard.tsx`, `frontend/src/views/home/slots.tsx`, `frontend/src/views/home/home.css`, `frontend/e2e/home-layout.spec.ts`
- Overwrite: `frontend/src/views/home/HomeView.tsx`
- Modify: `frontend/src/i18n/copy.ts`
- Test: `frontend/src/views/home/HomeView.test.tsx`

**Interfaces:**
- Consumes: Task 1 `HomeLayout`; Task 10 `usePortfolioResource` (`reload`), `holdingRow`, `summarizeHoldings` (Go `HoldingsSummary` + live overlay); Task 3 `Skeleton`, `Button`; Task 2 `useTicksStore`, `kiteOrder`, `tickFixture`.
- Produces: `HoldingsSummaryCard()` (FR-HOME-002; refetch on mount, on CNC fills via `versions.holdings`, every `HOLDINGS_REFRESH_MS = 300_000`); `HomeChartArea()` and `HomeWatchlistArea()` in `slots.tsx` — **P0-5 replaces these two component bodies** with the main chart (FR-CHART-001/002, first run NIFTY 50 at `chart.default_timeframe`) and the watchlist panel; `HomeView()`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/views/home/HomeView.test.tsx`:

```tsx
import { act, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { demoSeed, emptySeed, holdingA, kiteOrder, tickFixture } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { renderApp } from '../../test/render';

afterEach(() => vi.useRealTimers());

const card = () => screen.findByRole('region', { name: 'Holdings' });

describe('FR-HOME-002 Holdings summary card', () => {
  it("AC1: current value, investment, P&L with percent and day's change", async () => {
    renderApp();
    const c = await card();
    await within(c).findByText('₹2,050.00');
    expect(c).toHaveTextContent('Current value₹2,050.00');
    expect(c).toHaveTextContent('Investment₹2,000.00');
    expect(c).toHaveTextContent('+₹50.00 (+2.50%)');
    expect(c).toHaveTextContent("Day's change+₹0.00");
  });
  it('AC2: a tick moves Current value immediately', async () => {
    const { backend } = renderApp();
    const c = await card();
    await within(c).findByText('₹2,050.00');
    act(() => backend.emit('tick.batch', [tickFixture(holdingA.token, 120, 105)]));
    expect(within(c).getByText('₹2,150.00')).toBeInTheDocument();
  });
  it('AC3: zero holdings → "No holdings" and no ₹0.00 values', async () => {
    renderApp({ backend: createMockBackend(emptySeed()) });
    const c = await card();
    expect(await within(c).findByText('No holdings')).toBeInTheDocument();
    expect(c).not.toHaveTextContent('₹0.00');
  });
  it('AC4: clicking the card opens the Holdings view', async () => {
    const { user, router } = renderApp();
    const c = await card();
    await user.click(await within(c).findByRole('link'));
    expect(router.state.location.pathname).toBe('/holdings');
  });
  it('re-reads every 5 minutes and after a CNC fill, not after an intraday fill', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { backend } = renderApp();
    await within(await card()).findByText('₹2,050.00');
    const calls = () => backend.callsTo('portfolio.holdings').length;
    const start = calls();
    await act(async () => { vi.advanceTimersByTime(300_000); });
    expect(calls()).toBe(start + 1);
    await act(async () => {
      backend.emit('order.update', kiteOrder({ order_id: '261012000000002', tradingsymbol: 'INFY', product: 'MIS', transaction_type: 'SELL', status: 'COMPLETE' }));
    });
    expect(calls()).toBe(start + 1);
    await act(async () => { backend.emit('order.update', kiteOrder({ status: 'COMPLETE', filled_quantity: 10 })); }); // CNC
    expect(calls()).toBe(start + 2);
  });
});

describe('FR-HOME-001 dashboard layout', () => {
  it('AC2: a failed holdings request shows an error only in the card; the other areas still render', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('portfolio.holdings', 'Network unavailable');
    const { user } = renderApp({ backend });
    const c = await card();
    expect(await within(c).findByRole('alert')).toHaveTextContent('Network unavailable');
    expect(screen.getByRole('region', { name: 'Main chart' })).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: 'Watchlists and widgets' })).toBeInTheDocument();
    await user.click(within(c).getByRole('button', { name: 'Retry' }));
    expect(await within(c).findByText('₹2,050.00')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && npx vitest run src/views/home`
Expected: FAIL — no region named "Holdings" on Home.

- [ ] **Step 3: Implement the card, the slots and the view**

Append to `frontend/src/i18n/copy.ts`:

```ts
export const holdingsCardCopy = {
  title: 'Holdings',
  loading: 'Loading holdings summary',
  error: (reason: string) => `Holdings could not be loaded: ${reason}`,
  empty: 'No holdings',
  open: 'Open the Holdings view',
  currentValue: 'Current value',
  investment: 'Investment',
  pnl: 'P&L',
  dayChange: "Day's change",
} as const;
```

Create `frontend/src/views/home/slots.tsx`:

```tsx
import { homeCopy } from '../../i18n/copy';
import './home.css';

/** P0-5 replaces this body with the main chart (FR-CHART-001/002). */
export function HomeChartArea() {
  return <section className="card home-slot" aria-label={homeCopy.chartArea} />;
}

/** P0-5 replaces this body with the watchlist panel (FR-WATCH-001). */
export function HomeWatchlistArea() {
  return <section className="card home-slot" aria-label={homeCopy.watchlistArea} />;
}
```

Create `frontend/src/views/home/HoldingsSummaryCard.tsx`:

```tsx
import { useId, useMemo, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Button } from '../../components/Button';
import { Skeleton } from '../../components/Skeleton';
import { holdingsCardCopy as copy, listCopy } from '../../i18n/copy';
import { formatMoney, formatPercent, formatSignedMoney, toneOf } from '../../i18n/format';
import { holdingRow, summarizeHoldings } from '../../portfolio/calc';
import { usePortfolioResource } from '../../store/portfolio';
import { useTicksStore } from '../../store/ticks';
import './home.css';

export const HOLDINGS_REFRESH_MS = 300_000;

export function HoldingsSummaryCard() {
  const titleId = useId();
  // Reads the Go cache on mount, every 5 minutes and after CNC fills; "holdings.changed" pushes arrive in between.
  const { resource, reload } = usePortfolioResource('holdings', { pollMs: HOLDINGS_REFRESH_MS });
  const latest = useTicksStore((s) => s.latest);
  const rows = useMemo(() => resource.data.rows.map((h) => holdingRow(h, latest[h.token]?.ltp)), [resource.data, latest]);
  const s = summarizeHoldings(resource.data.summary, rows);

  let body: ReactNode;
  if (resource.status === 'idle') {
    body = <Skeleton rows={3} label={copy.loading} />;
  } else if (resource.status === 'error') {
    body = (
      <div className="card__error" role="alert">
        <p>{copy.error(resource.error ?? '')}</p>
        <Button onClick={() => void reload()}>{listCopy.retry}</Button>
      </div>
    );
  } else if (s.empty || rows.length === 0) {
    body = <Link to="/holdings" className="holdings-card__body" title={copy.open}>{copy.empty}</Link>;
  } else {
    const pnlClass = s.pnl >= 0 ? 'tone-up' : 'tone-down';
    body = (
      <Link to="/holdings" className="holdings-card__body" title={copy.open}>
        <dl className="holdings-card__grid">
          <div><dt>{copy.currentValue}</dt><dd>{formatMoney(s.currentValue)}</dd></div>
          <div><dt>{copy.investment}</dt><dd>{formatMoney(s.investment)}</dd></div>
          <div>
            <dt>{copy.pnl}</dt>
            <dd className={pnlClass}><span aria-hidden="true">{s.pnl >= 0 ? '▲' : '▼'}</span> {`${formatSignedMoney(s.pnl)} (${formatPercent(s.pnlPct)})`}</dd>
          </div>
          <div><dt>{copy.dayChange}</dt><dd className={`tone-${toneOf(s.dayChange)}`}>{formatSignedMoney(s.dayChange)}</dd></div>
        </dl>
      </Link>
    );
  }

  return (
    <section className="card holdings-card" aria-labelledby={titleId}>
      <h2 id={titleId} className="card__title">{copy.title}</h2>
      {body}
    </section>
  );
}
```

Create `frontend/src/views/home/home.css`:

```css
.card__title { font-size: var(--fs-md); margin-bottom: var(--space-3); }
.card__error { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-2); }
.holdings-card__body { display: block; padding: var(--space-1); border-radius: var(--radius-1); color: inherit; text-decoration: none; }
.holdings-card__body:hover { background: var(--surface-2); }
.holdings-card__grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-3); margin: 0; }
.holdings-card__grid dt { font-size: var(--fs-xs); color: var(--text-muted); }
.holdings-card__grid dd { margin: 0; font-size: var(--fs-lg); font-weight: 600; font-variant-numeric: tabular-nums; }
.home-slot { min-height: 360px; flex: 1 1 auto; }
```

Overwrite `frontend/src/views/home/HomeView.tsx`:

```tsx
import { HomeLayout } from '../../app/HomeLayout';
import { homeCopy } from '../../i18n/copy';
import { HoldingsSummaryCard } from './HoldingsSummaryCard';
import { HomeChartArea, HomeWatchlistArea } from './slots';

/** FR-HOME-001. Later phases add the Live Strategy Overview card (P4), analytics (P1) and right-column widgets (P2/P3). */
export function HomeView() {
  return (
    <>
      <h1 className="visually-hidden">{homeCopy.title}</h1>
      <HomeLayout topRow={<HoldingsSummaryCard />} main={<HomeChartArea />} right={<HomeWatchlistArea />} />
    </>
  );
}
```

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (35 test files), `tsc` exits 0.

- [ ] **Step 5: Add the independent-scroll Playwright check and run it**

Create `frontend/e2e/home-layout.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

test('FR-SHELL-001 AC4 + FR-HOME-001 AC3: the right column scrolls on its own; nothing scrolls horizontally at 1280×720', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/');
  const right = page.getByTestId('home-right');
  await expect(right).toBeVisible();
  await right.evaluate((el) => {
    const filler = document.createElement('div');
    filler.style.height = '3000px';
    el.appendChild(filler);
  });
  const chart = page.getByRole('region', { name: 'Main chart' });
  const before = await chart.boundingBox();
  await right.evaluate((el) => { el.scrollTop = 1500; });
  expect(await right.evaluate((el) => el.scrollTop)).toBeGreaterThan(0);
  expect(await chart.boundingBox()).toEqual(before);
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
});
```

Run: `cd frontend && npx playwright test e2e/home-layout.spec.ts`
Expected: PASS (1 test × 2 projects: webkit, chromium).

- [ ] **Step 6: Commit**

```bash
git add frontend/src frontend/e2e
git commit -m "feat(frontend): Home layout with holdings summary card and P0-5 slots (P0-4 Task 12)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
