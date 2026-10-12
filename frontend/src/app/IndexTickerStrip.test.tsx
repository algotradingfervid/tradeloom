import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

const nav = vi.hoisted(() => ({ navigate: vi.fn() }));
vi.mock('react-router-dom', () => ({ useNavigate: () => nav.navigate }));
const useSubscription = vi.fn();
vi.mock('../market/hooks', () => ({ useSubscription: (...a: unknown[]) => useSubscription(...a), usePolled: () => false }));
vi.mock('../api/instruments', async () => {
  const f = await import('../instruments/fixtures');
  return { instrumentByIdentifier: vi.fn(async (id: string) => f.ALL_IN_SCOPE.find((i) => `${i.exchange}:${i.tradingsymbol}` === id)), instrumentsByTokens: vi.fn(async () => []) };
});

import { IndexTickerStrip } from './IndexTickerStrip';
import { useTicksStore } from '../store/ticks';
import { tick } from '../market/testing';
import { FIXED_NOW } from '../api/mock/fixtures';
import { NIFTY50, NIFTYBANK, SENSEX } from '../instruments/fixtures';

// Deviation: showOnDashboard (P0-5 Task 10, store/chartContextsStore) is not on this branch; the strip takes it as
// the onShow prop until Task 10 lands.
const showOnDashboard = vi.fn();
const setWidth = (w: number) => { Object.defineProperty(window, 'innerWidth', { configurable: true, value: w }); window.dispatchEvent(new Event('resize')); };
const pill = (name: string) => screen.getByRole('button', { name: new RegExp(`^${name}`) });
// The symbol fallback shows before the identifiers resolve, so wait for the resolved names (data-instrument-name).
const resolved = () => waitFor(() => expect(document.querySelectorAll('[data-instrument-name]')).toHaveLength(3));
const fresh = { tsMs: FIXED_NOW }; // stamped in this run, so not stale (Ruling 93)

beforeEach(() => { useTicksStore.setState({ latest: {} }); vi.clearAllMocks(); setWidth(1600); });

