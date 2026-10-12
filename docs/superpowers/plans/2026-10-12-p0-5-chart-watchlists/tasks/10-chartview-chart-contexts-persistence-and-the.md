> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 10.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 10: `ChartView`, chart contexts, persistence and the dashboard chart

**Files:**
- Create: `frontend/src/api/uiState.ts`, `frontend/src/store/chartContextsStore.ts`, `frontend/src/chart/ui/ChartLegend.tsx`, `frontend/src/chart/ui/ScaleToggles.tsx`, `frontend/src/chart/ui/ChartView.tsx`, `frontend/src/chart/ui/DashboardChart.tsx`
- Test: `frontend/src/api/uiState.test.ts`, `frontend/src/store/chartContextsStore.test.ts`, `frontend/src/chart/ui/ChartView.test.tsx`, `frontend/src/chart/ui/DashboardChart.test.tsx`
- Modify: `frontend/src/views/home/slots.tsx` (P0-4 Task 12: replace the body of `HomeChartArea`), `frontend/src/chart/ui/chart.css` (append the Home slot rule)

**Interfaces:**
- Consumes: Tasks 2–9 (`fetchHistory`, `refreshRange`, `openLive`, `closeLive`, `onCandle`, `onBackfillProgress`, `currentBackend`, `setBackend`, `tickRupees`, `tick`). P0-4 `useSessionStore` (`session?.dataPlan`), `useTheme()` (`{ theme, tokens }`), `ThemeProvider`, `useSetting(key, fallback)`, `useSettingsStore`, `backend.settings.getUiState/setUiState`, `BackendProvider`, `createMockBackend`, `demoSeed`, `readySession`, `Button`, `Skeleton`, `formatPrice`, `formatSigned`, `formatPercent`, `formatCompact`, `homeCopy.chartArea`, `HomeChartArea` (`src/views/home/slots.tsx`).
- Produces:
  - `src/api/uiState.ts`: `readUiState<T>(key: string, fallback: T): Promise<T>` (`""` or invalid JSON → fallback), `writeUiState<T>(key: string, value: T): Promise<void>`
  - `useChartContexts` (zustand `{ contexts: Record<string, ChartContextState>; init(ctx, s); setSymbol(ctx, id); setTimeframe(ctx, tf) }`), `ChartContextState { identifier: string; timeframe: TimeframeId }`, `DASHBOARD_CONTEXT = 'dashboard'`, `DASHBOARD_UI_STATE_KEY = 'chart.dashboard'`, `DEFAULT_DASHBOARD_SYMBOL = 'NSE:NIFTY 50'`, `hydrateDashboard(defaultTf): Promise<void>`, `showOnDashboard(identifier): void`, `__resetChartContexts(): void`
  - `<ChartView contextId identifier timeframe onSymbolChange onTimeframeChange onOpenFullView? onBack? maxStudies? adapterFactory? dataApi? />` (container carries `data-shortcut-scope="chart"`)
  - `<DashboardChart />`, `<ChartLegend data hasVolume decimals />`, `<ScaleToggles auto log onAuto onLog />`

- [ ] **Step 1: Write the failing tests**

`frontend/src/api/uiState.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest';
import { setBackend } from './current';
import { demoSeed } from './mock/fixtures';
import { createMockBackend, type MockBackend } from './mock/index';
import { readUiState, writeUiState } from './uiState';

let backend: MockBackend;
beforeEach(() => { backend = createMockBackend(demoSeed()); setBackend(backend); });

describe('uiState over Settings.GetUiState/SetUiState', () => {
  it('round-trips JSON and returns the fallback when absent', async () => {
    expect(await readUiState('chart.dashboard', null)).toBeNull();
    await writeUiState('chart.dashboard', { identifier: 'NSE:RELIANCE', timeframe: '15m' });
    expect(await backend.settings.getUiState('chart.dashboard')).toBe('{"identifier":"NSE:RELIANCE","timeframe":"15m"}');
    expect(await readUiState('chart.dashboard', null)).toEqual({ identifier: 'NSE:RELIANCE', timeframe: '15m' });
  });
  it('a corrupt stored value falls back instead of throwing', async () => {
    await backend.settings.setUiState('chart.dashboard', '{oops');
    expect(await readUiState('chart.dashboard', 'fallback')).toBe('fallback');
  });
});
```

