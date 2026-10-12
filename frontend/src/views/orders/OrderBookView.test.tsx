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

  // Carry-forwards (p04-carry-forwards.md, Task 11 / Ruling 101).
  it('Task 10 M8: a failed background re-read keeps the shown orders and shows a non-blocking notice', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const seed = demoSeed();
    seed.status.feed = { ...seed.status.feed, state: 'offline' };
    const backend = createMockBackend(seed);
    renderApp({ route: '/orders', backend });
    await screen.findByRole('heading', { name: 'Open (1)' });
    backend.failNext('portfolio.orderBook', 'Network unavailable');
    await act(async () => {
      vi.advanceTimersByTime(30_000);
    });
    expect(screen.getByRole('row', { name: 'RELIANCE 261012000000001' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull();
    expect(screen.getByText(/Couldn’t update the order book; showing the last data loaded\. Network unavailable/)).toBeInTheDocument();
  });
  it('P0-3 Ruling 38: an out-of-scope order is shown read-only, flagged "View only" (never clipped) and counted', async () => {
    const goldm = {
      ...ordersFixture[0], orderId: '261012000000010', exchange: 'MCX', symbol: 'GOLDM26OCTFUT', token: 53_505_799, outOfScope: true,
    };
    renderApp({ route: '/orders', backend: createMockBackend({ ...demoSeed(), book: bookOf([...ordersFixture, goldm]) }) });
    expect(await screen.findByRole('heading', { name: 'Open (2)' })).toBeInTheDocument();
    const row = screen.getByRole('row', { name: 'GOLDM26OCTFUT 261012000000010' });
    const flag = within(row).getByText('View only');
    expect(flag.closest('.cell-truncate')).toBeNull();
    expect(row.querySelector('[title^="GOLDM26OCTFUT · View only — "]')).not.toBeNull();
    expect(within(row).getAllByRole('cell')[2].querySelector('.cell-truncate')).toBeNull();
  });
  it('orders with the same time are ordered by order ID, newest first (Go orders.BuildBook)', async () => {
    const a = { ...ordersFixture[0], orderId: '261012000000011', symbol: 'SBIN', timeMs: FIXED_NOW };
    const b = { ...ordersFixture[0], orderId: '261012000000012', symbol: 'ITC', timeMs: FIXED_NOW };
    renderApp({ route: '/orders', backend: createMockBackend({ ...demoSeed(), book: bookOf([a, b]) }) });
    const later = await screen.findByRole('row', { name: 'ITC 261012000000012' });
    const earlier = screen.getByRole('row', { name: 'SBIN 261012000000011' });
    expect(later.compareDocumentPosition(earlier) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });
  it('Buy and Sell carry text, not colour alone; there are no order buttons (cancel, modify) in P0', async () => {
    renderApp({ route: '/orders' });
    const open = await screen.findByRole('row', { name: 'RELIANCE 261012000000001' });
    expect(within(open).getAllByRole('cell')[1]).toHaveTextContent('Buy');
    expect(within(open).queryAllByRole('button')).toHaveLength(0);
    expect(screen.queryByRole('button', { name: /^(buy|sell|cancel|modify|b|s)\b/i })).toBeNull();
  });
  it('empty state', async () => {
    renderApp({ route: '/orders', backend: createMockBackend({ ...demoSeed(), book: bookOf([]) }) });
    expect(await screen.findByText('No orders today')).toBeInTheDocument();
  });
});