describe('IndexTickerStrip', () => {
  it('subscribes the three indices as a widget consumer', async () => {
    render(<IndexTickerStrip />);
    await waitFor(() => expect(useSubscription).toHaveBeenLastCalledWith('index-ticker-strip', 'widget', [NIFTY50.token, NIFTYBANK.token, SENSEX.token]));
  });

  it('AC4: no data → each pill shows — and never 0.00', async () => {
    render(<IndexTickerStrip />);
    await resolved();
    expect(screen.getAllByText('—')).toHaveLength(3);
    expect(screen.queryByText(/0\.00/)).toBeNull();
    expect(pill('SENSEX')).toHaveAccessibleName('SENSEX, no price yet');
  });

  it('AC1+AC2: a tick updates the pill to 22,600.00 ▲ +100.00 (+0.44%) in the up style', async () => {
    render(<IndexTickerStrip />);
    await resolved();
    act(() => useTicksStore.getState().applyBatch([tick(NIFTY50.token, 22600, 22500, fresh)]));
    const p = pill('NIFTY 50');
    expect(p).toHaveTextContent('22,600.00');
    expect(p).toHaveTextContent('▲+100.00 (+0.44%)');
    expect(p.className).toContain('ticker-pill--up');
    // The direction is in text, not colour alone.
    expect(p).toHaveAccessibleName('NIFTY 50, 22,600.00, up +100.00 (+0.44%)');
    expect(p).not.toHaveAttribute('data-stale');
  });

  it('narrower than 1440 px drops the absolute change', async () => {
    setWidth(1300);
    render(<IndexTickerStrip />);
    await resolved();
    act(() => useTicksStore.getState().applyBatch([tick(NIFTYBANK.token, 47000, 48000, fresh)]));
    const p = pill('NIFTY BANK');
    expect(p).toHaveTextContent('▼(-2.08%)');
    expect(p).not.toHaveTextContent('-1,000.00');
    expect(p.className).toContain('ticker-pill--down');
    expect(p).toHaveAccessibleName('NIFTY BANK, 47,000.00, down (-2.08%)');
  });

  it('resizing across 1440 px switches the format', async () => {
    render(<IndexTickerStrip />);
    await resolved();
    act(() => useTicksStore.getState().applyBatch([tick(NIFTYBANK.token, 47000, 48000, fresh)]));
    expect(pill('NIFTY BANK')).toHaveTextContent('-1,000.00');
    act(() => setWidth(1280));
    expect(pill('NIFTY BANK')).not.toHaveTextContent('-1,000.00');
  });

  it('no change: flat style, no arrow, "unchanged" in the name (PriceChange semantics)', async () => {
    render(<IndexTickerStrip />);
    await resolved();
    act(() => useTicksStore.getState().applyBatch([tick(SENSEX.token, 74000, 74000, fresh)]));
    const p = pill('SENSEX');
    expect(p.className).toContain('ticker-pill--flat');
    expect(p).not.toHaveTextContent('▲');
    expect(p).not.toHaveTextContent('▼');
    expect(p).toHaveAccessibleName('SENSEX, 74,000.00, unchanged +0.00 (+0.00%)');
  });

  it('no previous close: the price alone, never a fake +0.00%', async () => {
    render(<IndexTickerStrip />);
    await resolved();
    act(() => useTicksStore.getState().applyBatch([tick(SENSEX.token, 74000, 0, fresh)]));
    const p = pill('SENSEX');
    expect(p).toHaveTextContent('74,000.00');
    expect(p).not.toHaveTextContent('%');
    expect(p).toHaveAccessibleName('SENSEX, 74,000.00');
  });

  it('AC3: clicking SENSEX loads it on the dashboard chart and navigates Home', async () => {
    render(<IndexTickerStrip onShow={showOnDashboard} />);
    fireEvent.click(await screen.findByRole('button', { name: /^SENSEX/ }));
    expect(showOnDashboard).toHaveBeenCalledWith('BSE:SENSEX');
    expect(nav.navigate).toHaveBeenCalledWith('/home');
  });

  it('AC5: outside market hours the last quote from the Watch snapshot is shown, marked stale until a fresh tick', async () => {
    useTicksStore.getState().applyBatch([tick(SENSEX.token, 74100.25, 74000)]); // tsMs 1: from a previous run
    render(<IndexTickerStrip />);
    await waitFor(() => expect(pill('SENSEX')).toHaveTextContent('74,100.25'));
    expect(pill('SENSEX')).toHaveAttribute('data-stale', 'true');
    expect(pill('SENSEX')).toHaveAccessibleName('SENSEX, 74,100.25, up +100.25 (+0.14%). Last known price, not live yet');
    act(() => useTicksStore.getState().applyBatch([tick(SENSEX.token, 74120, 74000, fresh)]));
    expect(pill('SENSEX')).not.toHaveAttribute('data-stale');
  });

  it('price updates are not announced: no live region in the strip', async () => {
    const { container } = render(<IndexTickerStrip />);
    await resolved();
    act(() => useTicksStore.getState().applyBatch([tick(SENSEX.token, 74100, 74000, fresh)]));
    expect(container.querySelector('[aria-live], [role="status"], [role="log"], [role="alert"], [role="marquee"], [role="timer"]')).toBeNull();
  });

  it('pills are the only controls: three buttons, no order actions (indices are not tradable)', async () => {
    render(<IndexTickerStrip />);
    await resolved();
    expect(screen.getAllByRole('button')).toHaveLength(3);
    expect(screen.queryByRole('button', { name: /buy|sell|order/i })).toBeNull();
  });

  it('resolved names carry data-instrument-name and the token (FR-WATCH-013 AC3)', async () => {
    render(<IndexTickerStrip />);
    await resolved();
    const el = screen.getByText('NIFTY BANK');
    expect(el).toHaveAttribute('data-instrument-name');
    expect(el).toHaveAttribute('data-instrument-token', String(NIFTYBANK.token));
  });
});