`frontend/src/store/chartContextsStore.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest';

const saved = vi.hoisted(() => ({ value: null as unknown }));
vi.mock('../api/uiState', () => ({ readUiState: vi.fn(async () => saved.value), writeUiState: vi.fn(async () => undefined) }));
import { readUiState, writeUiState } from '../api/uiState';
import { __resetChartContexts, DASHBOARD_CONTEXT, hydrateDashboard, showOnDashboard, useChartContexts } from './chartContextsStore';

beforeEach(() => { __resetChartContexts(); saved.value = null; vi.clearAllMocks(); });

describe('chartContextsStore', () => {
  it('FR-HOME-001 AC1: first run → NIFTY 50 at the default timeframe', async () => {
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts[DASHBOARD_CONTEXT]).toEqual({ identifier: 'NSE:NIFTY 50', timeframe: '1h' });
    expect(readUiState).toHaveBeenCalledWith('chart.dashboard', null);
  });

  it('FR-CHART-016 AC1: restores the saved symbol/timeframe and persists changes under chart.dashboard', async () => {
    saved.value = { identifier: 'NSE:RELIANCE', timeframe: '15m' };
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts.dashboard).toEqual({ identifier: 'NSE:RELIANCE', timeframe: '15m' });
    useChartContexts.getState().setTimeframe(DASHBOARD_CONTEXT, '1d');
    expect(writeUiState).toHaveBeenCalledWith('chart.dashboard', { identifier: 'NSE:RELIANCE', timeframe: '1d' });
  });

  it('FR-CHART-016 AC2: hydrating again (Home → Scanners → Home) keeps the in-memory state', async () => {
    await hydrateDashboard('1h');
    useChartContexts.getState().setSymbol(DASHBOARD_CONTEXT, 'NSE:SBIN');
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts.dashboard.identifier).toBe('NSE:SBIN');
    expect(readUiState).toHaveBeenCalledTimes(1);
  });

  it('a junk saved timeframe falls back to the shipped default', async () => {
    saved.value = { identifier: 'NSE:TCS', timeframe: '2h' };
    await hydrateDashboard('1d');
    expect(useChartContexts.getState().contexts.dashboard.timeframe).toBe('1h');
  });

  it('FR-CHART-001 AC1: contexts are independent and only the dashboard is persisted', async () => {
    await hydrateDashboard('1h');
    useChartContexts.getState().init('full', { identifier: 'NSE:TCS', timeframe: '5m' });
    useChartContexts.getState().setTimeframe('full', '1d');
    expect(useChartContexts.getState().contexts.dashboard.timeframe).toBe('1h');
    expect(writeUiState).not.toHaveBeenCalled();
  });

  it('showOnDashboard before Home has hydrated is applied on hydrate', async () => {
    showOnDashboard('BSE:SENSEX');
    await hydrateDashboard('1h');
    expect(useChartContexts.getState().contexts.dashboard.identifier).toBe('BSE:SENSEX');
  });
});
```

`frontend/src/chart/ui/ChartView.test.tsx`:

