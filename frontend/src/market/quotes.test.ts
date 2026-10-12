import { describe, expect, it } from 'vitest';
import { useTicksStore } from '../store/ticks';
import { applySnapshot, isStaleQuote, quoteChange, runStartedMs } from './quotes';
import { tick } from './testing';

describe('quote selectors over useTicksStore', () => {
  it('FR-WATCH-005 AC4: LTP 1,000 vs previous close 1,010 is -0.99%', () => {
    const ch = quoteChange(tick(1, 1000, 1010))!;
    expect(ch.abs).toBe(-10);
    expect(ch.pct.toFixed(2)).toBe('-0.99');
  });
  it('no previous close → no change (never a fake 0.00%)', () => {
    expect(quoteChange(tick(1, 10, 0))).toBeNull();
    expect(quoteChange(undefined)).toBeNull();
  });
  it('a Watch snapshot never overwrites a newer live tick', () => {
    useTicksStore.getState().applyBatch([tick(1, 11, 9, { tsMs: 200 })]);
    applySnapshot([tick(1, 10, 9, { tsMs: 100 }), tick(2, 5, 4, { tsMs: 100 })]);
    expect(useTicksStore.getState().latest[1].ltp).toBe(11);
    expect(useTicksStore.getState().latest[2].ltp).toBe(5);
  });
  it('a snapshot with the same tsMs as the stored tick does not replace it (the newer tsMs wins, ties keep the tick)', () => {
    useTicksStore.getState().applyBatch([tick(1, 11, 9, { tsMs: 200, polled: true })]);
    applySnapshot([tick(1, 10, 9, { tsMs: 200 })]);
    expect(useTicksStore.getState().latest[1]).toMatchObject({ ltp: 11, polled: true });
  });
  it('a newer snapshot quote does replace an older stored tick', () => {
    useTicksStore.getState().applyBatch([tick(1, 11, 9, { tsMs: 100 })]);
    applySnapshot([tick(1, 12, 9, { tsMs: 300 })]);
    expect(useTicksStore.getState().latest[1].ltp).toBe(12);
  });
});

describe('stale quotes (persisted snapshot from a previous run)', () => {
  it('a quote whose tsMs predates this run is stale until a fresh tick arrives', () => {
    const old = tick(1, 100, 99, { tsMs: runStartedMs() - 60_000 });
    const fresh = tick(1, 101, 99, { tsMs: runStartedMs() + 5 });
    expect(isStaleQuote(old)).toBe(true);
    expect(isStaleQuote(fresh)).toBe(false);
    expect(isStaleQuote(undefined)).toBe(false);
  });
});
