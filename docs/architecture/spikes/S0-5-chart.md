# S0-5 Chart prototype — lightweight-charts 5.2.1 behind ChartAdapter (OQ-01)

Date: 2026-10-13 · Engineer: Claude (P0-5 Task 3 implementer) · Commit: the commit that adds this file (branch `p0/p05-t3`)
Harness: `frontend/spike/s05.html` · Checks: `frontend/e2e/spike-s05.spec.ts` · Adapter: `frontend/src/chart/LightweightChartsAdapter.ts`

## Go/no-go checklist (all must be GO for OQ-01 = lightweight-charts)

This run was made without a canvas (Vitest + jsdom against `testing/fakeLwc.ts`). Step 7 (Playwright in Chromium and
WebKit, then `wails dev` on WebKit2GTK 4.1) was **not run** in this session, by instruction: no browser, dev server or
Wails app may be started here. The engine columns stay open until the Task 19 controlled run fills them.

| # | Criterion | Threshold | Chromium | WebKit | wails dev (WebKit2GTK 4.1) | Result |
|---|---|---|---|---|---|---|
| G1 | Candlesticks + volume pane | panes = [[Candlestick],[Histogram]] | pending | pending | pending | Adapter wiring GO (unit: `creates candles in pane 0 and volume in pane 1…`); real render pending |
| G2 | One separate study pane (RSI) | third pane [Line] | pending | pending | pending | Adapter wiring GO (unit: `separate-pane study adds and removes its pane…`); real render pending |
| G3 | Custom primitive (PriceBandPrimitive) drawn | band pixel ≠ background pixel | pending | pending | pending | Draw maths GO (unit: `fills the band … in bitmap space`, attach/detach unit); pixel check pending |
| G4 | Theme switch repaint | < 300 ms | pending | pending | pending | Options plumbing GO (unit: `setTheme applies …`, `readChartTheme`); timing pending |
| G5 | Lazy history keeps view | visibleTimeRange identical before/after prepend | pending | pending | pending | Logical-range shift GO (unit: `FR-CHART-003 AC3: prependHistory …`); real scroll pending |
| G6a | setCandles 20 000 bars | < 250 ms | pending | pending | pending | pending (needs a real canvas) |
| G6b | 1 000 forming-bar updates | < 200 ms total | pending | pending | pending | pending (needs a real canvas) |
| G7 | IST axis + `01 Oct '26 09:15` crosshair label | `time.test.ts` green | n/a | n/a | n/a | GO — green under the host zone, `TZ=America/New_York` and `TZ=Pacific/Kiritimati` |
| G8 | Licence | Apache-2.0; fancy-canvas MIT; attribution on About (`CHART_ATTRIBUTION`); `attributionLogo:false` | n/a | n/a | n/a | GO — lockfile: lightweight-charts 5.2.1 Apache-2.0, fancy-canvas 2.1.0 MIT; `attributionLogo:false` asserted in unit test; About-page rendering is a later task |

Fill each engine column with the measured value from the `S05 metrics` lines (or the console in `wails dev`).

## Decision

**Provisional GO, final decision pending the real-render run.** Every criterion that can be checked without a canvas
(G7, G8, and the adapter-side wiring for G1–G5) is GO. G1–G6 still need the Step 7 run in Chromium, WebKit and
`wails dev` (Task 19 controlled run). If any of those rows fails after one day of tuning, the decision becomes NO-GO.

GO → OQ-01 stays "lightweight-charts 5.2.1 + own studies/drawings"; P1 builds studies on `addStudy`/`ICustomSeriesPaneView`.
NO-GO (any row fails after one day of tuning) → raise OQ-01 to the user: flip to the licensed TradingView Charting Library before P1 M1b (ROADMAP P0 risk); the `ChartAdapter` contract is kept and only `LightweightChartsAdapter` is replaced.

## Notes
- Times are shifted to IST wall-clock seconds (`time.ts`) because the library formats UTCTimestamp in UTC.
- `update()` on an older bar throws unless `historicalUpdate` is set and the bar exists; conflation must be off for historical updates (verified in 5.2.1 source) — the adapter disables conflation.
- `attributionLogo` is off: a click would navigate the single Wails webview off-origin.
- 5.2.1 removes a non-preserved pane automatically when its last series is removed (`_cleanupIfPaneIsEmpty`, development build). The adapter's `removeSeriesWithPane` reads the pane index first and only calls `removePane` if that index still holds an empty pane, so it never removes a second pane. `fakeLwc` does not model the auto-cleanup; both paths end in the same pane list.
- The npm package does not ship the NOTICE file, so the `CHART_ATTRIBUTION.notice` text (year 2025, as in the plan) could not be checked against the installed package; the dist banner reads "Copyright (c) 2026 TradingView, Inc.". Check against upstream NOTICE before the About page ships.
