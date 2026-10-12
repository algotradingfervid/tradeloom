import { act, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { demoSeed, emptySeed, FIXED_NOW, holdingA, holdingB, holdingsOf, kiteOrder, tickFixture } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { setRunStartedMs } from '../../market/quotes';
import { renderApp } from '../../test/render';
import { watchlistItemOf } from '../../api/mock/market';
import { SBIN } from '../../instruments/fixtures';
import { DASHBOARD_CONTEXT, useChartContexts } from '../../store/chartContextsStore';

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
  it('AC1: a loss carries a minus sign in the text, not colour alone', async () => {
    const seed = demoSeed();
    seed.holdings = holdingsOf([holdingB]);
    renderApp({ backend: createMockBackend(seed) });
    const c = await card();
    await within(c).findByText('₹950.00');
    expect(c).toHaveTextContent('-₹50.00 (-5.00%)');
    expect(c).toHaveTextContent("Day's change-₹50.00");
  });
  it('AC2: a tick moves Current value immediately', async () => {
    const { backend } = renderApp();
    const c = await card();
    await within(c).findByText('₹2,050.00');
    act(() => backend.emit('tick.batch', [tickFixture(holdingA.token, 120, 105)]));
    expect(within(c).getByText('₹2,150.00')).toBeInTheDocument();
  });
  it('Ruling 93: a quote stamped before this run started does not move the totals', async () => {
    setRunStartedMs(FIXED_NOW);
    const { backend } = renderApp();
    const c = await card();
    await within(c).findByText('₹2,050.00');
    const old = { ...tickFixture(holdingA.token, 120, 105), tsMs: FIXED_NOW - 60_000 };
    act(() => backend.emit('tick.batch', [old]));
    expect(within(c).getByText('₹2,050.00')).toBeInTheDocument();
    expect(within(c).queryByText('₹2,150.00')).not.toBeInTheDocument();
  });
  it('P0-3 Ruling 38: out-of-scope holdings are counted in the totals', async () => {
    const seed = demoSeed();
    seed.holdings = holdingsOf([{ ...holdingA, outOfScope: true }, holdingB]);
    renderApp({ backend: createMockBackend(seed) });
    const c = await card();
    expect(await within(c).findByText('₹2,050.00')).toBeInTheDocument();
    expect(c).toHaveTextContent('Investment₹2,000.00');
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
  it('re-reads every 5 minutes; fills arrive as holdings.changed, not as a re-read on order.update (P0-3 Ruling 71)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { backend } = renderApp();
    const c = await card();
    await within(c).findByText('₹2,050.00');
    const calls = () => backend.callsTo('portfolio.holdings').length;
    const start = calls();
    await act(async () => { vi.advanceTimersByTime(300_000); });
    expect(calls()).toBe(start + 1);
    await act(async () => {
      backend.emit('order.update', kiteOrder({ order_id: '261012000000002', tradingsymbol: 'INFY', product: 'MIS', transaction_type: 'SELL', status: 'COMPLETE' }));
      backend.emit('order.update', kiteOrder({ status: 'COMPLETE', filled_quantity: 10 })); // CNC
    });
    expect(calls()).toBe(start + 1);
    // Go publishes the post-fill holdings (here: RELIANCE 20 @ 100, LTP 110); the card shows them without a read.
    const filled = { ...holdingA, qty: 20, investedPaise: 200_000, currentPaise: 220_000, pnlPaise: 20_000, dayChangePaise: 10_000 };
    await act(async () => { backend.emit('holdings.changed', holdingsOf([filled, holdingB], FIXED_NOW)); });
    expect(within(c).getByText('₹3,150.00')).toBeInTheDocument();
    expect(calls()).toBe(start + 1);
  });
  it('Task 10 M8: a failed background re-read keeps the shown totals and adds a non-blocking notice', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { backend, user } = renderApp();
    const c = await card();
    await within(c).findByText('₹2,050.00');
    backend.failNext('portfolio.holdings', 'Network unavailable');
    await act(async () => { vi.advanceTimersByTime(300_000); });
    expect(within(c).getByText('₹2,050.00')).toBeInTheDocument();
    expect(within(c).queryByRole('alert')).not.toBeInTheDocument();
    expect(within(c).getByRole('status')).toHaveTextContent('Network unavailable');
    await user.click(within(c).getByRole('button', { name: 'Retry' }));
    // Ruling 105 M2: the live region stays mounted; only its contents go.
    await vi.waitFor(() => expect(within(c).getByRole('status')).toBeEmptyDOMElement());
    expect(within(c).getByText('₹2,050.00')).toBeInTheDocument();
  });
  it('shows no order buttons', async () => {
    renderApp();
    await within(await card()).findByText('₹2,050.00');
    expect(screen.queryByRole('button', { name: /^(buy|sell|cancel|modify|exit|convert|b|s)\b/i })).not.toBeInTheDocument();
  });
});

describe('FR-HOME-001 dashboard layout', () => {
  it('AC2: a failed holdings request shows an error only in the card; the other areas still render', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('portfolio.holdings', 'Network unavailable');
    const { user } = renderApp({ backend });
    const c = await card();
    expect(await within(c).findByRole('alert')).toHaveTextContent('Network unavailable');
    // Ruling 105: no data held → the alert only, no stale-data notice.
    expect(within(c).getByRole('status')).toBeEmptyDOMElement();
    expect(screen.getByRole('region', { name: 'Main chart' })).toBeInTheDocument();
    expect(screen.getByRole('complementary', { name: 'Watchlists and widgets' })).toBeInTheDocument();
    await user.click(within(c).getByRole('button', { name: 'Retry' }));
    expect(await within(c).findByText('₹2,050.00')).toBeInTheDocument();
  });
  it('leaves the P0-5 watchlist slot inside the right column', async () => {
    renderApp();
    const right = await screen.findByRole('complementary', { name: 'Watchlists and widgets' });
    // P0-5 Task 16 renames the slot's region to its panel ("Watchlist"); either name is the watchlist slot.
    expect(within(right).getByRole('region', { name: /^Watchlists?$/ })).toBeInTheDocument();
  });
});

describe('FR-WATCH-001 watchlist panel on Home (P0-5 Task 16)', () => {
  it('clicking a watchlist row shows that instrument on the dashboard chart', async () => {
    const backend = createMockBackend(demoSeed());
    backend.state.watchlists = { lists: [{ id: 'wl-1', name: 'Watchlist 1', items: [watchlistItemOf(SBIN)] }], activeId: 'wl-1' };
    const { user } = renderApp({ backend });
    const panel = await screen.findByRole('region', { name: 'Watchlist' });
    await vi.waitFor(() => expect(useChartContexts.getState().contexts[DASHBOARD_CONTEXT]?.identifier).toBe('NSE:NIFTY 50'));
    await user.click(await within(panel).findByRole('button', { name: 'Show SBIN on chart' }));
    expect(useChartContexts.getState().contexts[DASHBOARD_CONTEXT]?.identifier).toBe('NSE:SBIN');
  });
});
