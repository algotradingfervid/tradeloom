> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 11.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 11: Full-view chart (FR-CHART-017)

The full view is a route in P0-4's `rootChildren`, i.e. **outside the shell**: no top bar, sidebar or ticker is rendered, so the chart fills the window without any shell flag. `chart.fullView` (F11, global) moves between the framed Home chart and the full view in both directions.

**Files:**
- Create: `frontend/src/views/chart-full/ChartFullView.tsx`
- Test: `frontend/src/views/chart-full/ChartFullView.test.tsx`
- Modify: `frontend/src/app/routes.tsx` (P0-4: append the route to `rootChildren`), `frontend/src/chart/ui/DashboardChart.tsx` (pass `onOpenFullView`, register F11), `frontend/src/chart/ui/DashboardChart.test.tsx`, `frontend/src/chart/ui/chart.css`

**Interfaces:**
- Consumes: `ChartView`, `parseTimeframe`, `SHIPPED_DEFAULT_TIMEFRAME`, `DEFAULT_DASHBOARD_SYMBOL`, P0-4 `useShortcut('chart.fullView', handler)` (`src/shortcuts/ShortcutProvider.tsx`), `rootChildren` / `shellChildren` (`src/app/routes.tsx`), `.visually-hidden`.
- Produces: route `/chart-full` (in `rootChildren`) with `location.state: FullViewParams`; `interface FullViewParams { identifier: string; timeframe: TimeframeId }`; `openFullViewChart(navigate: NavigateFunction, params: FullViewParams): void` (P3 scanner rows call this); `FULL_VIEW_CONTEXT = 'full'`, `FULL_VIEW_PATH = '/chart-full'`.

- [ ] **Step 1: Write the failing test**

`frontend/src/views/chart-full/ChartFullView.test.tsx`:

```tsx
import { act, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const shortcuts = vi.hoisted(() => ({ handlers: {} as Record<string, () => void> }));
vi.mock('../../shortcuts/ShortcutProvider', () => ({ useShortcut: (id: string, h: () => void) => { shortcuts.handlers[id] = h; } }));
vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../../chart/LightweightChartsAdapter', async () => ({ LightweightChartsAdapter: (await import('../../chart/testing/FakeAdapter')).FakeAdapter }));

import { ChartFullView } from './ChartFullView';
import { setBackend } from '../../api/current';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { useSessionStore } from '../../store/session';

const renderAt = () => render(
  <MemoryRouter initialEntries={['/home', { pathname: '/chart-full', state: { identifier: 'NSE:ADANIENT', timeframe: '1d' } }]} initialIndex={1}>
    <Routes>
      <Route path="/home" element={<p>Home view</p>} />
      <Route path="/chart-full" element={<ChartFullView />} />
    </Routes>
  </MemoryRouter>,
);

beforeEach(() => {
  shortcuts.handlers = {};
  setBackend(createMockBackend(demoSeed()));
  useSessionStore.setState({ session: readySession() });
});

describe('ChartFullView', () => {
  it('AC1: opens with the caller instrument and timeframe (ADANIENT NSE, 1 Day)', async () => {
    renderAt();
    expect(await screen.findByText('ADANIENT')).toBeInTheDocument();
    expect(screen.getByText('NSE')).toBeInTheDocument();
    expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('1d');
    expect(screen.getByRole('heading', { level: 1, name: 'Full-view chart' })).toBeInTheDocument();
  });

  it('AC2: the back arrow returns to the originating view', async () => {
    renderAt();
    fireEvent.click(await screen.findByRole('button', { name: 'Back' }));
    expect(await screen.findByText('Home view')).toBeInTheDocument();
  });

  it('AC3: F11 (chart.fullView) in the full view restores the framed view it came from', async () => {
    renderAt();
    await screen.findByText('ADANIENT');
    act(() => shortcuts.handlers['chart.fullView']());
    expect(await screen.findByText('Home view')).toBeInTheDocument();
  });

  it('AC3: the route lives outside the shell (rootChildren), so no frame is rendered around it', async () => {
    const { rootChildren, shellChildren } = await import('../../app/routes');
    expect(rootChildren.some((r) => r.path === 'chart-full')).toBe(true);
    expect(shellChildren.some((r) => r.path === 'chart-full')).toBe(false);
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/views/chart-full`
Expected: FAIL with `Failed to resolve import "./ChartFullView"`.

- [ ] **Step 3: Implement**

`frontend/src/views/chart-full/ChartFullView.tsx`:

```tsx
import { useState } from 'react';
import { useLocation, useNavigate, type NavigateFunction } from 'react-router-dom';
import { ChartView } from '../../chart/ui/ChartView';
import { parseTimeframe, SHIPPED_DEFAULT_TIMEFRAME, type TimeframeId } from '../../chart/timeframes';
import { DEFAULT_DASHBOARD_SYMBOL } from '../../store/chartContextsStore';
import { useShortcut } from '../../shortcuts/ShortcutProvider';

export interface FullViewParams { identifier: string; timeframe: TimeframeId }
export const FULL_VIEW_CONTEXT = 'full';
export const FULL_VIEW_PATH = '/chart-full';

/** Entry point for "Open in full view", F11 on Home and (P3) scanner result rows. */
export function openFullViewChart(navigate: NavigateFunction, params: FullViewParams): void {
  navigate(FULL_VIEW_PATH, { state: params });
}

/** Rendered from rootChildren, outside the shell: no top bar or sidebar, the chart fills the window (FR-CHART-017 AC3). */
export function ChartFullView() {
  const location = useLocation();
  const navigate = useNavigate();
  const initial = (location.state as Partial<FullViewParams> | null) ?? {};
  const [identifier, setIdentifier] = useState(initial.identifier ?? DEFAULT_DASHBOARD_SYMBOL);
  const [timeframe, setTimeframe] = useState<TimeframeId>(parseTimeframe(initial.timeframe ?? SHIPPED_DEFAULT_TIMEFRAME));
  const back = () => { void navigate(-1); };

  useShortcut('chart.fullView', back); // F11 again returns to the framed view it came from

  return (
    <main className="chart-full" aria-label="Full-view chart">
      <h1 className="visually-hidden">Full-view chart</h1>
      <ChartView contextId={FULL_VIEW_CONTEXT} identifier={identifier} timeframe={timeframe}
        onSymbolChange={setIdentifier} onTimeframeChange={setTimeframe} onBack={back} />
    </main>
  );
}
```

