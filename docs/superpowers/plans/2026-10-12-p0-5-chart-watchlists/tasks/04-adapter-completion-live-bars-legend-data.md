> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 4.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 4: Adapter completion — live bars, legend data, scales, navigation, instrument spec, study limit

**Files:**
- Modify: `frontend/src/chart/ChartAdapter.ts`, `frontend/src/chart/LightweightChartsAdapter.ts`
- Test: `frontend/src/chart/LightweightChartsAdapter.live.test.ts`

**Interfaces:**
- Consumes: Task 3 adapter internals (`data`, `index`, `candles`, `volume`, `spec`, `syncVolumeSeries`, `toCandle`, `toVolume`), `fromChartTime`, `InstrumentSpec`, `LegendData`, `UpdateOutcome`.
- Produces (added to `ChartAdapter`): `setInstrument(spec: InstrumentSpec): void`, `updateLastBar(bar: Bar): UpdateOutcome`, `onCrosshairMove(cb: (d: LegendData | null) => void): () => void`, `setPriceScale(o: { log?: boolean; auto?: boolean }): void`, `priceScaleState(): { log: boolean; auto: boolean }`, `panBars(delta: number): void`, `zoom(factor: number): void`, `scrollToLatest(): void`. Also `class StudyLimitError extends Error` (message `This chart allows up to N indicators`).

- [ ] **Step 1: Write the failing test**

`frontend/src/chart/LightweightChartsAdapter.live.test.ts`:

```ts
import { PriceScaleMode } from 'lightweight-charts';
import { beforeEach, describe, expect, it } from 'vitest';
import { LightweightChartsAdapter } from './LightweightChartsAdapter';
import { StudyLimitError } from './ChartAdapter';
import { makeFakeLwc, type FakeChart } from './testing/fakeLwc';
import { toChartTime } from './time';
import type { Bar } from './types';

const STEP = 300_000;
const T0 = Date.UTC(2026, 9, 1, 4, 30); // 10:00 IST
const mk = (i: number, close = 100 + i): Bar => ({ time: T0 + i * STEP, open: 100, high: 110 + i, low: 90, close, volume: 5 });

let chart: FakeChart;
let a: LightweightChartsAdapter;
const setup = (maxStudies?: number) => {
  const fake = makeFakeLwc();
  a = new LightweightChartsAdapter(document.createElement('div'), { lib: fake.lib, maxStudies });
  chart = fake.charts[0];
};
beforeEach(() => setup());

describe('adapter completion', () => {
  it('index instrument: volume series and its pane are removed; precision and daily axis applied', () => {
    a.setInstrument({ intraday: false, hasVolume: false, pricePrecision: 2, minMove: 0.05 });
    expect(chart.paneTypes()).toEqual([['Candlestick']]);
    expect(chart.seriesOfType('Candlestick').options.priceFormat).toEqual({ type: 'price', precision: 2, minMove: 0.05 });
    expect(chart.options.timeScale.timeVisible).toBe(false);
    a.setInstrument({ intraday: true, hasVolume: true, pricePrecision: 2, minMove: 0.1 });
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram']]);
  });

  it('live forming bar: append then replace', () => {
    a.setCandles([mk(0), mk(1)]);
    expect(a.updateLastBar(mk(2))).toBe('appended');
    expect(a.updateLastBar(mk(2, 150))).toBe('replaced');
    expect(a.bars().at(-1)!.close).toBe(150);
    expect(chart.seriesOfType('Candlestick').data.at(-1).close).toBe(150);
  });

  it('late close for an older loaded bar updates in place; unknown older bar is ignored without throwing', () => {
    a.setCandles([mk(0), mk(1), mk(2)]);
    expect(a.updateLastBar(mk(1, 777))).toBe('historical');
    const series = chart.seriesOfType('Candlestick');
    expect(series.updates.at(-1)).toEqual({ bar: expect.objectContaining({ close: 777 }), historical: true });
    expect(a.bars()[1].close).toBe(777);
    expect(() => a.updateLastBar(mk(-5))).not.toThrow();
    expect(a.updateLastBar(mk(-5))).toBe('ignored');
    expect(a.bars()).toHaveLength(3);
  });

  it('FR-CHART-011 AC1: crosshair reports the hovered bar and its previous close; outside = last bar', () => {
    a.setCandles([mk(0), mk(1), mk(2)]);
    const seen: unknown[] = [];
    a.onCrosshairMove((d) => seen.push(d));
    chart.fireCrosshair({ time: toChartTime(mk(1).time), seriesData: new Map() });
    chart.fireCrosshair({ seriesData: new Map() });
    expect(seen).toEqual([{ bar: mk(1), previousClose: mk(0).close }, { bar: mk(2), previousClose: mk(1).close }]);
  });

  it('FR-CHART-011 AC2: L toggles log scale, A re-enables auto-scale', () => {
    a.setPriceScale({ log: true });
    expect(chart.priceScale('right', 0).options().mode).toBe(PriceScaleMode.Logarithmic);
    expect(a.priceScaleState()).toEqual({ log: true, auto: true });
    chart.priceScale('right', 0).applyOptions({ autoScale: false }); // user dragged the price scale
    a.setPriceScale({ auto: true });
    expect(a.priceScaleState()).toEqual({ log: true, auto: true });
    a.setPriceScale({ log: false });
    expect(chart.priceScale('right', 0).options().mode).toBe(PriceScaleMode.Normal);
  });

  it('FR-CHART-011 AC4 + keyboard nav: pan, zoom around the right edge, Home = latest', () => {
    chart.ts.position = 0;
    a.panBars(-10);
    expect(chart.ts.position).toBe(-10);
    chart.ts.logical = { from: 0, to: 100 };
    a.zoom(2);
    expect(chart.ts.logical).toEqual({ from: 50, to: 100 });
    a.scrollToLatest();
    expect(chart.ts.realTimeCalls).toBe(1);
  });

  it('FR-CHART-001 AC2: a host limit of 5 refuses the 6th study', () => {
    setup(5);
    for (let i = 0; i < 5; i++) a.addStudy({ id: `s${i}`, kind: 'line', pane: 'overlay', color: 'c', points: [] });
    expect(() => a.addStudy({ id: 's5', kind: 'line', pane: 'overlay', color: 'c', points: [] })).toThrow(StudyLimitError);
    expect(() => a.addStudy({ id: 's5', kind: 'line', pane: 'overlay', color: 'c', points: [] })).toThrow('This chart allows up to 5 indicators');
  });
});
```

