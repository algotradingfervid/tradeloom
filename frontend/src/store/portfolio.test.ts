import { describe, expect, it } from 'vitest';
import { demoSeed, holdingA, holdingsOf, readySession } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { startEventBridge } from '../api/events';
import { usePortfolioStore } from './portfolio';
import { useSessionStore } from './session';

describe('portfolio store', () => {
  it('Ruling 83-F M5: signing out clears the portfolio (account data)', () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: () => {} });
    useSessionStore.getState().setSession(readySession());
    backend.emit('holdings.changed', holdingsOf([holdingA], 1_000));
    expect(usePortfolioStore.getState().holdings.data.rows).toHaveLength(1);
    backend.emit('session.state', readySession({ state: 'logged_out' }));
    expect(usePortfolioStore.getState().holdings).toMatchObject({ status: 'idle', data: { rows: [] } });
    stop();
  });

  it('orders.changed and positions.changed replace the shown snapshot', () => {
    const backend = createMockBackend(demoSeed());
    const stop = startEventBridge(backend, { navigate: () => {} });
    backend.emit('orders.changed', { ...backend.state.book, asOfMs: 5 });
    backend.emit('positions.changed', { ...backend.state.positions, asOfMs: 5 });
    expect(usePortfolioStore.getState().orders).toMatchObject({ status: 'ready', data: { asOfMs: 5 } });
    expect(usePortfolioStore.getState().positions).toMatchObject({ status: 'ready', data: { asOfMs: 5 } });
    stop();
  });
});
