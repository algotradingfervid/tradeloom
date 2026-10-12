import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { InstrumentLabel } from './InstrumentLabel';
import { PriceChange } from './PriceChange';
import { useTicksStore } from '../../store/ticks';
import { tick } from '../../market/testing';
import { NIFTY50, NIFTY_OPTION, RELIANCE, RELIANCE_BSE, RELIANCE_OCT_FUT, SBIN } from '../../instruments/fixtures';
import { FIXED_NOW } from '../../api/mock/fixtures';
import { labelOfItem } from '../../instruments/naming';

beforeEach(() => useTicksStore.setState({ latest: {} }));

describe('InstrumentLabel', () => {
  it('FR-WATCH-013 AC1: future shows "RELIANCE OCT FUT", an F badge and tag NFO-FUT', () => {
    render(<InstrumentLabel instrument={RELIANCE_OCT_FUT} />);
    expect(screen.getByText('RELIANCE OCT FUT')).toHaveAttribute('data-instrument-name');
    expect(screen.getByLabelText('Future')).toHaveTextContent('F');
    expect(screen.getByText('NFO-FUT')).toBeInTheDocument();
  });
  it('FR-WATCH-013 AC2: NIFTY 50 tagged INDICES, no F badge; company name optional', () => {
    const { container } = render(<InstrumentLabel instrument={NIFTY50} />);
    expect(container.firstElementChild).toHaveAttribute('data-exchange-tag', 'INDICES');
    expect(screen.queryByLabelText('Future')).toBeNull();
    render(<InstrumentLabel instrument={RELIANCE} showName />);
    expect(screen.getByText('RELIANCE INDUSTRIES')).toBeInTheDocument();
  });
  it('carries the token and the full text as its title; only the symbol truncates (Ruling 96 I2)', () => {
    const { container } = render(<InstrumentLabel instrument={RELIANCE_OCT_FUT} />);
    const root = container.firstElementChild!;
    expect(root).toHaveAttribute('data-instrument-token', String(RELIANCE_OCT_FUT.token));
    expect(root).toHaveAttribute('title', 'RELIANCE OCT FUT · NFO-FUT · Future');
    expect(screen.getByText('NFO-FUT').closest('.instrument-label__symbol')).toBeNull();
  });
  it('FR-WATCH-008: a BSE row also listed on NSE carries an "Also on NSE" flag in text', () => {
    render(<InstrumentLabel instrument={{ ...RELIANCE_BSE, alsoOnNse: true }} />);
    expect(screen.getByText('Also on NSE')).toBeInTheDocument();
    expect(screen.getByText('Also on NSE').closest('.instrument-label__symbol')).toBeNull();
    render(<InstrumentLabel instrument={RELIANCE_BSE} />);
    expect(screen.getAllByText('Also on NSE')).toHaveLength(1);
  });
  it('accepts a watchlist item through labelOfItem', () => {
    const item = { token: SBIN.token, exchange: 'NSE', symbol: 'SBIN', displayName: 'SBIN', name: 'STATE BANK OF INDIA', isIndex: false };
    const { container } = render(<InstrumentLabel instrument={labelOfItem(item)} />);
    expect(container.firstElementChild).toHaveAttribute('data-exchange-tag', 'NSE');
  });
  it('NFR-15: an out-of-scope instrument renders nothing (and does not throw)', () => {
    const { container } = render(<InstrumentLabel instrument={NIFTY_OPTION} />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe('PriceChange', () => {
  it('shows — when no quote is known (never 0.00)', () => {
    render(<PriceChange token={SBIN.token} />);
    expect(screen.getByText('—')).toBeInTheDocument();
    expect(screen.queryByText(/0\.00/)).toBeNull();
  });
  it('FR-WATCH-005 AC4: 1,000 vs 1,010 renders (-0.99%) in the down style', () => {
    useTicksStore.getState().applyBatch([tick(SBIN.token, 1000, 1010)]);
    const { container } = render(<PriceChange token={SBIN.token} />);
    expect(screen.getByText('1,000.00')).toBeInTheDocument();
    expect(screen.getByText('(-0.99%)')).toBeInTheDocument();
    expect(container.firstElementChild).toHaveAttribute('data-direction', 'down');
  });
  it('absolute change when asked (ticker pills, wide)', () => {
    useTicksStore.getState().applyBatch([tick(NIFTY50.token, 22600, 22500)]);
    render(<PriceChange token={NIFTY50.token} showAbsolute />);
    expect(screen.getByText('+100.00 (+0.44%)')).toBeInTheDocument();
  });
  it('up moves are signed in text, not colour alone', () => {
    useTicksStore.getState().applyBatch([tick(SBIN.token, 1010, 1000, { tsMs: FIXED_NOW })]);
    const { container } = render(<PriceChange token={SBIN.token} />);
    expect(container.firstElementChild).toHaveAttribute('data-direction', 'up');
    expect(screen.getByText('(+1.00%)')).toBeInTheDocument();
  });
  it('without a previous close: price only, no fake (+0.00%), flat direction', () => {
    useTicksStore.getState().applyBatch([tick(SBIN.token, 1000, 0, { tsMs: FIXED_NOW })]);
    const { container } = render(<PriceChange token={SBIN.token} />);
    expect(screen.getByText('1,000.00')).toBeInTheDocument();
    expect(screen.queryByText(/%/)).toBeNull();
    expect(container.firstElementChild).toHaveAttribute('data-direction', 'flat');
  });
  it('an unchanged price is flat', () => {
    useTicksStore.getState().applyBatch([tick(SBIN.token, 1000, 1000, { tsMs: FIXED_NOW })]);
    const { container } = render(<PriceChange token={SBIN.token} />);
    expect(container.firstElementChild).toHaveAttribute('data-direction', 'flat');
    expect(screen.getByText('(+0.00%)')).toBeInTheDocument();
  });
  it('Ruling 93: a quote from a previous run is shown with a stale cue until a fresh tick arrives', () => {
    useTicksStore.getState().applyBatch([tick(SBIN.token, 1000, 1010, { tsMs: 1 })]);
    const { container, rerender } = render(<PriceChange token={SBIN.token} />);
    expect(container.firstElementChild).toHaveAttribute('data-stale', 'true');
    expect(screen.getByText('Last known price, not live yet')).toBeInTheDocument();
    expect(screen.getByText('1,000.00')).toBeInTheDocument();

    useTicksStore.getState().applyBatch([tick(SBIN.token, 1001, 1010, { tsMs: FIXED_NOW })]);
    rerender(<PriceChange token={SBIN.token} />);
    expect(container.firstElementChild).not.toHaveAttribute('data-stale');
    expect(screen.queryByText('Last known price, not live yet')).toBeNull();
  });
});
