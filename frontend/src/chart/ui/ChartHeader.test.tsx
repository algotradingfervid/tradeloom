import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { QuoteConsumer } from '../../api/quotes';

const { useSubscription, watch } = vi.hoisted(() => ({
  useSubscription: vi.fn(),
  watch: vi.fn(async (_c: QuoteConsumer, tokens: number[]) => ({ live: [...tokens], polled: [] as number[], snapshot: [] })),
}));
// A real coordinator behind the real hook, with a fake Quotes.Watch, so the lifecycle test sees what Go would be sent.
vi.mock('../../market/index', async () => {
  const { SubscriptionCoordinator } = await import('../../market/subscriptions');
  return { subscriptions: new SubscriptionCoordinator({ watch, onSnapshot: () => {}, releaseDelayMs: 0, debounceMs: 0 }) };
});
vi.mock('../../market/hooks', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../market/hooks')>();
  function useSubscriptionSpy(...a: Parameters<typeof actual.useSubscription>) {
    useSubscription(...a);
    actual.useSubscription(...a);
  }
  return { ...actual, useSubscription: useSubscriptionSpy, usePolled: () => false };
});

import { ChartHeader, type ChartHeaderProps } from './ChartHeader';
import { useTicksStore } from '../../store/ticks';
import { subscriptions } from '../../market/index';
import { tick } from '../../market/testing';
import { NIFTY_OPTION, RELIANCE, SBIN } from '../../instruments/fixtures';

const props = (over: Partial<ChartHeaderProps> = {}): ChartHeaderProps => ({
  contextId: 'dashboard', instrument: RELIANCE, timeframe: '1h', refreshing: false, refreshDisabled: false,
  onOpenSearch: vi.fn(), onRefresh: vi.fn(), onTimeframe: vi.fn(), ...over,
});

beforeEach(() => { useTicksStore.setState({ latest: {} }); useSubscription.mockClear(); });