- [ ] **Step 2: Run the test to check it fails**

Run: `cd frontend && npx vitest run src/chart/LightweightChartsAdapter.live.test.ts`
Expected: FAIL. `StudyLimitError` is not exported, and `a.setInstrument is not a function`.

- [ ] **Step 3: Implement**

In `frontend/src/chart/ChartAdapter.ts`, change the import line to
`import type { Bar, ChartTheme, Handle, InstrumentSpec, LegendData, PriceBandSpec, RangeInfo, StudySpec, TimeRange, UpdateOutcome } from './types';`.
Add these members to the `ChartAdapter` interface, after `destroy(): void;`:

```ts
  setInstrument(spec: InstrumentSpec): void;
  /** Live candle from the Go builder: append / replace last / update an older loaded bar / ignore unknown older bar. */
  updateLastBar(bar: Bar): UpdateOutcome;
  /** Bar under the crosshair (last bar when outside the chart). */
  onCrosshairMove(cb: (d: LegendData | null) => void): () => void;
  setPriceScale(opts: { log?: boolean; auto?: boolean }): void;
  priceScaleState(): { log: boolean; auto: boolean };
  panBars(delta: number): void;
  zoom(factor: number): void;
  scrollToLatest(): void;
```

And at the bottom of the file:

```ts
export class StudyLimitError extends Error {
  constructor(readonly max: number) {
    super(`This chart allows up to ${max} indicators`);
    this.name = 'StudyLimitError';
  }
}
```

