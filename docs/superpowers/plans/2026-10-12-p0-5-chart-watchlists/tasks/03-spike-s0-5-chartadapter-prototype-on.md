> **P0-5 Chart, Watchlists & Ticker UI Implementation Plan — Task 3.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 3: Spike S0-5 — `ChartAdapter` prototype on lightweight-charts 5.2.1 (go/no-go for OQ-01)

Timebox: 3 days (ROADMAP S0-5). The prototype is kept as the adapter's first implementation (ARCHITECTURE §12.1). Every lightweight-charts call below was checked against `lightweight-charts@5.2.1/dist/typings.d.ts`: `createChart`, `IChartApi.addSeries/addPane/panes/removePane/removeSeries/timeScale/priceScale/applyOptions/subscribeCrosshairMove/takeScreenshot/chartElement/remove`, `IPaneApi.addSeries/setStretchFactor/getSeries/paneIndex`, `ISeriesApi.setData/update(bar, historicalUpdate)/barsInLogicalRange/attachPrimitive/detachPrimitive/priceToCoordinate/getPane/applyOptions/seriesType`, `ITimeScaleApi.getVisibleLogicalRange/setVisibleLogicalRange/getVisibleRange/subscribeVisibleLogicalRangeChange/scrollPosition/scrollToPosition/scrollToRealTime`, `ISeriesPrimitive`, `IPrimitivePaneView`, `IPrimitivePaneRenderer.draw(target)`, and fancy-canvas `useBitmapCoordinateSpace(scope => …)` with `scope.context/bitmapSize/verticalPixelRatio`. Series definitions are imported as `CandlestickSeries`, `HistogramSeries`, `LineSeries`. `update` with an older time throws `Cannot update oldest data…` unless `historicalUpdate` is true and the bar exists, and historical updates are refused when conflation is on. That is why the adapter sets `enableConflation: false`.

**Files:**
- Modify: `frontend/package.json` (`npm install --save-exact lightweight-charts@5.2.1`), `frontend/eslint.config.js`, `frontend/src/design/tokens.css`
- Create: `frontend/src/chart/ChartAdapter.ts`, `lwc.ts`, `bars.ts`, `time.ts`, `format.ts`, `theme.ts`, `attribution.ts`, `primitives/PriceBandPrimitive.ts`, `LightweightChartsAdapter.ts`, `testing/fakeLwc.ts`
- Test: `frontend/src/chart/bars.test.ts`, `time.test.ts`, `format.test.ts`, `theme.test.ts`, `primitives/PriceBandPrimitive.test.ts`, `LightweightChartsAdapter.test.ts`
- Create: `frontend/spike/s05.html`, `frontend/spike/s05.ts`, `frontend/e2e/spike-s05.spec.ts`, `docs/architecture/spikes/S0-5-chart.md`

