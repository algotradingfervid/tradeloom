import { act, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { HoldingRow } from '../../api/backend';
import { demoSeed, emptySeed, FIXED_NOW, holdingA, holdingB, holdingsOf, kiteOrder, tickFixture } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { renderApp } from '../../test/render';

const cells = (row: HTMLElement) => within(row).getAllByRole('cell').slice(1).map((c) => c.textContent);

// P0-3 Ruling 38: an MCX holding is kept, flagged and counted in totals.
const goldm: HoldingRow = {
  exchange: 'MCX', symbol: 'GOLDM', token: 53505799, qty: 1, avgPaise: 7_000_000, ltpPaise: 7_100_000, prevClosePaise: 7_050_000,
  investedPaise: 7_000_000, currentPaise: 7_100_000, pnlPaise: 100_000, netChangeBp: 143, dayChangePaise: 50_000, dayChangeBp: 71, outOfScope: true,
};

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
  it('down values carry a minus sign, not colour alone', async () => {
    renderApp({ route: '/holdings' });
    const tcs = cells(await screen.findByRole('row', { name: 'TCS' }));
    expect(tcs.slice(5)).toEqual(['-₹50.00', '-5.00%', '-₹50.00']);
  });
  it('updates LTP-driven values live from ticks', async () => {
    const { backend } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    act(() => backend.emit('tick.batch', [tickFixture(holdingA.token, 120, 105)]));
    expect(cells(screen.getByRole('row', { name: 'RELIANCE' }))[4]).toBe('₹1,200.00');
    expect(screen.getAllByRole('rowgroup')[2]).toHaveTextContent('₹2,150.00');
  });
  it('carry-forward: a persisted quote older than the session is not shown as live', async () => {
    const { backend } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    const loggedInAtMs = backend.state.session.loggedInAtMs;
    act(() => backend.emit('tick.batch', [{ ...tickFixture(holdingA.token, 150, 105), tsMs: loggedInAtMs - 60_000 }]));
    expect(cells(screen.getByRole('row', { name: 'RELIANCE' }))[2]).toBe('110.00');
    act(() => backend.emit('tick.batch', [{ ...tickFixture(holdingA.token, 120, 105), tsMs: FIXED_NOW }]));
    expect(cells(screen.getByRole('row', { name: 'RELIANCE' }))[2]).toBe('120.00');
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
  it('ignores a holdings.changed snapshot older than the one shown', async () => {
    const { backend } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    act(() => backend.emit('holdings.changed', holdingsOf([holdingB], 1)));
    expect(screen.getByRole('row', { name: 'RELIANCE' })).toBeInTheDocument();
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
  it('searches by symbol or exchange and reports matches to the list page (Task 5 carry-forward)', async () => {
    const { user } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    await user.type(screen.getByRole('searchbox', { name: 'Search holdings' }), 'tcs');
    expect(screen.queryByRole('row', { name: 'RELIANCE' })).toBeNull();
    expect(screen.getByRole('row', { name: 'TCS' })).toBeInTheDocument();
    await user.clear(screen.getByRole('searchbox', { name: 'Search holdings' }));
    await user.type(screen.getByRole('searchbox', { name: 'Search holdings' }), 'zzz');
    expect(screen.getByText('No matches for “zzz”')).toBeInTheDocument();
  });
  it('P0-3 Ruling 38: an out-of-scope holding is shown, flagged as view-only and counted in totals', async () => {
    const backend = createMockBackend({ ...demoSeed(), holdings: holdingsOf([holdingA, holdingB, goldm]) });
    renderApp({ route: '/holdings', backend });
    const row = await screen.findByRole('row', { name: 'GOLDM' });
    expect(row).toHaveTextContent('View only');
    expect(within(row).queryAllByRole('button')).toHaveLength(0);
    const totals = screen.getAllByRole('rowgroup')[2];
    expect(totals).toHaveTextContent('₹72,000.00');
    expect(totals).toHaveTextContent('₹73,050.00');
  });
  it('Ruling 96 I2: the "View only" flag is never clipped by the name truncation and is in the cell title', async () => {
    const backend = createMockBackend({ ...demoSeed(), holdings: holdingsOf([holdingA, goldm]) });
    renderApp({ route: '/holdings', backend });
    const row = await screen.findByRole('row', { name: 'GOLDM' });
    const flag = within(row).getByText('View only');
    expect(flag.closest('.cell-truncate')).toBeNull();
    expect(row.querySelector('[title="GOLDM · View only — Outside Tradeloom’s equity scope: shown for completeness and counted in the totals."]')).not.toBeNull();
    expect(within(row).getByText('GOLDM')).toHaveClass('instrument__name');
    expect(within(screen.getByRole('row', { name: 'RELIANCE' })).getByText('RELIANCE').closest('[title="RELIANCE"]')).not.toBeNull();
  });
  it('order.update only toasts its terminal status ("Order RELIANCE: Complete") and does not re-read (P0-3 Ruling 71)', async () => {
    const { backend } = renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    const before = backend.callsTo('portfolio.holdings').length;
    act(() => backend.emit('order.update', kiteOrder({ status: 'COMPLETE', filled_quantity: 10, average_price: 111 })));
    await act(async () => {});
    expect(screen.getByText('Order RELIANCE: Complete')).toBeInTheDocument();
    expect(backend.callsTo('portfolio.holdings').length).toBe(before);
  });
  it('shows no Buy/Sell order buttons', async () => {
    renderApp({ route: '/holdings' });
    await screen.findByRole('row', { name: 'RELIANCE' });
    expect(screen.queryByRole('button', { name: /^(buy|sell|b|s)\b/i })).toBeNull();
  });
});
