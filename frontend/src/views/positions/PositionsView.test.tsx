import { act, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { PositionRow } from '../../api/backend';
import { demoSeed, emptySeed, FIXED_NOW, positionClosed, positionOpen, positionsOf, tickFixture } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { setRunStartedMs } from '../../market/quotes';
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

  // Carry-forwards (p04-carry-forwards.md, Task 11 / Ruling 101).
  const nifty: PositionRow = {
    key: 'net|NFO|NIFTY26OCT25000CE|NRML', exchange: 'NFO', symbol: 'NIFTY26OCT25000CE', token: 12_345_678, product: 'NRML',
    qty: 50, avgPaise: 10_000, ltpPaise: 10_200, pnlPaise: 10_000, netChangeBp: 200, multiplier: 1, closed: false, outOfScope: true,
  };
  const usdinr: PositionRow = {
    key: 'net|CDS|USDINR26OCTFUT|NRML', exchange: 'CDS', symbol: 'USDINR26OCTFUT', token: 412_675, product: 'NRML',
    qty: 1, avgPaise: 8_350, ltpPaise: 8_360, pnlPaise: 1_000, netChangeBp: 12, multiplier: 1000, closed: false, outOfScope: true,
  };
  const withRows = (net: PositionRow[]) => createMockBackend({ ...demoSeed(), positions: positionsOf(net, []) });

  it('P0-3 Ruling 38: an out-of-scope position is shown read-only, flagged "View only" (never clipped) and counted in the total', async () => {
    renderApp({ route: '/positions', backend: withRows([positionOpen, positionClosed, nifty]) });
    const row = await screen.findByRole('row', { name: 'NIFTY26OCT25000CE' });
    const flag = within(row).getByText('View only');
    expect(flag.closest('.cell-truncate')).toBeNull();
    expect(row.querySelector('.cell-truncate')).toBeNull();
    expect(row.querySelector('[title^="NIFTY26OCT25000CE · View only — "]')).not.toBeNull();
    expect(within(row).queryAllByRole('button')).toHaveLength(0);
    expect(screen.getAllByRole('rowgroup')[2]).toHaveTextContent('Total P&L+₹174.50');
  });
  it('Ruling 96 M1: a 4-decimal CDS/BCD row keeps Go’s snapshot P&L instead of the live overlay', async () => {
    const { backend } = renderApp({ route: '/positions', backend: withRows([positionOpen, usdinr]) });
    const row = await screen.findByRole('row', { name: 'USDINR26OCTFUT' });
    expect(within(row).getAllByRole('cell')[5]).toHaveTextContent('+₹10.00');
    act(() => backend.emit('tick.batch', [tickFixture(usdinr.token, 83.7525, 83.5)]));
    expect(within(screen.getByRole('row', { name: 'USDINR26OCTFUT' })).getAllByRole('cell')[5]).toHaveTextContent('+₹10.00');
    expect(screen.getAllByRole('rowgroup')[2]).toHaveTextContent('Total P&L+₹130.00');
  });
  it('carry-forward (Ruling 93): a persisted quote from a previous run is not applied as live', async () => {
    setRunStartedMs(FIXED_NOW);
    const { backend } = renderApp({ route: '/positions' });
    await screen.findByRole('row', { name: 'INFY' });
    act(() => backend.emit('tick.batch', [{ ...tickFixture(positionOpen.token, 150, 110), tsMs: FIXED_NOW - 60_000 }]));
    expect(within(screen.getByRole('row', { name: 'INFY' })).getAllByRole('cell')[4]).toHaveTextContent('112.00');
    act(() => backend.emit('tick.batch', [{ ...tickFixture(positionOpen.token, 113, 110), tsMs: FIXED_NOW }]));
    expect(within(screen.getByRole('row', { name: 'INFY' })).getAllByRole('cell')[4]).toHaveTextContent('113.00');
  });
  it('down values carry a minus sign, not colour alone', async () => {
    renderApp({ route: '/positions' });
    const row = await screen.findByRole('row', { name: 'TATAMOTORS' });
    expect(within(row).getAllByRole('cell')[5]).toHaveTextContent('-₹45.50');
  });
  it('searches by symbol and reports matches to the list page (Task 5 carry-forward)', async () => {
    const { user } = renderApp({ route: '/positions' });
    await screen.findByRole('row', { name: 'INFY' });
    await user.type(screen.getByRole('searchbox', { name: 'Search positions' }), 'zzz');
    expect(screen.getByText('No matches for “zzz”')).toBeInTheDocument();
  });
  it('Refresh asks Go to re-fetch from Kite (Portfolio.Refresh("positions"))', async () => {
    const { backend, user } = renderApp({ route: '/positions' });
    await screen.findByRole('row', { name: 'INFY' });
    await user.click(screen.getByRole('button', { name: 'Refresh positions' }));
    expect(backend.callsTo('portfolio.refresh').map((c) => c.args)).toEqual([['positions']]);
  });
});