In `frontend/src/chart/LightweightChartsAdapter.ts`:
- Change the `lightweight-charts` import to add `PriceScaleMode` and `type MouseEventParams`.
- Change `import type { ChartAdapter } from './ChartAdapter';` to `import { StudyLimitError, type ChartAdapter } from './ChartAdapter';`.
- Add `LegendData, UpdateOutcome` to the `./types` import.
- Insert as the first line of `addStudy`:

```ts
    if (this.opts.maxStudies !== undefined && this.studies.size >= this.opts.maxStudies) throw new StudyLimitError(this.opts.maxStudies);
```

Add these methods to the class, before `// ---- internals`:

```ts
  setInstrument(spec: InstrumentSpec): void {
    this.spec = spec;
    this.candles.applyOptions({ priceFormat: { type: 'price', precision: spec.pricePrecision, minMove: spec.minMove } });
    this.chart.applyOptions({ timeScale: { timeVisible: spec.intraday } });
    this.syncVolumeSeries();
  }

  updateLastBar(bar: Bar): UpdateOutcome {
    const last = this.data[this.data.length - 1];
    if (!last || bar.time > last.time) {
      this.data.push(bar);
      this.index.set(bar.time, this.data.length - 1);
      this.candles.update(this.toCandle(bar));
      this.volume?.update(this.toVolume(bar));
      return 'appended';
    }
    if (bar.time === last.time) {
      this.data[this.data.length - 1] = bar;
      this.candles.update(this.toCandle(bar));
      this.volume?.update(this.toVolume(bar));
      return 'replaced';
    }
    const i = this.index.get(bar.time);
    if (i === undefined) return 'ignored'; // older than anything loaded, or a gap: never throw (Review Focus 4)
    this.data[i] = bar;
    this.candles.update(this.toCandle(bar), true);
    this.volume?.update(this.toVolume(bar), true);
    return 'historical';
  }

  onCrosshairMove(cb: (d: LegendData | null) => void): () => void {
    const handler = (p: MouseEventParams<Time>) => {
      const idx = p.time === undefined ? this.data.length - 1 : this.index.get(fromChartTime(p.time as UTCTimestamp));
      if (idx === undefined || idx < 0) { cb(null); return; }
      cb({ bar: this.data[idx], previousClose: idx > 0 ? this.data[idx - 1].close : null });
    };
    this.chart.subscribeCrosshairMove(handler);
    return () => this.chart.unsubscribeCrosshairMove(handler);
  }

  setPriceScale(opts: { log?: boolean; auto?: boolean }): void {
    const patch: { mode?: PriceScaleMode; autoScale?: boolean } = {};
    if (opts.log !== undefined) patch.mode = opts.log ? PriceScaleMode.Logarithmic : PriceScaleMode.Normal;
    if (opts.auto !== undefined) patch.autoScale = opts.auto;
    this.chart.priceScale('right', 0).applyOptions(patch);
  }

  priceScaleState(): { log: boolean; auto: boolean } {
    const o = this.chart.priceScale('right', 0).options();
    return { log: o.mode === PriceScaleMode.Logarithmic, auto: o.autoScale };
  }

  /** Negative = towards older bars. */
  panBars(delta: number): void {
    const ts = this.chart.timeScale();
    ts.scrollToPosition(ts.scrollPosition() + delta, false);
  }

  /** factor > 1 zooms in, < 1 zooms out; the right edge stays fixed. */
  zoom(factor: number): void {
    const ts = this.chart.timeScale();
    const r = ts.getVisibleLogicalRange();
    if (!r || factor <= 0) return;
    const width = (r.to - r.from) / factor;
    ts.setVisibleLogicalRange({ from: r.to - width, to: r.to });
  }

  scrollToLatest(): void {
    this.chart.timeScale().scrollToRealTime();
  }
```

- [ ] **Step 4: Run all chart tests**

Run: `cd frontend && npx vitest run src/chart && npm run typecheck`
Expected: PASS (all Task 3 tests and the 7 new ones). Typecheck exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/chart
git commit -m "$(cat <<'EOF'
feat(chart): live bar updates with historical guard, legend data, scales, navigation, study limit

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
