import { describe, expect, it } from 'vitest';
import type { HoldingsSnapshot } from '../api/backend';
import { startEventBridge } from '../api/events';
import { demoSeed, holdingA, holdingsOf, kiteOrder, readySession } from '../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../api/mock/index';
import { loadPortfolio, refreshPortfolio, usePortfolioStore } from './portfolio';
import { useSessionStore } from './session';
import { useToastStore } from './toasts';

function deferHoldings(backend: MockBackend) {
  let resolve!: (h: HoldingsSnapshot) => void;
  backend.portfolio.holdings = () => new Promise<HoldingsSnapshot>((r) => { resolve = r; });
  return (h: HoldingsSnapshot) => resolve(h);
}

function bridged() {
  const backend = createMockBackend(demoSeed());
  const stop = startEventBridge(backend, { navigate: () => {} });
  useSessionStore.getState().setSession(readySession());
  return { backend, stop };
}

describe('portfolio store', () => {
  it('Ruling 83-F M5: signing out clears the portfolio (account data)', () => {
    const { backend, stop } = bridged();
    backend.emit('holdings.changed', holdingsOf([holdingA], 1_000));
    expect(usePortfolioStore.getState().holdings.data.rows).toHaveLength(1);
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'idle', data: { rows: [] } });
    stop();
  });

  it('orders.changed and positions.changed replace the shown snapshot', () => {
    const { backend, stop } = bridged();
    backend.emit('orders.changed', { ...backend.state.book, asOfMs: 5 });
    backend.emit('positions.changed', { ...backend.state.positions, asOfMs: 5 });
    expect(usePortfolioStore.getState().orders).toMatchObject({ status: 'ready', data: { asOfMs: 5 } });
    expect(usePortfolioStore.getState().positions).toMatchObject({ status: 'ready', data: { asOfMs: 5 } });
    stop();
  });

  it('Ruling 96 I1: a cache read that resolves after logout does not repopulate the store', async () => {
    const { backend, stop } = bridged();
    const resolve = deferHoldings(backend);
    const read = loadPortfolio('holdings', backend);
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    resolve(holdingsOf([holdingA], 2_000));
    await read;
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'idle', loading: false, data: { rows: [] } });
    stop();
  });

  it('Ruling 96 I1: a cache read that resolves after a login as a different user is dropped', async () => {
    const { backend, stop } = bridged();
    const resolve = deferHoldings(backend);
    const read = loadPortfolio('holdings', backend);
    useSessionStore.getState().setSession(readySession({ userId: 'ZZ9999', sessionId: 'sess-9' }));
    resolve(holdingsOf([holdingA], 2_000));
    await read;
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'idle', loading: false, data: { rows: [] } });
    stop();
  });

  it('Ruling 96 I1: a refresh interrupted by logout neither re-reads nor reports an error', async () => {
    const { backend, stop } = bridged();
    let finish!: () => void;
    backend.portfolio.refresh = () => new Promise<void>((r) => { finish = r; });
    const refresh = refreshPortfolio('holdings', backend);
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    finish();
    await refresh;
    expect(backend.callsTo('portfolio.holdings')).toHaveLength(0);
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'idle', loading: false });
    stop();
  });

  it('Ruling 96 M5: order.update toasts once per order id and status', () => {
    const { backend, stop } = bridged();
    backend.emit('order.update', kiteOrder({ status: 'COMPLETE' }));
    backend.emit('order.update', kiteOrder({ status: 'COMPLETE' }));
    backend.emit('order.update', kiteOrder({ order_id: '261012000000009', status: 'COMPLETE' }));
    expect(useToastStore.getState().visible.map((t) => t.message)).toEqual(['Order RELIANCE: Complete', 'Order RELIANCE: Complete']);
    // The de-duplication memory is account data: it is cleared on sign-out.
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    backend.emit('order.update', kiteOrder({ status: 'COMPLETE' }));
    expect(useToastStore.getState().visible).toHaveLength(3);
    stop();
  });

  it('Ruling 96 M5: a REJECTED update is a warning toast; a non-terminal update shows nothing', () => {
    const { backend, stop } = bridged();
    backend.emit('order.update', kiteOrder({ status: 'OPEN' }));
    backend.emit('order.update', kiteOrder({ status: 'TRIGGER PENDING' }));
    expect(useToastStore.getState().visible).toEqual([]);
    backend.emit('order.update', kiteOrder({ status: 'REJECTED' }));
    expect(useToastStore.getState().visible.map((t) => [t.level, t.message])).toEqual([['warning', 'Order RELIANCE: Rejected']]);
    stop();
  });
});