**Interfaces:**
- Consumes: Task 2 `Bar`, `ChartTheme`, `StudySpec`, `PriceBandSpec`, `Handle`, `RangeInfo`, `TimeRange`, `InstrumentSpec` (in `chart/types.ts`). P0-4 Task 1 `formatPrice(v, tickDecimals)`, `colourTokens` / `ColourTokens` / `ThemeName` (`src/design/tokens.ts`), `applyTheme(name)` (`src/app/theme.tsx`, sets `data-theme` and the `--bg`/`--up`/`--down`/… variables on `<html>`).
- Produces:
  - `interface ChartAdapter { setCandles(bars, opts?: {keepView?: boolean}): void; prependHistory(bars): number; bars(): readonly Bar[]; addStudy(spec): Handle; removeStudy(h): void; addPriceBand(spec): Handle; removePriceBand(h): void; setTheme(t): void; onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void; visibleTimeRange(): TimeRange | null; destroy(): void }` (Task 4 adds the rest)
  - `class LightweightChartsAdapter implements ChartAdapter` with `constructor(container: HTMLElement, opts?: { lib?: LwcLib; maxStudies?: number })`
  - `LwcLib`, `realLwc`; `makeFakeLwc(): { lib: LwcLib; charts: FakeChart[] }`
  - `dedupeSorted(bars): Bar[]`, `mergeBars(existing, incoming): Bar[]`
  - `IST_OFFSET_SECONDS`, `toChartTime(ms): UTCTimestamp`, `fromChartTime(t): number`, `formatCrosshairTime(t, intraday): string`, `formatTickMark(t, type): string`
  - `precisionForTick(tick): number`, `chartPriceFormatter(decimals): (p: number) => string`
  - `readChartTheme(tokens: ColourTokens, el?): ChartTheme` (background/text/up/down from P0-4's colour tokens, grid/crosshair/separator/volume from the chart CSS variables)
  - `class PriceBandPrimitive`
  - `CHART_ATTRIBUTION`

- [ ] **Step 1: Install the pinned library and add the import fence**

Run: `cd frontend && npm install --save-exact lightweight-charts@5.2.1 && node -e "const p=require('./package.json');if(p.dependencies['lightweight-charts']!=='5.2.1')process.exit(1)"`
Expected: exit 0. `package-lock.json` resolves `fancy-canvas` `2.1.0`.

Append this block to the exported array in `frontend/eslint.config.js`:

```js
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: [
      'src/chart/LightweightChartsAdapter.ts', 'src/chart/lwc.ts', 'src/chart/time.ts',
      'src/chart/primitives/**', 'src/chart/testing/**', 'src/chart/**/*.test.ts',
    ],
    rules: {
      'no-restricted-imports': ['error', { paths: [{
        name: 'lightweight-charts',
        message: 'Use ChartAdapter (src/chart/ChartAdapter.ts); only the adapter may import lightweight-charts (FR-CHART-001).',
      }] }],
    },
  },
```

P0-4 keeps the theme colours in `src/design/tokens.ts` and writes them as CSS variables on `<html>` (`applyTheme`), which also sets `data-theme`. The chart needs a few extra, chart-only colours. Append them to `frontend/src/design/tokens.css` (P0-4's spacing/type file; colour literals are allowed only under `src/design/`). `--chart-bg` and `--chart-text` alias P0-4's `--bg` and `--text`, so they follow the theme with no literal of their own:

```css
/* chart (P0-5): background, text, up and down come from P0-4's colour tokens */
:root {
  --chart-bg: var(--bg);
  --chart-text: var(--text);
  --chart-grid: #eef1f4;
  --chart-crosshair: #7b8794;
  --chart-separator: #d9dee5;
  --chart-volume-up: rgba(22, 163, 74, 0.45);
  --chart-volume-down: rgba(220, 38, 38, 0.45);
  --chart-band: rgba(59, 130, 246, 0.12);
}

:root[data-theme="dark"] {
  --chart-grid: #1d232b;
  --chart-crosshair: #8a96a6;
  --chart-separator: #2a313b;
  --chart-volume-up: rgba(34, 197, 94, 0.45);
  --chart-volume-down: rgba(239, 68, 68, 0.45);
  --chart-band: rgba(96, 165, 250, 0.16);
}
```

- [ ] **Step 2: Write the failing unit tests**

`frontend/src/chart/bars.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { dedupeSorted, mergeBars } from './bars';

const b = (time: number, close: number) => ({ time, open: close, high: close, low: close, close, volume: 1 });

describe('bars', () => {
  it('sorts ascending and keeps the last duplicate', () => {
    expect(dedupeSorted([b(3, 3), b(1, 1), b(3, 30), b(2, 2)])).toEqual([b(1, 1), b(2, 2), b(3, 30)]);
  });
  it('mergeBars lets incoming replace existing times and adds new ones', () => {
    expect(mergeBars([b(1, 1), b(2, 2)], [b(2, 20), b(4, 4)])).toEqual([b(1, 1), b(2, 20), b(4, 4)]);
  });
});
```

`frontend/src/chart/time.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { TickMarkType } from 'lightweight-charts';
import { formatCrosshairTime, formatTickMark, fromChartTime, IST_OFFSET_SECONDS, toChartTime } from './time';

// 01 Oct 2026 09:15 IST == 03:45 UTC
const OCT1_0915_IST = Date.UTC(2026, 9, 1, 3, 45);

describe('IST time axis', () => {
  it('shifts epoch ms to IST wall-clock seconds and back', () => {
    expect(IST_OFFSET_SECONDS).toBe(19800);
    expect(toChartTime(OCT1_0915_IST)).toBe(OCT1_0915_IST / 1000 + 19800);
    expect(fromChartTime(toChartTime(OCT1_0915_IST))).toBe(OCT1_0915_IST);
  });
  it("FR-CHART-011 AC1: crosshair label reads 01 Oct '26 09:15", () => {
    expect(formatCrosshairTime(toChartTime(OCT1_0915_IST), true)).toBe("01 Oct '26 09:15");
    expect(formatCrosshairTime(toChartTime(OCT1_0915_IST), false)).toBe("01 Oct '26");
  });
  it('tick marks are IST', () => {
    const t = toChartTime(OCT1_0915_IST);
    expect(formatTickMark(t, TickMarkType.Year)).toBe('2026');
    expect(formatTickMark(t, TickMarkType.Month)).toBe('Oct');
    expect(formatTickMark(t, TickMarkType.DayOfMonth)).toBe('01');
    expect(formatTickMark(t, TickMarkType.Time)).toBe('09:15');
    expect(formatTickMark(t, TickMarkType.TimeWithSeconds)).toBe('09:15:00');
  });
});
```

`frontend/src/chart/format.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { chartPriceFormatter, precisionForTick } from './format';

describe('chart price format', () => {
  it('derives precision from tick size with a floor of 2', () => {
    expect(precisionForTick(0.05)).toBe(2);
    expect(precisionForTick(0.1)).toBe(2);
    expect(precisionForTick(0.0025)).toBe(4);
    expect(precisionForTick(1)).toBe(2);
  });
  it('uses Indian grouping on the price scale', () => {
    expect(chartPriceFormatter(2)(2252045.5)).toBe('22,52,045.50');
  });
});
```

`frontend/src/chart/theme.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest';
import { colourTokens } from '../design/tokens';
import { readChartTheme } from './theme';

describe('readChartTheme', () => {
  it('takes background/text/up/down from the theme tokens and the chart-only colours from CSS variables', () => {
    const vars: Record<string, string> = {
      '--chart-grid': ' #000003', '--chart-crosshair': '#000004', '--chart-separator': '#000005',
      '--chart-volume-up': '#000008', '--chart-volume-down': '#000009',
    };
    // jsdom's cascade support for custom properties is partial; stub the computed style deterministically.
    const spy = vi.spyOn(window, 'getComputedStyle').mockReturnValue({ getPropertyValue: (n: string) => vars[n] ?? '' } as never);
    const el = document.createElement('div');
    const t = colourTokens.dark;
    expect(readChartTheme(t, el)).toEqual({
      background: t.bg, text: t.text, grid: '#000003', crosshair: '#000004', separator: '#000005',
      up: t.up, down: t.down, volumeUp: '#000008', volumeDown: '#000009',
    });
    expect(spy).toHaveBeenCalledWith(el);
    spy.mockRestore();
  });
});
```

`frontend/src/chart/primitives/PriceBandPrimitive.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest';
import { PriceBandPrimitive } from './PriceBandPrimitive';

describe('PriceBandPrimitive', () => {
  it('fills the band between the two price coordinates in bitmap space', () => {
    const p = new PriceBandPrimitive({ from: 10, to: 20, color: 'band' });
    const requestUpdate = vi.fn();
    p.attached({ series: { priceToCoordinate: (x: number) => x }, requestUpdate } as never);
    const fillRect = vi.fn();
    const ctx = { fillRect, fillStyle: '' };
    const target = {
      useBitmapCoordinateSpace: (f: (s: unknown) => void) =>
        f({ context: ctx, bitmapSize: { width: 100, height: 80 }, verticalPixelRatio: 2, horizontalPixelRatio: 2, mediaSize: { width: 50, height: 40 } }),
    };
    p.paneViews()[0].renderer()!.draw(target as never);
    expect(ctx.fillStyle).toBe('band');
    expect(fillRect).toHaveBeenCalledWith(0, 20, 100, 20);
    p.update({ to: 30 });
    expect(requestUpdate).toHaveBeenCalledTimes(1);
    expect(p.paneViews()[0].zOrder?.()).toBe('bottom');
  });
});
```

`frontend/src/chart/LightweightChartsAdapter.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest';
import { LightweightChartsAdapter } from './LightweightChartsAdapter';
import { makeFakeLwc, type FakeChart } from './testing/fakeLwc';
import type { Bar, ChartTheme } from './types';

const STEP = 300_000;
const T0 = Date.UTC(2026, 9, 1, 3, 45);
const mk = (i: number, close = 100 + i): Bar => ({ time: T0 + i * STEP, open: 100 + i, high: 102 + i, low: 99 + i, close, volume: 10 * i });
const range = (from: number, n: number) => Array.from({ length: n }, (_, k) => mk(from + k));
const THEME: ChartTheme = { background: 'bg', text: 'tx', grid: 'gr', crosshair: 'cr', separator: 'sp', up: 'up', down: 'dn', volumeUp: 'vu', volumeDown: 'vd' };

let chart: FakeChart;
let adapter: LightweightChartsAdapter;
beforeEach(() => {
  const fake = makeFakeLwc();
  adapter = new LightweightChartsAdapter(document.createElement('div'), { lib: fake.lib });
  chart = fake.charts[0];
});

describe('LightweightChartsAdapter (S0-5 core)', () => {
  it('creates candles in pane 0 and volume in pane 1; attribution logo and conflation off', () => {
    expect(chart.options.layout.attributionLogo).toBe(false);
    expect(chart.options.timeScale.enableConflation).toBe(false);
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram']]);
  });

  it('setCandles sorts, de-duplicates and maps times to IST chart time', () => {
    adapter.setCandles([mk(2), mk(0), mk(1), mk(2, 999)]);
    const candles = chart.seriesOfType('Candlestick');
    expect(candles.data.map((d) => d.time)).toEqual([0, 1, 2].map((i) => (T0 + i * STEP) / 1000 + 19800));
    expect(candles.data[2].close).toBe(999);
    expect(adapter.bars()).toHaveLength(3);
  });

  it('colours volume bars from the theme', () => {
    adapter.setTheme(THEME);
    adapter.setCandles([mk(0, 101), mk(1, 50)]);
    expect(chart.seriesOfType('Histogram').data.map((d) => d.color)).toEqual(['vu', 'vd']);
  });

  it('FR-CHART-003 AC3: prependHistory adds only older bars and keeps the visible window', () => {
    adapter.setCandles(range(100, 100));
    chart.ts.logical = { from: 2, to: 60 };
    const added = adapter.prependHistory([...range(50, 60)]); // 50..109 — only 50..99 are older
    expect(added).toBe(50);
    expect(adapter.bars()).toHaveLength(150);
    expect(chart.ts.setLogicalCalls.at(-1)).toEqual({ from: 52, to: 110 });
    expect(adapter.prependHistory(range(100, 5))).toBe(0);
  });

  it('separate-pane study adds and removes its pane; NaN becomes whitespace', () => {
    const h = adapter.addStudy({ id: 'rsi', kind: 'line', pane: 'separate', color: 'c', points: [{ time: T0, value: Number.NaN }, { time: T0 + STEP, value: 55 }] });
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram'], ['Line']]);
    expect(chart.seriesOfType('Line').data).toEqual([{ time: T0 / 1000 + 19800 }, { time: (T0 + STEP) / 1000 + 19800, value: 55 }]);
    adapter.removeStudy(h);
    expect(chart.paneTypes()).toEqual([['Candlestick'], ['Histogram']]);
  });

  it('attaches and detaches a price-band primitive on the candle series', () => {
    const h = adapter.addPriceBand({ id: 'b', from: 1, to: 2, color: 'c' });
    expect(chart.seriesOfType('Candlestick').primitives).toHaveLength(1);
    adapter.removePriceBand(h);
    expect(chart.seriesOfType('Candlestick').primitives).toHaveLength(0);
  });

  it('setTheme applies layout, grid, crosshair and candle colours', () => {
    adapter.setTheme(THEME);
    expect(chart.options.layout.background).toEqual({ type: 'solid', color: 'bg' });
    expect(chart.options.layout.textColor).toBe('tx');
    expect(chart.options.grid.vertLines.color).toBe('gr');
    expect(chart.seriesOfType('Candlestick').options).toMatchObject({ upColor: 'up', downColor: 'dn', wickUpColor: 'up', wickDownColor: 'dn' });
  });

  it('reports barsBefore on visible range change and unsubscribes', () => {
    adapter.setCandles(range(0, 100));
    const seen: number[] = [];
    const off = adapter.onVisibleRangeChange((r) => seen.push(r.barsBefore));
    chart.ts.fire({ from: 7, to: 50 });
    off();
    chart.ts.fire({ from: 1, to: 50 });
    expect(seen).toEqual([7]);
  });

  it('visibleTimeRange converts back to epoch ms; destroy removes the chart', () => {
    chart.ts.visibleTimes = { from: T0 / 1000 + 19800, to: (T0 + STEP) / 1000 + 19800 };
    expect(adapter.visibleTimeRange()).toEqual({ from: T0, to: T0 + STEP });
    adapter.destroy();
    expect(chart.removed).toBe(true);
  });
});
```

- [ ] **Step 3: Run the tests to check they fail**

Run: `cd frontend && npx vitest run src/chart`
Expected: FAIL with `Failed to resolve import` for `./bars`, `./time`, `./format`, `./theme`, `./PriceBandPrimitive`, `./LightweightChartsAdapter`, `./testing/fakeLwc`.

- [ ] **Step 4: Implement the adapter core**

`frontend/src/chart/ChartAdapter.ts`:

```ts
import type { Bar, ChartTheme, Handle, PriceBandSpec, RangeInfo, StudySpec, TimeRange } from './types';

/** FR-CHART-001 swap point (ARCHITECTURE §8.5). Host views talk only to this interface. */
export interface ChartAdapter {
  setCandles(bars: readonly Bar[], opts?: { keepView?: boolean }): void;
  /** Lazy history: prepends only bars older than the first loaded one, without moving the view. Returns the count added. */
  prependHistory(bars: readonly Bar[]): number;
  bars(): readonly Bar[];
  addStudy(spec: StudySpec): Handle;
  removeStudy(h: Handle): void;
  addPriceBand(spec: PriceBandSpec): Handle;
  removePriceBand(h: Handle): void;
  setTheme(theme: ChartTheme): void;
  onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void;
  visibleTimeRange(): TimeRange | null;
  destroy(): void;
}
```

`frontend/src/chart/lwc.ts`:

```ts
import { CandlestickSeries, HistogramSeries, LineSeries, createChart } from 'lightweight-charts';

/** Injection seam so adapter tests run against a fake library (no canvas in jsdom). */
export interface LwcLib {
  createChart: typeof createChart;
  CandlestickSeries: typeof CandlestickSeries;
  HistogramSeries: typeof HistogramSeries;
  LineSeries: typeof LineSeries;
}

export const realLwc: LwcLib = { createChart, CandlestickSeries, HistogramSeries, LineSeries };
```

`frontend/src/chart/bars.ts`:

```ts
import type { Bar } from './types';

export function dedupeSorted(bars: readonly Bar[]): Bar[] {
  const byTime = new Map<number, Bar>();
  for (const b of bars) byTime.set(b.time, b);
  return [...byTime.values()].sort((a, b) => a.time - b.time);
}

export function mergeBars(existing: readonly Bar[], incoming: readonly Bar[]): Bar[] {
  return dedupeSorted([...existing, ...incoming]);
}
```

`frontend/src/chart/time.ts`:

```ts
import { TickMarkType, type UTCTimestamp } from 'lightweight-charts';

/**
 * lightweight-charts renders UTCTimestamp in UTC. IST has no DST, so we feed it IST wall-clock seconds
 * (epoch + 19 800 s). The axis, day/month boundaries and labels are then IST whatever the OS zone is.
 */
export const IST_OFFSET_SECONDS = 19_800;
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const pad = (n: number) => String(n).padStart(2, '0');

export function toChartTime(ms: number): UTCTimestamp {
  return (Math.floor(ms / 1000) + IST_OFFSET_SECONDS) as UTCTimestamp;
}

export function fromChartTime(t: UTCTimestamp | number): number {
  return ((t as number) - IST_OFFSET_SECONDS) * 1000;
}

const wall = (t: number) => new Date((t as number) * 1000);

export function formatCrosshairTime(t: UTCTimestamp | number, intraday: boolean): string {
  const d = wall(t as number);
  const day = `${pad(d.getUTCDate())} ${MONTHS[d.getUTCMonth()]} '${pad(d.getUTCFullYear() % 100)}`;
  return intraday ? `${day} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}` : day;
}

export function formatTickMark(t: UTCTimestamp | number, type: TickMarkType): string {
  const d = wall(t as number);
  switch (type) {
    case TickMarkType.Year: return String(d.getUTCFullYear());
    case TickMarkType.Month: return MONTHS[d.getUTCMonth()];
    case TickMarkType.DayOfMonth: return pad(d.getUTCDate());
    case TickMarkType.Time: return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
    case TickMarkType.TimeWithSeconds: return `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`;
  }
}
```

`frontend/src/chart/format.ts`:

```ts
import { formatPrice } from '../i18n/format';

export function precisionForTick(tick: number): number {
  if (!(tick > 0)) return 2;
  const decimals = (String(tick).split('.')[1] ?? '').length;
  return Math.max(2, decimals);
}

export function chartPriceFormatter(decimals: number): (p: number) => string {
  return (p) => formatPrice(p, decimals);
}
```

`frontend/src/chart/theme.ts`:

```ts
import type { ColourTokens } from '../design/tokens';
import type { ChartTheme } from './types';

const read = (s: CSSStyleDeclaration, name: string) => s.getPropertyValue(name).trim();

/**
 * Chart colours come from design tokens (no literal colours in src/, ARCHITECTURE §8.6): the shared ones from
 * P0-4's `useTheme().tokens`, the chart-only ones from the `--chart-*` variables in tokens.css.
 */
export function readChartTheme(tokens: ColourTokens, el: HTMLElement = document.documentElement): ChartTheme {
  const s = getComputedStyle(el);
  return {
    background: tokens.bg, text: tokens.text, grid: read(s, '--chart-grid'),
    crosshair: read(s, '--chart-crosshair'), separator: read(s, '--chart-separator'),
    up: tokens.up, down: tokens.down,
    volumeUp: read(s, '--chart-volume-up'), volumeDown: read(s, '--chart-volume-down'),
  };
}
```

`frontend/src/chart/attribution.ts`:

```ts
/** Apache-2.0 NOTICE attribution for lightweight-charts (README "attribution notice"); rendered on Settings → About. */
export const CHART_ATTRIBUTION = {
  name: 'TradingView Lightweight Charts™',
  notice: 'Copyright (с) 2025 TradingView, Inc. https://www.tradingview.com/',
  licence: 'Apache-2.0',
  url: 'https://www.tradingview.com/',
} as const;
```

`frontend/src/chart/primitives/PriceBandPrimitive.ts`:

```ts
import type {
  IPrimitivePaneRenderer, IPrimitivePaneView, ISeriesApi, ISeriesPrimitive, SeriesAttachedParameter, SeriesType, Time,
} from 'lightweight-charts';

type Target = Parameters<IPrimitivePaneRenderer['draw']>[0];
export interface PriceBand { from: number; to: number; color: string }

class BandRenderer implements IPrimitivePaneRenderer {
  constructor(private readonly y1: number | null, private readonly y2: number | null, private readonly color: string) {}
  draw(target: Target): void {
    const { y1, y2 } = this;
    if (y1 === null || y2 === null) return;
    target.useBitmapCoordinateSpace((scope) => {
      const top = Math.round(Math.min(y1, y2) * scope.verticalPixelRatio);
      const height = Math.round(Math.abs(y2 - y1) * scope.verticalPixelRatio);
      scope.context.fillStyle = this.color;
      scope.context.fillRect(0, top, scope.bitmapSize.width, height);
    });
  }
}

class BandView implements IPrimitivePaneView {
  constructor(private readonly source: PriceBandPrimitive) {}
  zOrder() { return 'bottom' as const; }
  renderer(): IPrimitivePaneRenderer { return this.source.renderer(); }
}

/** S0-5 "custom primitive": a horizontal price band (later reused for band fills, ARCHITECTURE §8.5). */
export class PriceBandPrimitive implements ISeriesPrimitive<Time> {
  private series: Pick<ISeriesApi<SeriesType, Time>, 'priceToCoordinate'> | null = null;
  private requestUpdate: (() => void) | null = null;
  private readonly views: readonly IPrimitivePaneView[] = [new BandView(this)];

  constructor(private band: PriceBand) {}

  attached(param: SeriesAttachedParameter<Time>): void {
    this.series = param.series;
    this.requestUpdate = param.requestUpdate;
  }

  detached(): void {
    this.series = null;
    this.requestUpdate = null;
  }

  paneViews(): readonly IPrimitivePaneView[] { return this.views; }

  update(patch: Partial<PriceBand>): void {
    this.band = { ...this.band, ...patch };
    this.requestUpdate?.();
  }

  renderer(): IPrimitivePaneRenderer {
    const s = this.series;
    return new BandRenderer(s?.priceToCoordinate(this.band.from) ?? null, s?.priceToCoordinate(this.band.to) ?? null, this.band.color);
  }
}
```

`frontend/src/chart/LightweightChartsAdapter.ts`:

```ts
import {
  ColorType, CrosshairMode,
  type CandlestickData, type HistogramData, type IChartApi, type ISeriesApi, type LineData, type LogicalRange,
  type TickMarkType, type Time, type UTCTimestamp, type WhitespaceData,
} from 'lightweight-charts';
import type { ChartAdapter } from './ChartAdapter';
import type { Bar, ChartTheme, Handle, InstrumentSpec, PriceBandSpec, RangeInfo, StudySpec, TimeRange } from './types';
import { realLwc, type LwcLib } from './lwc';
import { dedupeSorted } from './bars';
import { formatCrosshairTime, formatTickMark, fromChartTime, toChartTime } from './time';
import { chartPriceFormatter } from './format';
import { PriceBandPrimitive } from './primitives/PriceBandPrimitive';

export interface AdapterOptions { lib?: LwcLib; maxStudies?: number }
type StudySeries = ISeriesApi<'Line'> | ISeriesApi<'Histogram'>;

export class LightweightChartsAdapter implements ChartAdapter {
  protected readonly lib: LwcLib;
  protected readonly chart: IChartApi;
  protected readonly candles: ISeriesApi<'Candlestick'>;
  protected volume: ISeriesApi<'Histogram'> | null = null;
  protected data: Bar[] = [];
  protected index = new Map<number, number>();
  protected spec: InstrumentSpec = { intraday: true, hasVolume: true, pricePrecision: 2, minMove: 0.05 };
  protected theme: ChartTheme | null = null;
  protected readonly studies = new Map<string, { series: StudySeries; separate: boolean }>();
  protected readonly bands = new Map<string, PriceBandPrimitive>();

  constructor(container: HTMLElement, protected readonly opts: AdapterOptions = {}) {
    this.lib = opts.lib ?? realLwc;
    this.chart = this.lib.createChart(container, {
      autoSize: true,
      layout: { attributionLogo: false },
      timeScale: {
        timeVisible: true, secondsVisible: false, rightOffset: 5, enableConflation: false,
        tickMarkFormatter: (t: Time, type: TickMarkType) => formatTickMark(t as UTCTimestamp, type),
      },
      localization: {
        locale: 'en-IN',
        timeFormatter: (t: Time) => formatCrosshairTime(t as UTCTimestamp, this.spec.intraday),
        priceFormatter: (p: number) => chartPriceFormatter(this.spec.pricePrecision)(p),
      },
      crosshair: { mode: CrosshairMode.Normal },
      handleScroll: true,
      handleScale: { mouseWheel: true, pinch: true, axisPressedMouseMove: true, axisDoubleClickReset: true },
    });
    this.candles = this.chart.addSeries(this.lib.CandlestickSeries, { priceFormat: { type: 'price', precision: 2, minMove: 0.05 } });
    this.syncVolumeSeries();
  }

  setCandles(bars: readonly Bar[], opts: { keepView?: boolean } = {}): void {
    const view = opts.keepView ? this.chart.timeScale().getVisibleLogicalRange() : null;
    this.data = dedupeSorted(bars);
    this.reindex();
    this.redraw();
    if (view) this.chart.timeScale().setVisibleLogicalRange(view);
  }

  prependHistory(bars: readonly Bar[]): number {
    const first = this.data.length > 0 ? this.data[0].time : Number.POSITIVE_INFINITY;
    const older = dedupeSorted(bars).filter((b) => b.time < first);
    if (older.length === 0) return 0;
    const view = this.chart.timeScale().getVisibleLogicalRange();
    this.data = [...older, ...this.data];
    this.reindex();
    this.redraw();
    if (view) this.chart.timeScale().setVisibleLogicalRange({ from: view.from + older.length, to: view.to + older.length });
    return older.length;
  }

  bars(): readonly Bar[] { return this.data; }

  addStudy(spec: StudySpec): Handle {
    const pane = spec.pane === 'separate' ? this.chart.addPane() : this.chart.panes()[0];
    const points: (LineData<Time> | WhitespaceData<Time>)[] = spec.points.map((p) =>
      Number.isFinite(p.value) ? { time: toChartTime(p.time), value: p.value } : { time: toChartTime(p.time) });
    let series: StudySeries;
    if (spec.kind === 'line') {
      const s = pane.addSeries(this.lib.LineSeries, { color: spec.color, priceLineVisible: false });
      s.setData(points);
      series = s;
    } else {
      const s = pane.addSeries(this.lib.HistogramSeries, { color: spec.color, priceLineVisible: false });
      s.setData(points);
      series = s;
    }
    this.studies.set(spec.id, { series, separate: spec.pane === 'separate' });
    return { id: spec.id };
  }

  removeStudy(h: Handle): void {
    const s = this.studies.get(h.id);
    if (!s) return;
    if (s.separate) this.removeSeriesWithPane(s.series);
    else this.chart.removeSeries(s.series);
    this.studies.delete(h.id);
  }

  addPriceBand(spec: PriceBandSpec): Handle {
    const p = new PriceBandPrimitive({ from: spec.from, to: spec.to, color: spec.color });
    this.candles.attachPrimitive(p);
    this.bands.set(spec.id, p);
    return { id: spec.id };
  }

  removePriceBand(h: Handle): void {
    const p = this.bands.get(h.id);
    if (!p) return;
    this.candles.detachPrimitive(p);
    this.bands.delete(h.id);
  }

  setTheme(t: ChartTheme): void {
    this.theme = t;
    this.chart.applyOptions({
      layout: { background: { type: ColorType.Solid, color: t.background }, textColor: t.text, panes: { separatorColor: t.separator } },
      grid: { vertLines: { color: t.grid }, horzLines: { color: t.grid } },
      crosshair: {
        vertLine: { color: t.crosshair, labelBackgroundColor: t.crosshair },
        horzLine: { color: t.crosshair, labelBackgroundColor: t.crosshair },
      },
    });
    this.candles.applyOptions({
      upColor: t.up, downColor: t.down, borderUpColor: t.up, borderDownColor: t.down, wickUpColor: t.up, wickDownColor: t.down,
    });
    this.volume?.setData(this.data.map((b) => this.toVolume(b)));
  }

  onVisibleRangeChange(cb: (r: RangeInfo) => void): () => void {
    const ts = this.chart.timeScale();
    const handler = (r: LogicalRange | null) => {
      if (!r) return;
      const info = this.candles.barsInLogicalRange(r);
      if (info) cb({ barsBefore: info.barsBefore });
    };
    ts.subscribeVisibleLogicalRangeChange(handler);
    return () => ts.unsubscribeVisibleLogicalRangeChange(handler);
  }

  visibleTimeRange(): TimeRange | null {
    const r = this.chart.timeScale().getVisibleRange();
    if (!r) return null;
    return { from: fromChartTime(r.from as UTCTimestamp), to: fromChartTime(r.to as UTCTimestamp) };
  }

  destroy(): void {
    this.chart.remove();
    this.studies.clear();
    this.bands.clear();
  }

  // ---- internals -------------------------------------------------------------------------------

  protected syncVolumeSeries(): void {
    if (this.spec.hasVolume && !this.volume) {
      this.chart.panes()[0].setStretchFactor(3);
      const pane = this.chart.addPane();
      pane.setStretchFactor(1);
      this.volume = pane.addSeries(this.lib.HistogramSeries, { priceFormat: { type: 'volume' }, lastValueVisible: false, priceLineVisible: false });
      this.volume.setData(this.data.map((b) => this.toVolume(b)));
    } else if (!this.spec.hasVolume && this.volume) {
      this.removeSeriesWithPane(this.volume);
      this.volume = null;
    }
  }

  protected removeSeriesWithPane(series: StudySeries): void {
    const paneIndex = series.getPane().paneIndex();
    this.chart.removeSeries(series);
    const pane = this.chart.panes()[paneIndex];
    if (paneIndex > 0 && pane && pane.getSeries().length === 0) this.chart.removePane(paneIndex);
  }

  protected reindex(): void {
    this.index = new Map(this.data.map((b, i) => [b.time, i]));
  }

  protected redraw(): void {
    this.candles.setData(this.data.map((b) => this.toCandle(b)));
    this.volume?.setData(this.data.map((b) => this.toVolume(b)));
  }

  protected toCandle(b: Bar): CandlestickData<Time> {
    return { time: toChartTime(b.time), open: b.open, high: b.high, low: b.low, close: b.close };
  }

  protected toVolume(b: Bar): HistogramData<Time> {
    const time = toChartTime(b.time);
    if (!this.theme) return { time, value: b.volume };
    return { time, value: b.volume, color: b.close >= b.open ? this.theme.volumeUp : this.theme.volumeDown };
  }
}
```

`frontend/src/chart/testing/fakeLwc.ts`. This fake records calls and mirrors the real library's `update` rules, so a misuse throws the same way the real library does:

```ts
import type { LwcLib } from '../lwc';

type Opts = Record<string, any>;
const merge = (t: Opts, s: Opts): Opts => {
  for (const [k, v] of Object.entries(s ?? {})) {
    if (v && typeof v === 'object' && !Array.isArray(v) && typeof v !== 'function') t[k] = merge(t[k] ?? {}, v);
    else t[k] = v;
  }
  return t;
};

export class FakePriceScale {
  options_: Opts = { mode: 0, autoScale: true };
  applyOptions(o: Opts) { merge(this.options_, o); }
  options() { return this.options_; }
}

export class FakeSeries {
  data: any[] = [];
  updates: { bar: any; historical: boolean }[] = [];
  primitives: any[] = [];
  options: Opts;
  constructor(readonly type: string, options: Opts | undefined, public pane: FakePane) { this.options = merge({}, options ?? {}); }
  seriesType() { return this.type; }
  setData(d: any[]) { this.data = [...d]; }
  update(bar: any, historical = false) {
    const last = this.data[this.data.length - 1];
    if (!historical && last && bar.time < last.time) throw new Error(`Cannot update oldest data, last time=${last.time}, new time=${bar.time}`);
    if (historical) {
      const i = this.data.findIndex((x) => x.time === bar.time);
      if (i < 0) throw new Error('Cannot update non-existing data point when historicalUpdate is true');
      this.data[i] = bar;
    } else if (last && last.time === bar.time) this.data[this.data.length - 1] = bar;
    else this.data.push(bar);
    this.updates.push({ bar, historical });
  }
  applyOptions(o: Opts) { merge(this.options, o); }
  attachPrimitive(p: any) { this.primitives.push(p); }
  detachPrimitive(p: any) { this.primitives = this.primitives.filter((x) => x !== p); }
  barsInLogicalRange(r: { from: number; to: number }) { return { barsBefore: Math.max(0, Math.floor(r.from)), barsAfter: Math.max(0, this.data.length - 1 - Math.ceil(r.to)) }; }
  priceToCoordinate(p: number) { return p; }
  getPane() { return this.pane; }
}

export class FakePane {
  series: FakeSeries[] = [];
  stretch = 1;
  constructor(private readonly chart: FakeChart) {}
  paneIndex() { return this.chart.panesList.indexOf(this); }
  getSeries() { return this.series; }
  setStretchFactor(n: number) { this.stretch = n; }
  getStretchFactor() { return this.stretch; }
  addSeries(def: { type: string }, options?: Opts) { const s = new FakeSeries(def.type, options, this); this.series.push(s); return s; }
}

export class FakeTimeScale {
  logical: { from: number; to: number } | null = { from: 0, to: 100 };
  visibleTimes: { from: number; to: number } | null = null;
  position = 0;
  realTimeCalls = 0;
  setLogicalCalls: { from: number; to: number }[] = [];
  options: Opts = {};
  private handlers = new Set<(r: any) => void>();
  getVisibleLogicalRange() { return this.logical; }
  setVisibleLogicalRange(r: { from: number; to: number }) { this.logical = r; this.setLogicalCalls.push(r); this.fire(r); }
  getVisibleRange() { return this.visibleTimes; }
  subscribeVisibleLogicalRangeChange(h: (r: any) => void) { this.handlers.add(h); }
  unsubscribeVisibleLogicalRangeChange(h: (r: any) => void) { this.handlers.delete(h); }
  scrollPosition() { return this.position; }
  scrollToPosition(p: number) { this.position = p; }
  scrollToRealTime() { this.realTimeCalls++; this.position = 0; }
  applyOptions(o: Opts) { merge(this.options, o); }
  fire(r: { from: number; to: number } | null) { this.handlers.forEach((h) => h(r)); }
}

export class FakeChart {
  options: Opts;
  panesList: FakePane[] = [];
  ts = new FakeTimeScale();
  removed = false;
  private scales = new Map<string, FakePriceScale>();
  private crosshair = new Set<(p: any) => void>();
  constructor(options: Opts | undefined) { this.options = merge({}, options ?? {}); this.panesList.push(new FakePane(this)); }
  addSeries(def: { type: string }, options?: Opts, paneIndex = 0) { while (this.panesList.length <= paneIndex) this.panesList.push(new FakePane(this)); return this.panesList[paneIndex].addSeries(def, options); }
  addPane() { const p = new FakePane(this); this.panesList.push(p); return p; }
  panes() { return [...this.panesList]; }
  removePane(i: number) { this.panesList.splice(i, 1); }
  removeSeries(s: FakeSeries) { s.pane.series = s.pane.series.filter((x) => x !== s); }
  timeScale() { return this.ts; }
  priceScale(id: string, paneIndex = 0) { const k = `${id}@${paneIndex}`; if (!this.scales.has(k)) this.scales.set(k, new FakePriceScale()); return this.scales.get(k)!; }
  applyOptions(o: Opts) { merge(this.options, o); }
  subscribeCrosshairMove(h: (p: any) => void) { this.crosshair.add(h); }
  unsubscribeCrosshairMove(h: (p: any) => void) { this.crosshair.delete(h); }
  fireCrosshair(p: any) { this.crosshair.forEach((h) => h(p)); }
  remove() { this.removed = true; }
  // test helpers
  paneTypes() { return this.panesList.map((p) => p.series.map((s) => s.type)); }
  seriesOfType(type: string) { const s = this.panesList.flatMap((p) => p.series).find((x) => x.type === type); if (!s) throw new Error(`no ${type} series`); return s; }
}

export function makeFakeLwc(): { lib: LwcLib; charts: FakeChart[] } {
  const charts: FakeChart[] = [];
  const lib = {
    createChart: (_el: unknown, options?: Opts) => { const c = new FakeChart(options); charts.push(c); return c; },
    CandlestickSeries: { type: 'Candlestick' },
    HistogramSeries: { type: 'Histogram' },
    LineSeries: { type: 'Line' },
  } as unknown as LwcLib;
  return { lib, charts };
}
```

- [ ] **Step 5: Run the unit tests to check they pass**

Run: `cd frontend && npx vitest run src/chart && npm run typecheck && npx eslint src/chart`
Expected: PASS (bars 2, time 3, format 2, theme 1, primitive 1, adapter 8). Typecheck and ESLint exit 0.

- [ ] **Step 6: Build the spike harness**

`frontend/spike/s05.html` (Vite serves it in dev only; the production build input stays `index.html`):

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <title>S0-5 chart spike</title>
    <style>html, body { margin: 0; height: 100%; background: var(--chart-bg); } #chart { width: 1200px; height: 640px; }</style>
  </head>
  <body>
    <div id="chart"></div>
    <script type="module" src="./s05.ts"></script>
  </body>
</html>
```

`frontend/spike/s05.ts`:

```ts
import type { IChartApi } from 'lightweight-charts';
import '../src/design/tokens.css';
import { applyTheme } from '../src/app/theme';
import { colourTokens, type ThemeName } from '../src/design/tokens';
import { LightweightChartsAdapter } from '../src/chart/LightweightChartsAdapter';
import { realLwc, type LwcLib } from '../src/chart/lwc';
import { readChartTheme } from '../src/chart/theme';
import type { Bar, StudyPoint } from '../src/chart/types';

const FIVE_MIN = 300_000;
const END = Date.UTC(2026, 9, 9, 10, 0);
const token = (name: string) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();

function synth(endMs: number, count: number, seed: number): Bar[] {
  const out: Bar[] = [];
  let p = 1000 + seed;
  for (let i = count; i > 0; i--) {
    const t = endMs - i * FIVE_MIN;
    const d = Math.sin((t / FIVE_MIN) * 0.21 + seed) * 4;
    const o = p; const c = p + d;
    out.push({ time: t, open: o, high: Math.max(o, c) + 1, low: Math.min(o, c) - 1, close: c, volume: 1000 + (i % 37) * 25 });
    p = c;
  }
  return out;
}

function rsi(bars: Bar[], n = 14): StudyPoint[] {
  let gain = 0; let loss = 0;
  return bars.map((b, i) => {
    if (i === 0) return { time: b.time, value: Number.NaN };
    const ch = b.close - bars[i - 1].close;
    const g = Math.max(ch, 0); const l = Math.max(-ch, 0);
    if (i <= n) { gain += g / n; loss += l / n; return { time: b.time, value: i < n ? Number.NaN : 100 - 100 / (1 + gain / (loss || 1e-9)) }; }
    gain = (gain * (n - 1) + g) / n; loss = (loss * (n - 1) + l) / n;
    return { time: b.time, value: 100 - 100 / (1 + gain / (loss || 1e-9)) };
  });
}

let chartRef: IChartApi | null = null;
const lib: LwcLib = { ...realLwc, createChart: ((c, o) => (chartRef = realLwc.createChart(c, o))) as LwcLib['createChart'] };
const adapter = new LightweightChartsAdapter(document.getElementById('chart')!, { lib });
applyTheme('light');
adapter.setTheme(readChartTheme(colourTokens.light));

const metrics = { setCandles20kMs: 0, update1000Ms: 0, themeSwitchMs: 0, prepends: 0, lastPrependKeptView: false };

let t0 = performance.now();
adapter.setCandles(synth(END, 20_000, 1));
metrics.setCandles20kMs = performance.now() - t0;

const bars = synth(END, 2000, 1);
adapter.setCandles(bars);
adapter.addStudy({ id: 'rsi', kind: 'line', pane: 'separate', color: token('--chart-crosshair'), points: rsi(bars) });
const lo = Math.min(...bars.slice(-100).map((b) => b.low));
const hi = Math.max(...bars.slice(-100).map((b) => b.high));
const bandFrom = lo + (hi - lo) * 0.4;
const bandTo = lo + (hi - lo) * 0.6;
adapter.addPriceBand({ id: 'band', from: bandFrom, to: bandTo, color: token('--chart-band') });

// raw library update cost for the forming bar (Task 4 wraps this in updateLastBar)
const candleSeries = () => chartRef!.panes()[0].getSeries()[0];
const last = bars[bars.length - 1];
t0 = performance.now();
for (let i = 0; i < 1000; i++) {
  candleSeries().update({ time: (Math.floor(last.time / 1000) + 19_800) as never, open: last.open, high: last.high + i * 0.01, low: last.low, close: last.close + i * 0.01 });
}
metrics.update1000Ms = performance.now() - t0;

let oldest = bars[0].time;
let loading = false;
adapter.onVisibleRangeChange(({ barsBefore }) => {
  if (barsBefore > 10 || loading || metrics.prepends >= 5) return;
  loading = true;
  const before = adapter.visibleTimeRange();
  const older = synth(oldest, 500, 1);
  setTimeout(() => {
    adapter.prependHistory(older);
    oldest = older[0].time;
    const after = adapter.visibleTimeRange();
    metrics.lastPrependKeptView = !!before && !!after && before.from === after.from && before.to === after.to;
    metrics.prepends++;
    loading = false;
  }, 50);
});

function switchTheme(): Promise<number> {
  return new Promise((resolve) => {
    const start = performance.now();
    const root = document.documentElement;
    const next: ThemeName = root.dataset.theme === 'dark' ? 'light' : 'dark';
    applyTheme(next);
    adapter.setTheme(readChartTheme(colourTokens[next]));
    requestAnimationFrame(() => requestAnimationFrame(() => {
      metrics.themeSwitchMs = performance.now() - start;
      resolve(metrics.themeSwitchMs);
    }));
  });
}

/** Samples the screenshot inside the band vs above it: different pixels = primitive drawn. */
function bandVisible(): boolean {
  const canvas = chartRef!.takeScreenshot();
  const ctx = canvas.getContext('2d')!;
  const ratio = canvas.height / chartRef!.chartElement().clientHeight;
  const yIn = candleSeries().priceToCoordinate((bandFrom + bandTo) / 2)!;
  const yOut = candleSeries().priceToCoordinate(hi + (hi - lo))!;
  const px = (y: number) => Array.from(ctx.getImageData(2, Math.round(y * ratio), 1, 1).data).join(',');
  return px(yIn) !== px(yOut);
}

Object.assign(window, { __s05: { adapter, chart: () => chartRef, metrics, switchTheme, bandVisible, ready: true } });
```

`frontend/e2e/spike-s05.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

/* eslint-disable @typescript-eslint/no-explicit-any */
declare global { interface Window { __s05: any } }

test.describe('S0-5 lightweight-charts 5.2.1 go/no-go', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/spike/s05.html');
    await page.waitForFunction(() => window.__s05?.ready === true);
  });

  test('G1+G2 candles, volume pane and a separate study pane', async ({ page }) => {
    const panes = await page.evaluate(() => window.__s05.chart().panes().map((p: any) => p.getSeries().map((s: any) => s.seriesType())));
    expect(panes).toEqual([['Candlestick'], ['Histogram'], ['Line']]);
  });

  test('G3 custom primitive is drawn', async ({ page }) => {
    expect(await page.evaluate(() => window.__s05.bandVisible())).toBe(true);
  });

  test('G4 theme switch repaints within 300 ms (FR-SHELL-011 AC2)', async ({ page }) => {
    const ms = await page.evaluate(() => window.__s05.switchTheme());
    expect(ms).toBeLessThan(300);
    const [bg, token] = await page.evaluate(() => [
      window.__s05.chart().options().layout.background.color,
      getComputedStyle(document.documentElement).getPropertyValue('--chart-bg').trim(),
    ]);
    expect(bg).toBe(token);
  });

  test('G5 lazy history prepends without moving the visible window', async ({ page }) => {
    await page.evaluate(() => window.__s05.chart().timeScale().scrollToPosition(-1950, false));
    await page.waitForFunction(() => window.__s05.metrics.prepends >= 1);
    expect(await page.evaluate(() => window.__s05.metrics.lastPrependKeptView)).toBe(true);
  });

  test('G6 performance budget', async ({ page }) => {
    const m = await page.evaluate(() => window.__s05.metrics);
    console.log(`S05 metrics ${test.info().project.name}: ${JSON.stringify(m)}`);
    expect(m.setCandles20kMs).toBeLessThan(250);
    expect(m.update1000Ms).toBeLessThan(200);
  });
});
```

- [ ] **Step 7: Run the spike in both engines**

Run: `cd frontend && npx playwright test e2e/spike-s05.spec.ts --project=chromium --project=webkit --reporter=line`
Expected: 10 passed (5 checks × 2 engines). Two `S05 metrics …` lines are printed. WebKit is the closest stand-in for WebKit2GTK.

Then run the same harness inside the real Linux webview. In the P0-1 build container, run `wails dev`, open `http://localhost:34115/spike/s05.html` in the dev webview window, and run `await window.__s05.switchTheme()` plus `window.__s05.metrics` in its inspector console (right-click → Inspect Element is enabled in dev builds).
Expected: `themeSwitchMs < 300`, `prepends >= 1` after scrolling left with the mouse, and the RSI pane and band are visible.

- [ ] **Step 8: Record go/no-go**

Create `docs/architecture/spikes/S0-5-chart.md`:

```markdown
# S0-5 Chart prototype — lightweight-charts 5.2.1 behind ChartAdapter (OQ-01)

Date: <run date> · Engineer: <name> · Commit: <sha>
Harness: `frontend/spike/s05.html` · Checks: `frontend/e2e/spike-s05.spec.ts` · Adapter: `frontend/src/chart/LightweightChartsAdapter.ts`

## Go/no-go checklist (all must be GO for OQ-01 = lightweight-charts)

| # | Criterion | Threshold | Chromium | WebKit | wails dev (WebKit2GTK 4.1) | Result |
|---|---|---|---|---|---|---|
| G1 | Candlesticks + volume pane | panes = [[Candlestick],[Histogram]] | | | | |
| G2 | One separate study pane (RSI) | third pane [Line] | | | | |
| G3 | Custom primitive (PriceBandPrimitive) drawn | band pixel ≠ background pixel | | | | |
| G4 | Theme switch repaint | < 300 ms | | | | |
| G5 | Lazy history keeps view | visibleTimeRange identical before/after prepend | | | | |
| G6a | setCandles 20 000 bars | < 250 ms | | | | |
| G6b | 1 000 forming-bar updates | < 200 ms total | | | | |
| G7 | IST axis + `01 Oct '26 09:15` crosshair label | `time.test.ts` green | | | | |
| G8 | Licence | Apache-2.0; fancy-canvas MIT; attribution on About (`CHART_ATTRIBUTION`); `attributionLogo:false` | | | | |

Fill each engine column with the measured value from the `S05 metrics` lines (or the console in `wails dev`).

## Decision

GO → OQ-01 stays "lightweight-charts 5.2.1 + own studies/drawings"; P1 builds studies on `addStudy`/`ICustomSeriesPaneView`.
NO-GO (any row fails after one day of tuning) → raise OQ-01 to the user: flip to the licensed TradingView Charting Library before P1 M1b (ROADMAP P0 risk); the `ChartAdapter` contract is kept and only `LightweightChartsAdapter` is replaced.

## Notes
- Times are shifted to IST wall-clock seconds (`time.ts`) because the library formats UTCTimestamp in UTC.
- `update()` on an older bar throws unless `historicalUpdate` is set and the bar exists; conflation must be off for historical updates (verified in 5.2.1 source) — the adapter disables conflation.
- `attributionLogo` is off: a click would navigate the single Wails webview off-origin.
```

Fill in the run date, the measured values and the decision. If any row is NO-GO, stop and escalate to the user before going on to Task 4.

- [ ] **Step 9: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/eslint.config.js frontend/src/design/tokens.css \
  frontend/src/chart frontend/spike frontend/e2e/spike-s05.spec.ts docs/architecture/spikes/S0-5-chart.md
git commit -m "$(cat <<'EOF'
feat(chart): S0-5 spike - ChartAdapter on lightweight-charts 5.2.1 with go/no-go record

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
)"
```

---