```tsx
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../../api/instruments', async () => {
  const f = await import('../../instruments/fixtures');
  return {
    instrumentByIdentifier: vi.fn(async (id: string) => f.ALL_IN_SCOPE.find((i) => `${i.exchange}:${i.tradingsymbol}` === id)),
    searchInstruments: vi.fn(async () => [f.SBIN]), instrumentsByTokens: vi.fn(async () => []),
  };
});

import { ChartView, type ChartViewProps } from './ChartView';
import { FakeAdapter } from '../testing/FakeAdapter';
import type { ChartDataApi } from '../ChartDataController';
import type { Bar } from '../types';
import type { CandlesPage } from '../../api/chart';
import { readySession } from '../../api/mock/fixtures';
import { ThemeProvider } from '../../app/theme';
import { colourTokens, type ThemeName } from '../../design/tokens';
import { tick } from '../../market/testing';
import { useInstruments } from '../../store/instrumentsStore';
import { useSessionStore } from '../../store/session';
import { useTicksStore } from '../../store/ticks';
import { NIFTY50, RELIANCE, SBIN } from '../../instruments/fixtures';

const DAY = 86_400_000;
const YESTERDAY_1515 = Date.UTC(2026, 9, 8, 9, 45);
const bars = (n: number, end = YESTERDAY_1515, step = 3_600_000): Bar[] =>
  Array.from({ length: n }, (_, i) => ({ time: end - (n - 1 - i) * step, open: 100, high: 105, low: 95, close: 101 + i, volume: 254000 }));
const page = (b: Bar[], extra: Partial<CandlesPage> = {}): CandlesPage =>
  ({ bars: b, complete: true, backfillJobId: '', needsDataPlan: false, offline: false, ...extra });

let adapter: FakeAdapter;
let api: { [K in keyof ChartDataApi]: ReturnType<typeof vi.fn> };
const view = (over: Partial<ChartViewProps> = {}, theme: ThemeName = 'light') => (
  <ThemeProvider mode={theme} followSystem={false}>
    <ChartView contextId="t" identifier="NSE:RELIANCE" timeframe="1h" onSymbolChange={vi.fn()} onTimeframeChange={vi.fn()}
      adapterFactory={() => adapter} dataApi={api as unknown as ChartDataApi} {...over} />
  </ThemeProvider>
);
const renderView = (over: Partial<ChartViewProps> = {}) => render(view(over));

beforeEach(() => {
  adapter = new FakeAdapter();
  api = {
    fetchHistory: vi.fn(async () => page(bars(5))),
    refreshRange: vi.fn(), openLive: vi.fn(async () => 'live-1'), closeLive: vi.fn(async () => undefined),
    onCandle: vi.fn(() => () => undefined), onBackfillProgress: vi.fn(() => () => undefined),
  };
  useTicksStore.setState({ latest: {} });
  useInstruments.setState({ byToken: {}, byId: {} });
  useSessionStore.setState({ session: readySession() });
});

describe('ChartView', () => {
  it('loads the resolved instrument with the FR-CHART-003 window for the timeframe', async () => {
    renderView({ timeframe: '15m' });
    await waitFor(() => expect(api.fetchHistory).toHaveBeenCalledWith(RELIANCE.token, '15minute', 0, 30));
    expect(screen.queryByTestId('chart-loading')).toBeNull();
  });

  it('shows a loading skeleton while loading', async () => {
    api.fetchHistory.mockReturnValue(new Promise(() => undefined));
    renderView();
    expect(await screen.findByTestId('chart-loading')).toBeInTheDocument();
  });

  it('FR-CHART-003 AC4: no data plan (session dataPlan "none") shows the message and makes no historical call', async () => {
    useSessionStore.setState({ session: readySession({ dataPlan: 'none' }) });
    renderView();
    expect(await screen.findByText('Charts need a Kite Connect data plan')).toBeInTheDocument();
    expect(api.fetchHistory).not.toHaveBeenCalled();
  });

  it('failure shows an inline error with Retry', async () => {
    api.fetchHistory.mockRejectedValueOnce(new Error('Network unreachable'));
    renderView();
    expect(await screen.findByText('Network unreachable')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));
    await waitFor(() => expect(api.fetchHistory).toHaveBeenCalledTimes(2));
  });

  it('NFR-14: offline badge on cached data', async () => {
    api.fetchHistory.mockResolvedValue(page(bars(3), { offline: true }));
    renderView();
    expect(await screen.findByText('Offline — cached data')).toBeInTheDocument();
  });

  it('no history: empty-state message', async () => {
    api.fetchHistory.mockResolvedValue(page([], { complete: true }));
    renderView();
    expect(await screen.findByText('No candles yet for this instrument')).toBeInTheDocument();
  });

  it('FR-CHART-011 AC1: legend shows the hovered candle, last candle otherwise; volume in L/Cr', async () => {
    renderView();
    const legend = await screen.findByTestId('chart-legend');
    await waitFor(() => expect(legend).toHaveTextContent('C 105.00'));
    expect(legend).toHaveTextContent('Vol 2.54L');
    act(() => adapter.emitCrosshair({ bar: { time: 1, open: 100, high: 110, low: 90, close: 99, volume: 0 }, previousClose: 100 }));
    const hovered = screen.getByTestId('chart-legend');
    for (const part of ['O 100.00', 'H 110.00', 'L 90.00', 'C 99.00', '-1.00 (-1.00%)']) expect(hovered).toHaveTextContent(part);
    expect(hovered.className).toContain('chart-legend--down');
  });

  it('FR-CHART-011 AC3: index chart has no volume legend', async () => {
    renderView({ identifier: 'NSE:NIFTY 50' });
    await waitFor(() => expect(adapter.spec?.hasVolume).toBe(false));
    expect(screen.getByTestId('chart-legend')).not.toHaveTextContent('Vol');
    expect(NIFTY50.isIndex).toBe(true);
  });

  it('FR-CHART-011 AC2: L and A toggles drive the price scale', async () => {
    renderView();
    fireEvent.click(await screen.findByRole('button', { name: 'L' }));
    expect(adapter.scale.log).toBe(true);
    expect(screen.getByRole('button', { name: 'L' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: 'A' }));
    expect(adapter.scale.auto).toBe(true);
  });

  it('FR-CHART-011 AC4: keyboard pan/zoom/Home on the focused chart', async () => {
    const { container } = renderView();
    const canvas = await screen.findByRole('region', { name: /Price chart/ });
    expect(container.querySelector('[data-shortcut-scope="chart"]')).toContainElement(canvas);
    fireEvent.keyDown(canvas, { key: 'ArrowLeft' });
    fireEvent.keyDown(canvas, { key: '+' });
    fireEvent.keyDown(canvas, { key: 'Home' });
    fireEvent.keyDown(canvas, { key: 'h', altKey: true }); // chart tools (P6) are not handled here
    expect(adapter.calls.filter((c) => /pan|zoom|scroll/.test(c))).toEqual(['pan:-10', 'zoom:1.25', 'scrollToLatest']);
  });

  it('FR-CHART-001 AC3 / FR-SHELL-011 AC2: a theme change re-applies the chart colours from useTheme().tokens', async () => {
    const { rerender } = renderView();
    await waitFor(() => expect(adapter.calls.filter((c) => c === 'setTheme')).toHaveLength(1));
    expect(adapter.theme?.background).toBe(colourTokens.light.bg);
    rerender(view({}, 'dark'));
    await waitFor(() => expect(adapter.calls.filter((c) => c === 'setTheme')).toHaveLength(2));
    expect(adapter.theme?.background).toBe(colourTokens.dark.bg);
    expect(adapter.theme?.up).toBe(colourTokens.dark.up);
  });

  it('market closed: stale last candle stays, cached quote shown, no phantom bar', async () => {
    renderView();
    await waitFor(() => expect(adapter.bars()).toHaveLength(5));
    act(() => useTicksStore.getState().applyBatch([tick(RELIANCE.token, 1290.5, 1280, { tsMs: YESTERDAY_1515 + DAY })]));
    const header = screen.getByRole('button', { name: 'Change symbol' });
    expect(within(header).getByText('1,290.50')).toBeInTheDocument();
    expect(adapter.bars()).toHaveLength(5);
    expect(adapter.bars().at(-1)!.time).toBe(YESTERDAY_1515);
    expect(adapter.calls).not.toContain('updateLastBar');
    expect(screen.queryByTestId('chart-loading')).toBeNull();
  });

  it('symbol search selection reports the new identifier', async () => {
    const onSymbolChange = vi.fn();
    renderView({ onSymbolChange });
    fireEvent.click(await screen.findByRole('button', { name: 'Change symbol' }));
    fireEvent.change(await screen.findByRole('combobox', { name: 'Search instruments' }), { target: { value: 'sb' } });
    fireEvent.click(await screen.findByRole('option', { name: /SBIN/ }));
    expect(onSymbolChange).toHaveBeenCalledWith(`NSE:${SBIN.tradingsymbol}`);
  });
});
```

