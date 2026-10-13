# S0-5 Chart prototype — lightweight-charts 5.2.1 behind ChartAdapter (OQ-01)

Date: 2026-10-13 · Engineer: Claude (P0-5 Task 3 implementer) · Commit: the commit that adds this file (branch `p0/p05-t3`)
Harness: `frontend/spike/s05.html` · Checks: `frontend/e2e/spike-s05.spec.ts` · Adapter: `frontend/src/chart/LightweightChartsAdapter.ts`

## Go/no-go checklist (all must be GO for OQ-01 = lightweight-charts)

Task 3 made the first run without a canvas (Vitest + jsdom against `testing/fakeLwc.ts`). P0-5 Task 19 (2026-10-13) ran
Step 7 in Playwright 1.64.0 headless on macOS 26.5.2 arm64: Chromium 156.0.8078.4 and WebKit 27.2, against the Vite
dev server (`E2E_PORT=5292 S05=1 npx --no-install playwright test`, harness `spike/s05.html`, base commit 3ec6a2c). The
Chromium and WebKit columns below hold the values from the `S05 …` log lines of that run. The values held across
`--repeat-each 5` (all passed). The `wails dev` (WebKit2GTK 4.1, Linux) column is still pending: it needs a Linux
desktop, which this Mac does not have.

| # | Criterion | Threshold | Chromium | WebKit | wails dev (WebKit2GTK 4.1) | Result |
|---|---|---|---|---|---|---|
| G1 | Candlesticks + volume pane | panes = [[Candlestick],[Histogram]] | GO: [[Candlestick],[Histogram],[Line]] | GO: [[Candlestick],[Histogram],[Line]] | pending (user) | GO in Chromium + WebKit; wails dev pending |
| G2 | One separate study pane (RSI) | third pane [Line] | GO: third pane [Line] | GO: third pane [Line] | pending (user) | GO in Chromium + WebKit; wails dev pending |
| G3 | Custom primitive (PriceBandPrimitive) drawn | A/B with and without the band: ≥ 90 % of in-band pixel rows differ, 0 rows > 2 px outside the band differ | GO: 49/49 in-band rows, 0 outside | GO: 48/49 in-band rows (1 row is an opaque grid line drawn over the `bottom` band), 0 outside | pending (user) | GO in Chromium + WebKit; wails dev pending. Negative control (transparent band) → false in both |
| G4 | Theme switch repaint | < 300 ms | GO: 28.1 ms | GO: 24 ms | pending (user) | GO in Chromium + WebKit; wails dev pending |
| G5 | Lazy history keeps view | anchor bar's x (`timeToCoordinate`) moves ≤ 0.5 px across the prepend (`getVisibleRange` is clamped to the data, so its `from` is not compared); one-frame: the first frame painted after the prepend equals a forced full repaint of the settled state | GO: 0 px after 1 and 2 frames; frame 1 repainted and settled | GO: 0 px after 1 and 2 frames; frame 1 repainted and settled | pending (user) | GO in Chromium + WebKit; wails dev pending. Negative control (wrong range for one frame) → caught in both |
| G6a | setCandles 20 000 bars | < 250 ms | GO: 36.4 ms | GO: 47 ms | pending (user) | GO in Chromium + WebKit; wails dev pending |
| G6b | 1 000 forming-bar updates | < 200 ms total | GO: 15.1 ms | GO: 14 ms | pending (user) | GO in Chromium + WebKit; wails dev pending |
| G7 | IST axis + `01 Oct '26 09:15` crosshair label | `time.test.ts` green | n/a | n/a | n/a | GO — green under the host zone, `TZ=America/New_York` and `TZ=Pacific/Kiritimati` |
| G8 | Licence | Apache-2.0; fancy-canvas MIT; attribution on About (`CHART_ATTRIBUTION`); `attributionLogo:false` | n/a | n/a | n/a | GO: licences OK; Settings → About shows the name, licence, NOTICE text and a link that opens in the system browser (`AboutGroup.test.tsx`, Task 19) |

Fill each engine column with the measured value from the `S05 metrics` lines (or the console in `wails dev`).

G8 detail: lightweight-charts 5.2.1 (Apache-2.0) and fancy-canvas 2.1.0 (MIT) are listed in `THIRD_PARTY_NOTICES.md`
with full texts, including the upstream NOTICE (v5.2.1) and fancy-canvas's MIT text, which the npm packages do not ship
(`scripts/gen-notices.sh`, `build/notices/`). `attributionLogo:false` is asserted in a unit test. Settings → About has a
"Chart library" section (Task 19) with `CHART_ATTRIBUTION`'s name, licence and NOTICE text, plus a "TradingView website"
link. The link's `href` is the URL. A click calls `system.openExternal` (Wails `runtime.BrowserOpenURL`, the system
browser), so the app's only webview never navigates off-origin. Checking that click in `wails dev` is part of the
pending `wails dev` run.

