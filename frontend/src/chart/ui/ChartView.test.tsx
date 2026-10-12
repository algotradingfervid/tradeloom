import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

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
import type { CandleEvent, CandlesPage } from '../../api/chart';
import { BindingError } from '../../api/call';
import { setBackend } from '../../api/current';
import { demoSeed, readySession } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { ThemeProvider } from '../../app/theme';
import { colourTokens, type ThemeName } from '../../design/tokens';
import { tick } from '../../market/testing';
import { useInstruments } from '../../store/instrumentsStore';
import { resetOnLogout } from '../../store/reset';
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

  // ---- Task 10 carry-forwards (P0-5 progress.md: Rulings 109 I3, Task 4 review I1, Task 5 minors) ----

  it('carry-forward b: a refused OpenLive shows Go’s reason as a small badge while history still shows', async () => {
    api.openLive.mockRejectedValue(new BindingError('subscription_budget', 'The live subscription budget is full; candles are not live.'));
    renderView();
    const badge = await screen.findByText('The live subscription budget is full; candles are not live.');
    expect(badge).toHaveClass('chart-view__badge');
    expect(adapter.bars()).toHaveLength(5);
    expect(screen.getByTestId('chart-legend')).toHaveTextContent('C 105.00');
  });

  it('carry-forward d: after the pointer leaves (a copy of the last bar), live candles still reach the legend', async () => {
    let push!: (e: CandleEvent) => void;
    api.onCandle.mockImplementation((cb: (e: CandleEvent) => void) => { push = cb; return () => undefined; });
    renderView();
    const legend = await screen.findByTestId('chart-legend');
    await waitFor(() => expect(legend).toHaveTextContent('C 105.00'));
    const last = adapter.bars().at(-1)!;
    act(() => adapter.emitCrosshair({ bar: { ...last }, previousClose: 104 }));
    act(() => push({ token: RELIANCE.token, interval: '60minute', bar: { ...last, close: 107.5, high: 108 }, closed: false }));
    expect(screen.getByTestId('chart-legend')).toHaveTextContent('C 107.50');
    expect(screen.getByTestId('chart-legend')).toHaveTextContent('H 108.00');
    // Hovering an older candle keeps it in the legend while live candles arrive.
    const older = adapter.bars()[1];
    act(() => adapter.emitCrosshair({ bar: older, previousClose: adapter.bars()[0].close }));
    act(() => push({ token: RELIANCE.token, interval: '60minute', bar: { ...last, close: 109 }, closed: false }));
    expect(screen.getByTestId('chart-legend')).toHaveTextContent(`C ${older.close}.00`);
  });

  it('carry-forward a: logout clears the chart and keeps it idle; logging in again loads it', async () => {
    renderView();
    await waitFor(() => expect(adapter.bars()).toHaveLength(5));
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
    act(() => {
      useSessionStore.setState({ session: readySession({ state: 'logged_out', userId: '', sessionId: '' }) });
      resetOnLogout();
    });
    expect(adapter.bars()).toHaveLength(0);
    // The instrument store reset re-resolves the identifier; nothing loads while signed out.
    await act(async () => { await new Promise((r) => setTimeout(r, 20)); });
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId('chart-legend')).toBeEmptyDOMElement();
    act(() => useSessionStore.setState({ session: readySession() }));
    await waitFor(() => expect(api.fetchHistory).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByTestId('chart-legend')).toHaveTextContent('C 105.00'));
  });

  it('carry-forward a: a user switch through the bridge (reset, then ready(B)) reloads the chart', async () => {
    renderView();
    await waitFor(() => expect(adapter.bars()).toHaveLength(5));
    act(() => {
      resetOnLogout();
      useSessionStore.setState({ session: readySession({ userId: 'ZZ9999', sessionId: 'sess-2' }) });
    });
    await waitFor(() => expect(api.fetchHistory.mock.calls.length).toBeGreaterThanOrEqual(2));
    await waitFor(() => expect(adapter.bars()).toHaveLength(5));
    expect(screen.queryByTestId('chart-loading')).toBeNull();
  });

  it('carry-forward a: an answer dropped for a user change (controller reset, resetEpoch bumped) is reloaded for the new user', async () => {
    let answer!: (p: CandlesPage) => void;
    api.fetchHistory.mockImplementationOnce(() => new Promise<CandlesPage>((r) => { answer = r; }));
    renderView();
    await waitFor(() => expect(api.fetchHistory).toHaveBeenCalledTimes(1));
    // The user changes without a store reset: only the controller notices, when the old user's answer arrives.
    act(() => useSessionStore.setState({ session: readySession({ userId: 'ZZ9999', sessionId: 'sess-2' }) }));
    await act(async () => { answer(page(bars(2))); });
    await waitFor(() => expect(api.fetchHistory).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(adapter.bars()).toHaveLength(5));
    expect(screen.getByTestId('chart-legend')).toHaveTextContent('C 105.00');
  });
});

describe('ChartView leaks', () => {
  afterEach(() => { vi.useRealTimers(); setBackend(null); });

  it('unmount closes the live subscription and every event listener (mock backend openLiveIds)', async () => {
    const backend = createMockBackend(demoSeed());
    setBackend(backend);
    const { unmount } = render(view({ dataApi: undefined }));
    await waitFor(() => expect(backend.state.openLiveIds).toHaveLength(1));
    await waitFor(() => expect(adapter.bars().length).toBeGreaterThan(0));
    for (const n of ['candle.forming', 'candle.closed', 'backfill.progress'] as const) expect(backend.listenerCount(n)).toBe(1);
    unmount();
    await waitFor(() => expect(backend.state.openLiveIds).toEqual([]));
    for (const n of ['candle.forming', 'candle.closed', 'backfill.progress'] as const) expect(backend.listenerCount(n)).toBe(0);
    expect(adapter.destroyed).toBe(true);
  });

  it('unmount during a backfill wait leaves no timer behind', async () => {
    vi.useFakeTimers();
    api.fetchHistory.mockResolvedValue(page([], { complete: false, backfillJobId: 'bf-9' }));
    const { unmount } = renderView();
    for (let i = 0; i < 10 && api.fetchHistory.mock.calls.length === 0; i++) await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    expect(api.fetchHistory).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBeGreaterThan(0); // the 30 s backfill wait
    unmount();
    expect(vi.getTimerCount()).toBe(0);
    expect(api.closeLive).toHaveBeenCalledTimes(0); // closeLive runs once openLive's promise settles
    await act(async () => { await vi.advanceTimersByTimeAsync(0); });
    expect(api.closeLive).toHaveBeenCalledWith('live-1');
  });
});