Append to `frontend/src/chart/ui/chart.css`:

```css
.chart-full { display: flex; flex-direction: column; height: 100vh; min-width: 0; }
.chart-full > .chart-view { flex: 1; }
```

In `frontend/src/app/routes.tsx` (P0-4) add `import { ChartFullView } from '../views/chart-full/ChartFullView';` and append `{ path: 'chart-full', element: <ChartFullView /> },` to `rootChildren` (not `shellChildren`). Root children render inside `RootLayout` (session gate, toasts, shortcuts) but outside `ShellLayout`.

Replace `frontend/src/chart/ui/DashboardChart.tsx` with the version that opens the full view from the ⋯ menu and from F11:

```tsx
import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Skeleton } from '../../components/Skeleton';
import { useShortcut } from '../../shortcuts/ShortcutProvider';
import { useSetting } from '../../store/settings';
import { DASHBOARD_CONTEXT, hydrateDashboard, useChartContexts } from '../../store/chartContextsStore';
import { openFullViewChart } from '../../views/chart-full/ChartFullView';
import { parseTimeframe } from '../timeframes';
import { ChartView } from './ChartView';

export function DashboardChart() {
  // Contract §E: Settings → Chart default timeframe, stored in Kite form ("60minute").
  const [defaultSetting] = useSetting<string>('chart.default_timeframe', '60minute');
  const defaultTf = parseTimeframe(defaultSetting);
  const ctx = useChartContexts((s) => s.contexts[DASHBOARD_CONTEXT]);
  const navigate = useNavigate();
  useEffect(() => { void hydrateDashboard(defaultTf); }, [defaultTf]);
  const openFull = () => { if (ctx) openFullViewChart(navigate, { identifier: ctx.identifier, timeframe: ctx.timeframe }); };
  useShortcut('chart.fullView', openFull);
  if (!ctx) return <div className="chart-view"><Skeleton rows={6} label="Loading chart" /></div>;
  return (
    <ChartView contextId={DASHBOARD_CONTEXT} identifier={ctx.identifier} timeframe={ctx.timeframe}
      onSymbolChange={(id) => useChartContexts.getState().setSymbol(DASHBOARD_CONTEXT, id)}
      onTimeframeChange={(tf) => useChartContexts.getState().setTimeframe(DASHBOARD_CONTEXT, tf)}
      onOpenFullView={openFull} />
  );
}
```

In `frontend/src/chart/ui/DashboardChart.test.tsx`:
- add `act` and `fireEvent` to the `@testing-library/react` import and `Route, Routes, useLocation` to the `react-router-dom` import;
- add these two lines after the existing `vi.mock(…)` lines (DashboardChart now calls `useShortcut`, which needs P0-4's provider):

```tsx
const shortcuts = vi.hoisted(() => ({ handlers: {} as Record<string, () => void> }));
vi.mock('../../shortcuts/ShortcutProvider', () => ({ useShortcut: (id: string, h: () => void) => { shortcuts.handlers[id] = h; } }));
```

- append these tests inside the `describe` block:

```tsx
  const Probe = () => { const l = useLocation(); return <p>at {l.pathname} {JSON.stringify(l.state)}</p>; };
  const renderWithFullRoute = () => render(
    <BackendProvider backend={backend}>
      <MemoryRouter><Routes><Route path="/" element={<DashboardChart />} /><Route path="/chart-full" element={<Probe />} /></Routes></MemoryRouter>
    </BackendProvider>,
  );

  it('FR-CHART-017: ⋯ → Open in full view navigates with the current symbol and timeframe', async () => {
    await backend.settings.setUiState('chart.dashboard', JSON.stringify({ identifier: 'NSE:RELIANCE', timeframe: '15m' }));
    renderWithFullRoute();
    fireEvent.click(await screen.findByRole('button', { name: 'More chart actions' }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Open in full view' }));
    expect(await screen.findByText('at /chart-full {"identifier":"NSE:RELIANCE","timeframe":"15m"}')).toBeInTheDocument();
  });

  it('FR-CHART-017 AC3: F11 (chart.fullView) on Home opens the full view', async () => {
    renderWithFullRoute();
    await screen.findByText('NIFTY 50');
    act(() => shortcuts.handlers['chart.fullView']());
    expect(await screen.findByText('at /chart-full {"identifier":"NSE:NIFTY 50","timeframe":"1h"}')).toBeInTheDocument();
  });
```

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/views/chart-full src/chart/ui/DashboardChart.test.tsx && npm run typecheck`
Expected: PASS (4 + 5 tests). Typecheck exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/views/chart-full frontend/src/app/routes.tsx frontend/src/chart/ui/DashboardChart.tsx frontend/src/chart/ui/DashboardChart.test.tsx frontend/src/chart/ui/chart.css
git commit -m "$(cat <<'EOF'
feat(chart): full-view chart route outside the shell with back navigation and F11 toggle

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---