describe('background re-reads (Ruling 103)', () => {
  function rejectingHoldings(backend: MockBackend) {
    let reject!: (e: unknown) => void;
    backend.portfolio.holdings = () => new Promise<HoldingsSnapshot>((_, r) => { reject = r; });
    return (reason: string) => reject({ code: 'error', reason });
  }

  it('a failed poll while data is shown records backgroundError; status stays ready; no toast; a second failure adds nothing; success clears it', async () => {
    const { backend, stop } = bridged();
    await loadPortfolio('holdings', backend);
    const toasts = useToastStore.getState().visible.length;
    backend.failNext('portfolio.holdings', 'Network unavailable');
    await loadPortfolio('holdings', backend, { background: true });
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'ready', loading: false, error: null, backgroundError: 'Network unavailable' });
    const firstSeq = usePortfolioStore.getState().holdings.backgroundErrorSeq;
    backend.failNext('portfolio.holdings', 'Network unavailable');
    await loadPortfolio('holdings', backend, { background: true });
    // Same reason, same single notice; the sequence moves so the notice is announced again (Ruling 105 M1).
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'ready', backgroundError: 'Network unavailable', backgroundErrorSeq: firstSeq + 1 });
    expect(useToastStore.getState().visible).toHaveLength(toasts);
    await loadPortfolio('holdings', backend, { background: true });
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'ready', backgroundError: null });
    stop();
  });

  it('a background poll leaves loading alone (no busy indicator)', async () => {
    const { backend, stop } = bridged();
    await loadPortfolio('holdings', backend);
    const fail = rejectingHoldings(backend);
    const read = loadPortfolio('holdings', backend, { background: true });
    expect(usePortfolioStore.getState().holdings.loading).toBe(false);
    fail('Network unavailable');
    await read;
    expect(usePortfolioStore.getState().holdings.loading).toBe(false);
    stop();
  });

  it('a poll that fails after logout is dropped: no notice, store idle', async () => {
    const { backend, stop } = bridged();
    await loadPortfolio('holdings', backend);
    const fail = rejectingHoldings(backend);
    const read = loadPortfolio('holdings', backend, { background: true });
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    fail('Network unavailable');
    await read;
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'idle', backgroundError: null, data: { rows: [] } });
    stop();
  });

  it('reset clears backgroundError', async () => {
    const { backend, stop } = bridged();
    await loadPortfolio('holdings', backend);
    backend.failNext('portfolio.holdings', 'Network unavailable');
    await loadPortfolio('holdings', backend, { background: true });
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    expect(usePortfolioStore.getState().holdings.backgroundError).toBeNull();
    stop();
  });
});

describe('failed reads (Ruling 105)', () => {
  it('a background failure then a foreground failure keeps the rows: status ready, the notice, no error state', async () => {
    const { backend, stop } = bridged();
    await loadPortfolio('holdings', backend);
    backend.failNext('portfolio.holdings', 'Network unavailable');
    await loadPortfolio('holdings', backend, { background: true });
    backend.failNext('portfolio.holdings', 'Gateway timeout');
    await loadPortfolio('holdings', backend);
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'ready', error: null, loading: false, backgroundError: 'Gateway timeout' });
    expect(usePortfolioStore.getState().holdings.data.rows.length).toBeGreaterThan(0);
    stop();
  });
  it('a first read that fails with no data held is the error state, with no notice', async () => {
    const { backend, stop } = bridged();
    backend.failNext('portfolio.holdings', 'Network unavailable');
    await loadPortfolio('holdings', backend);
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'error', error: 'Network unavailable', backgroundError: null, loading: false });
    backend.failNext('portfolio.holdings', 'Still down');
    await loadPortfolio('holdings', backend, { background: true });
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'error', error: 'Still down', backgroundError: null });
    stop();
  });
  it('deviation (b): a poll during a pending Refresh (loading) is skipped and does not supersede the in-flight read', async () => {
    const { backend, stop } = bridged();
    await loadPortfolio('holdings', backend);
    let finishKite!: () => void;
    backend.portfolio.refresh = () => new Promise<void>((r) => { finishKite = r; });
    const refreshing = refreshPortfolio('holdings', backend);
    expect(usePortfolioStore.getState().holdings.loading).toBe(true);
    const before = backend.callsTo('portfolio.holdings').length;
    await loadPortfolio('holdings', backend, { background: true });
    expect(backend.callsTo('portfolio.holdings')).toHaveLength(before);
    expect(usePortfolioStore.getState().holdings.loading).toBe(true);
    // The foreground read Refresh starts after Kite is still current (latestRequest was not bumped by the skipped poll).
    const resolve = deferHoldings(backend);
    finishKite();
    for (let i = 0; i < 10; i++) await Promise.resolve();
    const skipped = loadPortfolio('holdings', backend, { background: true });
    resolve(holdingsOf([holdingA], 9_000_000_000_000));
    await skipped;
    await refreshing;
    expect(usePortfolioStore.getState().holdings).toMatchObject({ loading: false, data: { asOfMs: 9_000_000_000_000 } });
    stop();
  });
  it('deviation (b): a background read that resolves during a Refresh’s Kite wait keeps loading true', async () => {
    const { backend, stop } = bridged();
    await loadPortfolio('holdings', backend);
    const resolve = deferHoldings(backend);
    const poll = loadPortfolio('holdings', backend, { background: true });
    backend.portfolio.refresh = () => new Promise<void>(() => {});
    void refreshPortfolio('holdings', backend);
    expect(usePortfolioStore.getState().holdings.loading).toBe(true);
    resolve(holdingsOf([holdingA], 8_000_000_000_000));
    await poll;
    expect(usePortfolioStore.getState().holdings).toMatchObject({ loading: true, data: { asOfMs: 8_000_000_000_000 } });
    stop();
  });
});