`frontend/src/chart/ui/DashboardChart.test.tsx` (real mock backend; only the canvas adapter and the coordinator hooks are faked):

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../market/hooks', () => ({ useSubscription: vi.fn(), usePolled: () => false }));
vi.mock('../LightweightChartsAdapter', async () => ({ LightweightChartsAdapter: (await import('../testing/FakeAdapter')).FakeAdapter }));

import { DashboardChart } from './DashboardChart';
import { BackendProvider } from '../../api/BackendContext';
import { setBackend } from '../../api/current';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { __resetChartContexts } from '../../store/chartContextsStore';
import { useSessionStore } from '../../store/session';
import { useSettingsStore } from '../../store/settings';

let backend: MockBackend;
const renderDash = () => render(<BackendProvider backend={backend}><MemoryRouter><DashboardChart /></MemoryRouter></BackendProvider>);

beforeEach(() => {
  __resetChartContexts();
  backend = createMockBackend(demoSeed());
  setBackend(backend);
  useSessionStore.setState({ session: readySession() });
  useSettingsStore.getState().hydrate({});
});

describe('DashboardChart', () => {
  it('FR-HOME-001 AC1 / FR-CHART-004 default: NIFTY 50 at 1 Hour on first run', async () => {
    renderDash();
    expect(await screen.findByText('NIFTY 50')).toBeInTheDocument();
    expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('1h');
  });

  it('honours a changed Settings → Chart default timeframe (chart.default_timeframe) on first run', async () => {
    useSettingsStore.getState().hydrate({ 'chart.default_timeframe': '"day"' });
    renderDash();
    await waitFor(() => expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('1d'));
  });

  it('FR-CHART-016 AC1: restores RELIANCE at 15 Minutes from the chart.dashboard UI state', async () => {
    await backend.settings.setUiState('chart.dashboard', JSON.stringify({ identifier: 'NSE:RELIANCE', timeframe: '15m' }));
    renderDash();
    expect(await screen.findByText('RELIANCE')).toBeInTheDocument();
    expect((screen.getByLabelText('Timeframe') as HTMLSelectElement).value).toBe('15m');
  });
});
```

- [ ] **Step 2: Run the tests to check they fail**

Run: `cd frontend && npx vitest run src/api/uiState.test.ts src/store/chartContextsStore.test.ts src/chart/ui/ChartView.test.tsx src/chart/ui/DashboardChart.test.tsx`
Expected: FAIL with `Failed to resolve import "./uiState"` / `"./chartContextsStore"` / `"./ChartView"` / `"./DashboardChart"`.

- [ ] **Step 3: Implement**

`frontend/src/api/uiState.ts`:

```ts
import { currentBackend } from './current';

