import { describe, expect, it } from 'vitest';
import { makeFakeLwc } from './fakeLwc';

// The fake must fail the way lightweight-charts 5.2.1 does (development build), so adapter misuse shows up in tests.
const chartOf = () => {
  const fake = makeFakeLwc();
  fake.lib.createChart(document.createElement('div'));
  return fake.charts[0];
};
const line = { type: 'Line' };

describe('fakeLwc mirrors lightweight-charts 5.2.1', () => {
  it('removeSeries removes the pane it empties unless the pane was added with preserveEmptyPane', () => {
    const c = chartOf();
    const p1 = c.addPane();
    const s1 = p1.addSeries(line);
    const p2 = c.addPane(true);
    const s2 = p2.addSeries(line);
    c.removeSeries(s1);
    expect(c.panes()).toHaveLength(2); // pane 1 went away with its last series
    c.removeSeries(s2);
    expect(c.panes()).toHaveLength(2); // preserved pane stays, empty
    expect(c.panes()[1].getSeries()).toEqual([]);
  });

  it('never auto-removes the last pane', () => {
    const c = chartOf();
    const s = c.addSeries(line);
    c.removeSeries(s);
    expect(c.panes()).toHaveLength(1);
  });

  it('setData throws unless times are strictly ascending', () => {
    const s = chartOf().addSeries(line);
    expect(() => s.setData([{ time: 2 }, { time: 1 }])).toThrow(/data must be asc ordered by time/);
    expect(() => s.setData([{ time: 1 }, { time: 1 }])).toThrow(/data must be asc ordered by time/);
    expect(() => s.setData([{ time: 1 }, { time: 2 }])).not.toThrow();
  });

  it('barsInLogicalRange: null when empty, unclamped barsBefore/barsAfter at the edges', () => {
    const s = chartOf().addSeries(line);
    expect(s.barsInLogicalRange({ from: 0, to: 10 })).toBeNull();
    s.setData(Array.from({ length: 10 }, (_, i) => ({ time: i + 1, value: i })));
    expect(s.barsInLogicalRange({ from: 3, to: 6 })).toEqual({ barsBefore: 3, barsAfter: 3 });
    expect(s.barsInLogicalRange({ from: -4.5, to: 12 })).toEqual({ barsBefore: -4.5, barsAfter: -3 });
    expect(s.barsInLogicalRange({ from: 0.5, to: 8.5 })).toEqual({ barsBefore: 0.5, barsAfter: 0.5 });
  });

  it('removePane is a no-op with one pane left and throws on a bad index', () => {
    const c = chartOf();
    c.removePane(0);
    expect(c.panes()).toHaveLength(1);
    c.addPane(true);
    expect(() => c.removePane(5)).toThrow(/Invalid pane index/);
    expect(() => c.removePane(-1)).toThrow(/Invalid pane index/);
    c.removePane(1);
    expect(c.panes()).toHaveLength(1);
  });
});
