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
| G3 | Custom primitive (PriceBandPrimitive) drawn | A/B: in-band pixel row with the band ≠ the same row after `removePriceBand` | pending | pending | pending | Draw maths GO (unit: `fills the band … in bitmap space`, attach/detach unit); pixel check pending |
| G4 | Theme switch repaint | < 300 ms | pending | pending | pending | Options plumbing GO (unit: `setTheme applies …`, `readChartTheme`); timing pending |
| G5 | Lazy history keeps view | anchor bar's x (`timeToCoordinate`) moves ≤ 0.5 px across the prepend (`getVisibleRange` is clamped to the data, so its `from` is not compared) | pending | pending | pending | Logical-range shift GO (unit: `FR-CHART-003 AC3: prependHistory …`); real scroll pending |
| G6a | setCandles 20 000 bars | < 250 ms | pending | pending | pending | pending (needs a real canvas) |
| G6b | 1 000 forming-bar updates | < 200 ms total | pending | pending | pending | pending (needs a real canvas) |
| G7 | IST axis + `01 Oct '26 09:15` crosshair label | `time.test.ts` green | n/a | n/a | n/a | GO — green under the host zone, `TZ=America/New_York` and `TZ=Pacific/Kiritimati` |
| G8 | Licence | Apache-2.0; fancy-canvas MIT; attribution on About (`CHART_ATTRIBUTION`); `attributionLogo:false` | n/a | n/a | n/a | partial: licences OK; attribution and NOTICE shown on About later (P0-5 Task 19 brief) |

Fill each engine column with the measured value from the `S05 metrics` lines (or the console in `wails dev`).

G8 detail: lightweight-charts 5.2.1 (Apache-2.0) and fancy-canvas 2.1.0 (MIT) are listed in `THIRD_PARTY_NOTICES.md`
with full texts, including the upstream NOTICE (v5.2.1) and fancy-canvas's MIT text, which the npm packages do not ship
(`scripts/gen-notices.sh`, `build/notices/`). `attributionLogo:false` is asserted in a unit test. The About page does
not render `CHART_ATTRIBUTION` yet.

## Real-render steps (P0-5 Task 19 items)

1. Run Step 7 in Chromium and WebKit: `S05=1 npx playwright test e2e/spike-s05.spec.ts --project=chromium --project=webkit --reporter=line` (the spec is skipped in CI unless `S05` is set).
2. Run the harness in `wails dev` (WebKit2GTK 4.1): `await window.__s05.switchTheme()`, `window.__s05.bandVisible()`, scroll left until `window.__s05.metrics.prepends >= 1`, then read `window.__s05.metrics`.
3. Fill in the Chromium, WebKit and wails dev columns above with the measured values.
4. Record a final GO or NO-GO under Decision. A NO-GO escalates OQ-01 to the user (see below).
5. Render `CHART_ATTRIBUTION` (with its link) and the NOTICE on Settings → About, then mark G8 GO.

## Decision

**Provisional GO, final decision pending the real-render run.** Every criterion that can be checked without a canvas
is GO: G7, the licence half of G8, and the adapter-side wiring for G1–G5. G1–G6 still need the Step 7 run in Chromium, WebKit and
`wails dev` (Task 19 controlled run). If any of those rows fails after one day of tuning, the decision becomes NO-GO.

GO → OQ-01 stays "lightweight-charts 5.2.1 + own studies/drawings"; P1 builds studies on `addStudy`/`ICustomSeriesPaneView`.
NO-GO (any row fails after one day of tuning) → raise OQ-01 to the user: flip to the licensed TradingView Charting Library before P1 M1b (ROADMAP P0 risk); the `ChartAdapter` contract is kept and only `LightweightChartsAdapter` is replaced.

## Notes
- Times are shifted to IST wall-clock seconds (`time.ts`) because the library formats UTCTimestamp in UTC.
- `update()` on an older bar throws unless `historicalUpdate` is set and the bar exists; conflation must be off for historical updates (verified in 5.2.1 source) — the adapter disables conflation.
- `attributionLogo` is off: a click would navigate the single Wails webview off-origin.
- 5.2.1 removes a non-preserved pane automatically when its last series is removed (`_cleanupIfPaneIsEmpty`). The adapter's `removeSeriesWithPane` calls `removePane` only when the pane count shows the library kept the pane, so it never removes a neighbouring pane. `fakeLwc` models this, plus `addPane(preserveEmptyPane)`, the ascending-time assertion in `setData` (development build only; production does not check), the unclamped `barsInLogicalRange` and the `removePane` bounds.
- The npm package does not ship the NOTICE file. `CHART_ATTRIBUTION.notice` follows the upstream NOTICE at tag v5.2.1 ("Copyright (с) 2025 TradingView, Inc. https://www.tradingview.com/"), with the Cyrillic "с" (U+0441) written as ASCII "c"; a unit test keeps the string ASCII. `THIRD_PARTY_NOTICES.md` reproduces the NOTICE verbatim.