/** UI state lives in Go's window_state table (Settings.GetUiState/SetUiState, "" when absent); values are JSON. */
export async function readUiState<T>(key: string, fallback: T): Promise<T> {
  const raw = await currentBackend().settings.getUiState(key);
  if (!raw) return fallback;
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

export async function writeUiState<T>(key: string, value: T): Promise<void> {
  await currentBackend().settings.setUiState(key, JSON.stringify(value));
}
```

`frontend/src/store/chartContextsStore.ts`:

```ts
import { create } from 'zustand';
import { readUiState, writeUiState } from '../api/uiState';
import { parseTimeframe, type TimeframeId } from '../chart/timeframes';

export interface ChartContextState { identifier: string; timeframe: TimeframeId }
export const DASHBOARD_CONTEXT = 'dashboard';
/** Contract §E: UI-state key for the dashboard chart, JSON {identifier, timeframe}. */
export const DASHBOARD_UI_STATE_KEY = 'chart.dashboard';
export const DEFAULT_DASHBOARD_SYMBOL = 'NSE:NIFTY 50';
const PERSISTED: Readonly<Record<string, string>> = { [DASHBOARD_CONTEXT]: DASHBOARD_UI_STATE_KEY }; // Scalper slots join in P6

interface ChartContextsStore {
  contexts: Record<string, ChartContextState>;
  init(ctx: string, initial: ChartContextState): void;
  setSymbol(ctx: string, identifier: string): void;
  setTimeframe(ctx: string, timeframe: TimeframeId): void;
}

export const useChartContexts = create<ChartContextsStore>()((set, get) => {
  const update = (ctx: string, patch: Partial<ChartContextState>) => {
    const cur = get().contexts[ctx];
    if (!cur) return;
    const next = { ...cur, ...patch };
    set((s) => ({ contexts: { ...s.contexts, [ctx]: next } }));
    const key = PERSISTED[ctx];
    if (key) void writeUiState(key, next).catch(() => undefined);
  };
  return {
    contexts: {},
    init: (ctx, initial) => {
      if (get().contexts[ctx]) return;
      set((s) => ({ contexts: { ...s.contexts, [ctx]: initial } }));
    },
    setSymbol: (ctx, identifier) => update(ctx, { identifier }),
    setTimeframe: (ctx, timeframe) => update(ctx, { timeframe }),
  };
});

let hydrating: Promise<void> | null = null;
let pendingSymbol: string | null = null;

export function hydrateDashboard(defaultTimeframe: TimeframeId): Promise<void> {
  if (useChartContexts.getState().contexts[DASHBOARD_CONTEXT]) return Promise.resolve();
  hydrating ??= readUiState<Partial<ChartContextState> | null>(DASHBOARD_UI_STATE_KEY, null)
    .catch(() => null)
    .then((saved) => {
      const store = useChartContexts.getState();
      store.init(DASHBOARD_CONTEXT, saved && typeof saved.identifier === 'string'
        ? { identifier: saved.identifier, timeframe: parseTimeframe(saved.timeframe) }
        : { identifier: DEFAULT_DASHBOARD_SYMBOL, timeframe: defaultTimeframe });
      if (pendingSymbol) { store.setSymbol(DASHBOARD_CONTEXT, pendingSymbol); pendingSymbol = null; }
    })
    .finally(() => { hydrating = null; });
  return hydrating;
}

/** Ticker pills / watchlist rows: load an instrument into the dashboard chart (even before Home has mounted). */
export function showOnDashboard(identifier: string): void {
  const store = useChartContexts.getState();
  if (store.contexts[DASHBOARD_CONTEXT]) store.setSymbol(DASHBOARD_CONTEXT, identifier);
  else pendingSymbol = identifier;
}

export function __resetChartContexts(): void {
  useChartContexts.setState({ contexts: {} });
  hydrating = null;
  pendingSymbol = null;
}
```

`frontend/src/chart/ui/ChartLegend.tsx`:

```tsx
import { formatCompact, formatPercent, formatPrice, formatSigned } from '../../i18n/format';
import type { LegendData } from '../types';

export function ChartLegend({ data, hasVolume, decimals }: { data: LegendData | null; hasVolume: boolean; decimals: number }) {
  if (!data) return <div className="chart-legend" data-testid="chart-legend" />;
  const { bar, previousClose } = data;
  const up = bar.close >= bar.open;
  const change = previousClose !== null && previousClose > 0 ? bar.close - previousClose : null;
  const p = (v: number) => formatPrice(v, decimals);
  return (
    <div className={`chart-legend chart-legend--${up ? 'up' : 'down'}`} data-testid="chart-legend">
      <span>O <b>{p(bar.open)}</b></span>
      <span>H <b>{p(bar.high)}</b></span>
      <span>L <b>{p(bar.low)}</b></span>
      <span>C <b>{p(bar.close)}</b></span>
      {change !== null && previousClose !== null && (
        <span><b>{formatSigned(change, decimals)} ({formatPercent((change / previousClose) * 100)})</b></span>
      )}
      {hasVolume && <span className="chart-legend__volume">Vol <b>{formatCompact(bar.volume)}</b></span>}
    </div>
  );
}
```

`frontend/src/chart/ui/ScaleToggles.tsx` (text toggles, so plain buttons with `aria-pressed` rather than icon buttons):

```tsx
export function ScaleToggles({ auto, log, onAuto, onLog }: { auto: boolean; log: boolean; onAuto(): void; onLog(): void }) {
  return (
    <div className="chart-scale-toggles" role="group" aria-label="Price scale">
      <button type="button" aria-pressed={auto} title="Auto-scale (A)" onClick={onAuto}>A</button>
      <button type="button" aria-pressed={log} title="Logarithmic scale (L)" onClick={onLog}>L</button>
    </div>
  );
}
```

`frontend/src/chart/ui/ChartView.tsx`:

```tsx
import { useEffect, useReducer, useRef, useState, type KeyboardEvent } from 'react';
import { Button } from '../../components/Button';
import { Skeleton } from '../../components/Skeleton';
import { closeLive, fetchHistory, onBackfillProgress, onCandle, openLive, refreshRange } from '../../api/chart';
import { useTheme } from '../../app/theme';
import { useSessionStore } from '../../store/session';
import { useResolvedInstrument } from '../../store/instrumentsStore';
import { instrumentId, tickRupees } from '../../instruments/naming';
import type { ChartAdapter } from '../ChartAdapter';
import { LightweightChartsAdapter } from '../LightweightChartsAdapter';
import { ChartDataController, INITIAL_CHART_STATE, NO_DATA_PLAN_MESSAGE, type ChartDataApi, type ChartLoadState } from '../ChartDataController';
import { precisionForTick } from '../format';
import { readChartTheme } from '../theme';
import type { TimeframeId } from '../timeframes';
import type { LegendData } from '../types';
import { ChartHeader } from './ChartHeader';
import { ChartLegend } from './ChartLegend';
import { ScaleToggles } from './ScaleToggles';
import { SymbolSearchDialog } from './SymbolSearchDialog';
import './chart.css';

export interface ChartViewProps {
  contextId: string;
  identifier: string;
  timeframe: TimeframeId;
  onSymbolChange(identifier: string): void;
  onTimeframeChange(tf: TimeframeId): void;
  onOpenFullView?: () => void;
  onBack?: () => void;
  maxStudies?: number;
  adapterFactory?: (el: HTMLElement) => ChartAdapter;
  dataApi?: ChartDataApi;
}

const defaultDataApi: ChartDataApi = { fetchHistory, refreshRange, openLive, closeLive, onCandle, onBackfillProgress };

export function ChartView(props: ChartViewProps) {
  const { contextId, identifier, timeframe } = props;
  const hostRef = useRef<HTMLDivElement>(null);
  const [adapter, setAdapter] = useState<ChartAdapter | null>(null);
  const [controller, setController] = useState<ChartDataController | null>(null);
  const [state, setState] = useState<ChartLoadState>(INITIAL_CHART_STATE);
  const [hover, setHover] = useState<LegendData | null>(null);
  const [scale, setScale] = useState({ log: false, auto: true });
  const [searchOpen, setSearchOpen] = useState(false);
  const [, bump] = useReducer((x: number) => x + 1, 0);
  const { instrument, error: resolveError } = useResolvedInstrument(identifier);
  // "No data plan" comes from the session (contract §D), never from feed.state.
  const noPlan = useSessionStore((s) => s.session?.dataPlan === 'none');
  const { tokens } = useTheme();

  useEffect(() => {
    const el = hostRef.current!;
    const a = props.adapterFactory ? props.adapterFactory(el) : new LightweightChartsAdapter(el, { maxStudies: props.maxStudies });
    const c = new ChartDataController(a, props.dataApi ?? defaultDataApi, setState, bump);
    const offCross = a.onCrosshairMove(setHover);
    setAdapter(a);
    setController(c);
    return () => { offCross(); c.dispose(); a.destroy(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- one adapter per mounted view
  }, []);

  useEffect(() => { adapter?.setTheme(readChartTheme(tokens)); }, [adapter, tokens]);

  useEffect(() => {
    if (!controller || !instrument || noPlan) return;
    setHover(null);
    void controller.load(instrument, timeframe);
  }, [controller, instrument, timeframe, noPlan]);

  const bars = adapter?.bars() ?? [];
  const legend: LegendData | null = hover ?? (bars.length > 0
    ? { bar: bars[bars.length - 1], previousClose: bars.length > 1 ? bars[bars.length - 2].close : null }
    : null);
  const decimals = instrument ? precisionForTick(tickRupees(instrument)) : 2;

  const toggleAuto = () => { if (!adapter) return; adapter.setPriceScale({ auto: true }); setScale(adapter.priceScaleState()); };
  const toggleLog = () => { if (!adapter) return; adapter.setPriceScale({ log: !adapter.priceScaleState().log }); setScale(adapter.priceScaleState()); };

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!adapter || e.altKey || e.ctrlKey || e.metaKey) return;
    switch (e.key) {
      case 'ArrowLeft': adapter.panBars(-10); break;
      case 'ArrowRight': adapter.panBars(10); break;
      case '+': case '=': adapter.zoom(1.25); break;
      case '-': adapter.zoom(0.8); break;
      case 'Home': adapter.scrollToLatest(); break;
      case 'a': case 'A': toggleAuto(); break;
      case 'l': case 'L': toggleLog(); break;
      default: return;
    }
    e.preventDefault();
  };

  const status = noPlan ? 'no_data_plan' : state.status;
  return (
    <div className="chart-view" data-context={contextId} data-shortcut-scope="chart">
      <ChartHeader contextId={contextId} instrument={instrument} timeframe={timeframe}
        refreshing={state.refreshing} refreshDisabled={status !== 'ready'}
        onOpenSearch={() => setSearchOpen(true)} onRefresh={() => void controller?.refresh()}
        onTimeframe={props.onTimeframeChange} onOpenFullView={props.onOpenFullView} onBack={props.onBack} />
      <div className="chart-view__body">
        <div ref={hostRef} className="chart-view__canvas" tabIndex={0} role="region"
          aria-label="Price chart. Arrow keys pan, plus and minus zoom, Home shows the latest candle, A auto-scale, L log scale."
          onKeyDown={onKeyDown} />
        <ChartLegend data={status === 'ready' ? legend : null} hasVolume={!!instrument && !instrument.isIndex} decimals={decimals} />
        <ScaleToggles auto={scale.auto} log={scale.log} onAuto={toggleAuto} onLog={toggleLog} />
        {state.offline && status === 'ready' && <span className="chart-view__badge">Offline — cached data</span>}
        {status === 'ready' && state.message && <span className="chart-view__badge" role="alert">{state.message}</span>}
        {status === 'loading' && (
          <div className="chart-view__overlay" data-testid="chart-loading"><Skeleton rows={6} label="Loading candles" /></div>
        )}
        {status === 'no_data_plan' && <div className="chart-view__overlay" role="status">{NO_DATA_PLAN_MESSAGE}</div>}
        {status === 'empty' && <div className="chart-view__overlay" role="status">{state.message}</div>}
        {status === 'error' && (
          <div className="chart-view__overlay" role="alert">
            <span>{state.message}</span>
            <Button onClick={() => void controller?.retry()}>Retry</Button>
          </div>
        )}
        {resolveError && <div className="chart-view__overlay" role="alert">{resolveError}</div>}
      </div>
      <SymbolSearchDialog open={searchOpen} onClose={() => setSearchOpen(false)}
        onSelect={(i) => props.onSymbolChange(instrumentId(i))} />
    </div>
  );
}
```

`frontend/src/chart/ui/DashboardChart.tsx`:

```tsx
import { useEffect } from 'react';
import { Skeleton } from '../../components/Skeleton';
import { useSetting } from '../../store/settings';
import { DASHBOARD_CONTEXT, hydrateDashboard, useChartContexts } from '../../store/chartContextsStore';
import { parseTimeframe } from '../timeframes';
import { ChartView } from './ChartView';

export function DashboardChart() {
  // Contract §E: Settings → Chart default timeframe, stored in Kite form ("60minute").
  const [defaultSetting] = useSetting<string>('chart.default_timeframe', '60minute');
  const defaultTf = parseTimeframe(defaultSetting);
  const ctx = useChartContexts((s) => s.contexts[DASHBOARD_CONTEXT]);
  useEffect(() => { void hydrateDashboard(defaultTf); }, [defaultTf]);
  if (!ctx) return <div className="chart-view"><Skeleton rows={6} label="Loading chart" /></div>;
  return (
    <ChartView contextId={DASHBOARD_CONTEXT} identifier={ctx.identifier} timeframe={ctx.timeframe}
      onSymbolChange={(id) => useChartContexts.getState().setSymbol(DASHBOARD_CONTEXT, id)}
      onTimeframeChange={(tf) => useChartContexts.getState().setTimeframe(DASHBOARD_CONTEXT, tf)} />
  );
}
```

In `frontend/src/views/home/slots.tsx` (P0-4 Task 12) add `import { DashboardChart } from '../../chart/ui/DashboardChart';` and replace the body of `HomeChartArea` (keep its doc comment's intent, now fulfilled):

```tsx
export function HomeChartArea() {
  return (
    <section className="card home-slot home-slot--chart" aria-label={homeCopy.chartArea}>
      <DashboardChart />
    </section>
  );
}
```

Append to `frontend/src/chart/ui/chart.css`:

```css
.home-slot--chart { display: flex; flex-direction: column; min-height: 420px; padding: 0; overflow: hidden; }
.home-slot--chart > .chart-view { flex: 1; }
```

- [ ] **Step 4: Run the tests to check they pass**

Run: `cd frontend && npx vitest run src/api/uiState.test.ts src/store/chartContextsStore.test.ts src/chart/ui && npm run typecheck && npx eslint src/chart src/store src/api`
Expected: PASS (2 + 6 + 13 + 3 tests, plus the earlier ui tests). Typecheck and ESLint exit 0. The ESLint fence must not flag `ChartView.tsx`: it imports `LightweightChartsAdapter`, not `lightweight-charts`.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/uiState.ts frontend/src/api/uiState.test.ts frontend/src/store/chartContextsStore.ts frontend/src/store/chartContextsStore.test.ts frontend/src/chart/ui frontend/src/views/home/slots.tsx
git commit -m "$(cat <<'EOF'
feat(chart): ChartView with legend, scale toggles, keyboard nav, load states; dashboard chart in the Home slot

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---