describe('ChartHeader', () => {
  it('FR-CHART-002 AC1: RELIANCE NSE <LTP> (<pct>%) coloured, LTP live from the chart subscription', () => {
    useTicksStore.getState().applyBatch([tick(RELIANCE.token, 1000, 990)]);
    render(<ChartHeader {...props()} />);
    const symbol = screen.getByRole('button', { name: 'Change symbol' });
    expect(within(symbol).getByText('RELIANCE')).toBeInTheDocument();
    expect(within(symbol).getByText('NSE')).toBeInTheDocument();
    expect(within(symbol).getByText('1,000.00')).toBeInTheDocument();
    expect(within(symbol).getByText('(+1.01%)')).toBeInTheDocument();
    expect(useSubscription).toHaveBeenCalledWith('chart-header:dashboard', 'chart', [RELIANCE.token]);
  });

  it('FR-CHART-002 AC2: every control has an accessible name/tooltip', () => {
    render(<ChartHeader {...props({ onOpenFullView: vi.fn() })} />);
    expect(screen.getByRole('button', { name: 'Change symbol' })).toHaveAttribute('title', 'Search instruments');
    expect(screen.getByRole('button', { name: 'Refresh chart data' })).toBeInTheDocument();
    expect(screen.getByLabelText('Timeframe')).toHaveAttribute('title', 'Timeframe');
    expect(screen.getByRole('button', { name: 'More chart actions' })).toBeInTheDocument();
  });

  it('FR-CHART-004 AC1: exactly 8 timeframes in order; change reports the id', () => {
    const onTimeframe = vi.fn();
    render(<ChartHeader {...props({ onTimeframe })} />);
    const select = screen.getByLabelText('Timeframe') as HTMLSelectElement;
    expect([...select.options].map((o) => o.text)).toEqual(['1 Minute', '3 Minutes', '5 Minutes', '10 Minutes', '15 Minutes', '30 Minutes', '1 Hour', '1 Day']);
    expect(select.value).toBe('1h');
    fireEvent.change(select, { target: { value: '1d' } });
    expect(onTimeframe).toHaveBeenCalledWith('1d');
  });

  it('FR-CHART-006 AC2: refresh is disabled and marked spinning while in progress', () => {
    const onRefresh = vi.fn();
    render(<ChartHeader {...props({ refreshing: true, onRefresh })} />);
    const btn = screen.getByRole('button', { name: 'Refresh chart data' });
    expect(btn).toBeDisabled();
    expect(btn.className).toContain('is-spinning');
    fireEvent.click(btn);
    expect(onRefresh).not.toHaveBeenCalled();
  });

  it('FR-CHART-017: ⋯ menu offers "Open in full view"', () => {
    const onOpenFullView = vi.fn();
    render(<ChartHeader {...props({ onOpenFullView })} />);
    fireEvent.click(screen.getByRole('button', { name: 'More chart actions' }));
    fireEvent.click(screen.getByRole('menuitem', { name: 'Open in full view' }));
    expect(onOpenFullView).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('a fall is cued in text (minus sign), not by colour alone; refreshDisabled disables refresh', () => {
    useTicksStore.getState().applyBatch([tick(RELIANCE.token, 990, 1000)]);
    render(<ChartHeader {...props({ refreshDisabled: true })} />);
    const symbol = screen.getByRole('button', { name: 'Change symbol' });
    expect(within(symbol).getByText(/^\([-−]1\.00%\)$/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Refresh chart data' })).toBeDisabled();
  });

  it('no instrument: shows — (never 0.00) and subscribes nothing; back button is named', () => {
    const onBack = vi.fn();
    render(<ChartHeader {...props({ instrument: undefined, onBack })} />);
    const symbol = screen.getByRole('button', { name: 'Change symbol' });
    expect(symbol).toHaveTextContent('—');
    expect(symbol).not.toHaveTextContent('0.00');
    expect(useSubscription).toHaveBeenCalledWith('chart-header:dashboard', 'chart', []);
    fireEvent.click(screen.getByRole('button', { name: 'Back' }));
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('an out-of-scope instrument keeps its name with the "View only" flag', () => {
    render(<ChartHeader {...props({ instrument: NIFTY_OPTION })} />);
    expect(within(screen.getByRole('button', { name: 'Change symbol' })).getByText('View only')).toBeInTheDocument();
  });

  it('G-0: no order actions in the header', () => {
    render(<ChartHeader {...props({ onOpenFullView: vi.fn() })} />);
    fireEvent.click(screen.getByRole('button', { name: 'More chart actions' }));
    expect(screen.queryByRole('button', { name: /buy|sell|order/i })).toBeNull();
    expect(screen.queryByRole('menuitem', { name: /buy|sell|order/i })).toBeNull();
  });
});

describe('ChartHeader subscription lifecycle', () => {
  const settle = async () => {
    await new Promise((r) => setTimeout(r, 0));
    await subscriptions.idle();
  };

  it('moves the chart set on token/context change and releases it on unmount (no leak)', async () => {
    await settle();
    watch.mockClear();
    const { rerender, unmount } = render(<ChartHeader {...props()} />);
    await settle();
    expect(watch).toHaveBeenLastCalledWith('chart', [RELIANCE.token]);

    rerender(<ChartHeader {...props({ instrument: SBIN })} />);
    await settle();
    expect(watch).toHaveBeenLastCalledWith('chart', [SBIN.token]);

    rerender(<ChartHeader {...props({ instrument: SBIN, contextId: 'full' })} />);
    await settle();
    expect(useSubscription).toHaveBeenLastCalledWith('chart-header:full', 'chart', [SBIN.token]);
    expect(watch).toHaveBeenLastCalledWith('chart', [SBIN.token]);

    unmount();
    await settle();
    expect(watch).toHaveBeenLastCalledWith('chart', []);
    expect(watch.mock.calls.every(([c]) => c === 'chart')).toBe(true);
  });
});