G3 harness change (Task 19). Task 18 saw G3 fail in WebKit only. The cause was the harness, not the rendering. The old
check compared one pixel row, at the band's middle price. In WebKit that row (y = 201) is a horizontal grid line.
lightweight-charts paints `bottom`-zOrder primitives before the opaque grid (`lightweight-charts.development.mjs`
`_drawSources(sourceBottomPaneViews)` then `_drawGrid`), so the grid line covers the band on that row, and the row
was identical with and without the band. Grid rows follow the price-axis font metrics, so they differ per engine.
Chromium had no grid line inside the band. A full A/B diff showed the band drawn in both engines (rows 175–225,
≈ 51 750 changed pixels each). The check now compares every row. It needs ≥ 90 % of the in-band rows to change and
no change more than 2 px outside the band, so it is stricter than one row: it also proves where the band is drawn. A
negative control (a transparent band) must return false. The adapter is unchanged.

G5 one-frame timing (Task 19). The anchor check reads model state, not pixels. So the harness also plants a sentinel
pixel on the main pane canvas before the prepend. In the next frame the sentinel must be gone, which proves the pane
was repainted. That first frame's pixels must also equal a forced full repaint (`applyOptions({})`) one frame later.
Comparing against the pixels from before the prepend is wrong: the blank area left of the oldest bar fills with the
new bars, and the price scale re-fits to them. A negative control applies the unshifted logical range at once and
corrects it one frame later, and the check must catch it (it does, in both engines). Measured: anchor shift 0 px after
1 and 2 frames, and the first frame is settled, in Chromium and WebKit. The synchronous read is logged only, because
the library applies range changes lazily.

## Real-render steps (P0-5 Task 19 items)

1. ~~Run Step 7 in Chromium and WebKit~~: done in Task 19 (see above). Re-run with `S05=1 npx playwright test e2e/spike-s05.spec.ts --project=chromium --project=webkit --reporter=line`; the spec is skipped in CI unless `S05` is set.
2. Run the harness in `wails dev` (WebKit2GTK 4.1): `await window.__s05.switchTheme()`, `window.__s05.bandVisible()` (then `window.__s05.bandCheck` for the row counts), scroll left until `window.__s05.metrics.prepends >= 1`, then read `window.__s05.metrics` (G5 needs `lastPrependKeptView`, `lastPrependFramePainted` and `lastPrependFirstFrameSettled` all true; G6 values come from a fresh load).
3. Fill in the wails dev column above with the measured values (Chromium and WebKit are filled).
4. Record a final GO or NO-GO under Decision. A NO-GO escalates OQ-01 to the user (see below).
5. ~~Render `CHART_ATTRIBUTION` (with its link) and the NOTICE on Settings → About~~: done in Task 19; G8 GO. In `wails dev`, click Settings → About → "TradingView website": the system browser opens and the app window stays on About.

## Decision

**Chromium: final GO. WebKit: final GO. `wails dev` (WebKit2GTK 4.1): pending (user).** (P0-5 Task 19, 2026-10-13.)
G1–G6 pass in both headless engines with wide margins, and G7 and G8 are GO. The G3 WebKit failure came from the
harness's single-row sample, not from the renderer (see "G3 harness change"). OQ-01 is not escalated for these two
engines. The overall OQ-01 decision becomes final when the `wails dev` column is filled. If a row fails there after
one day of tuning, the decision becomes NO-GO and OQ-01 is escalated.

GO → OQ-01 stays "lightweight-charts 5.2.1 + own studies/drawings"; P1 builds studies on `addStudy`/`ICustomSeriesPaneView`.
NO-GO (any row fails after one day of tuning) → raise OQ-01 to the user: flip to the licensed TradingView Charting Library before P1 M1b (ROADMAP P0 risk); the `ChartAdapter` contract is kept and only `LightweightChartsAdapter` is replaced.

## Notes
- Times are shifted to IST wall-clock seconds (`time.ts`) because the library formats UTCTimestamp in UTC.
- `update()` on an older bar throws unless `historicalUpdate` is set and the bar exists; conflation must be off for historical updates (verified in 5.2.1 source) — the adapter disables conflation.
- `attributionLogo` is off: a click would navigate the single Wails webview off-origin.
- 5.2.1 removes a non-preserved pane automatically when its last series is removed (`_cleanupIfPaneIsEmpty`). The adapter's `removeSeriesWithPane` calls `removePane` only when the pane count shows the library kept the pane, so it never removes a neighbouring pane. `fakeLwc` models this, plus `addPane(preserveEmptyPane)`, the ascending-time assertion in `setData` (development build only; production does not check), the unclamped `barsInLogicalRange` and the `removePane` bounds.
- The npm package does not ship the NOTICE file. `CHART_ATTRIBUTION.notice` follows the upstream NOTICE at tag v5.2.1 ("Copyright (с) 2025 TradingView, Inc. https://www.tradingview.com/"), with the Cyrillic "с" (U+0441) written as ASCII "c"; a unit test keeps the string ASCII. `THIRD_PARTY_NOTICES.md` reproduces the NOTICE verbatim.
