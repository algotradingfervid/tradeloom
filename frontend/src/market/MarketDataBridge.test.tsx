import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const resendAll = vi.fn();
vi.mock('./index', () => ({ subscriptions: { resendAll: () => resendAll() } }));

import type { FeedState } from '../api/backend';
import { BackendProvider } from '../api/BackendContext';
import { currentBackend } from '../api/current';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { useStatusStore } from '../store/status';
import { MarketDataBridge } from './MarketDataBridge';

const setFeedState = (state: FeedState['state']) => useStatusStore.setState({ feed: { ...useStatusStore.getState().feed, state } });

describe('MarketDataBridge', () => {
  it('registers the backend for the market helpers and re-sends subscriptions when the feed becomes live', () => {
    const backend = createMockBackend(demoSeed());
    setFeedState('offline');
    const { unmount } = render(<BackendProvider backend={backend}><MarketDataBridge /></BackendProvider>);
    expect(currentBackend()).toBe(backend);
    setFeedState('reconnecting');
    expect(resendAll).not.toHaveBeenCalled();
    setFeedState('live');
    expect(resendAll).toHaveBeenCalledTimes(1);
    unmount();
    setFeedState('offline');
    setFeedState('live');
    expect(resendAll).toHaveBeenCalledTimes(1);
  });
});
