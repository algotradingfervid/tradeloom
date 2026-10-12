## Coverage — every AC of this plan's FRs → test

"Unit" paths are under `frontend/src/`. "e2e" specs are under `frontend/e2e/`. "Go" means a P0-2 or P0-3 test that this plan depends on and does not write.

| FR · AC | Test (task) |
|---|---|
| CHART-001 AC1 independent instances | `store/chartContextsStore.test.ts` "contexts are independent…" (T10) |
| CHART-001 AC2 max 5 studies | `chart/LightweightChartsAdapter.live.test.ts` "a host limit of 5 refuses the 6th study" (T4) |
| CHART-001 AC3 dark palette | `chart/ui/ChartView.test.tsx` "theme change re-applies chart colours" (T10); `e2e/spike-s05.spec.ts` G4 (T3) |
| CHART-002 AC1 `RELIANCE NSE <LTP> (<pct>%)` | `chart/ui/ChartHeader.test.tsx` AC1 (T8) |
| CHART-002 AC2 focus ring + tooltip | `ChartHeader.test.tsx` AC2 (T8); `e2e/layout-1280.spec.ts` Tab order (T18); Task 19 Step 4 #16 visual |
| CHART-003 AC1 ≥ 740 daily candles, windows ≤ 2000 d | `chart/ChartDataController.test.ts` AC1 (750 trading days ending now requested, T5); `api/market.test.ts` fetchHistory (T2); Go P0-2 backfill window test |
| CHART-003 missing range → backfill job (contract §C) | `ChartDataController.test.ts` "waits for backfill.progress", "finished before the answer", "abandoned after 30 s" (T5); `api/market.test.ts` backfill (T2) |
| CHART-003 AC2 10:00 candle updates, 10:05 new candle | Go P0-2 `candles/builder` boundary test; UI apply: `ChartDataController.test.ts` AC2 (T5), `e2e/chart-watchlists.spec.ts` forming candle (T18) |
| CHART-003 AC3 older candles without zoom reset | `LightweightChartsAdapter.test.ts` prependHistory (T3); `ChartDataController.test.ts` AC3 (T5); `spike-s05.spec.ts` G5 (T3) |
| CHART-003 AC4 Personal plan message, no historical calls | `ChartView.test.tsx` AC4 (session `dataPlan: 'none'`, T10); `ChartDataController.test.ts` AC4 (`needsDataPlan`, T5); `api/market.test.ts` Personal plan (T2); `chart-watchlists.spec.ts` `?seed=personal` (T18); Task 19 Step 5 |
| CHART-003 AC5 reopen from store, no API call | Go P0-2 coverage-map test (`Chart.Candles` serves cached ranges with an empty `backfillJobId`); UI makes one `Candles` call per load when no job is returned (`ChartDataController.test.ts` AC1) |
| CHART-004 AC1 8 options in order | `chart/timeframes.test.ts` (T1); `ChartHeader.test.tsx` AC1 (T8); `chart-watchlists.spec.ts` Home (T18) |
| CHART-004 AC2/AC3 | Deferred: P1 (studies) / ANLY |
| CHART-005 AC1 focus + 4 chips | `chart/ui/SymbolSearchDialog.test.tsx` AC1 (T9); `chart-watchlists.spec.ts` (T18) |
| CHART-005 AC2 BANKNIFTY → index + futures, no options | `SymbolSearchDialog.test.tsx` AC2 (T9); `api/market.test.ts` NFR-15 filter (T2); Go P0-2 FR-WATCH-008 AC2 |
| CHART-005 AC3 "reliance oct fut" + F badge | `SymbolSearchDialog.test.tsx` AC3 (T9) |
| CHART-005 AC4 Enter selects ≤ 1 s, modal closes | `SymbolSearchDialog.test.tsx` AC4 (T9); `chart-watchlists.spec.ts` timing (T18) |
| CHART-005 AC5 Escape keeps symbol | `SymbolSearchDialog.test.tsx` AC5 (T9); `chart-watchlists.spec.ts` (T18) |
| CHART-006 AC1 wrong close replaced | `ChartDataController.test.ts` FR-CHART-006 (T5); Go P0-3 `Chart.Refresh` upsert test |
| CHART-006 AC2 second click no effect | `ChartDataController.test.ts` (T5); `ChartHeader.test.tsx` disabled (T8) |
| CHART-006 AC3 same range visible | `ChartDataController.test.ts` keepView (T5); `LightweightChartsAdapter.test.ts` (setCandles keepView path) (T3) |
| CHART-011 AC1 `01 Oct '26 09:15` + legend | `chart/time.test.ts` (T3); `LightweightChartsAdapter.live.test.ts` crosshair (T4); `ChartView.test.tsx` legend (T10) |
| CHART-011 AC2 L / A | `LightweightChartsAdapter.live.test.ts` (T4); `ChartView.test.tsx` (T10) |
| CHART-011 AC3 index without volume | `LightweightChartsAdapter.live.test.ts` (T4); `ChartDataController.test.ts` (T5); `ChartView.test.tsx` (T10) |
| CHART-011 AC4 Home key | `LightweightChartsAdapter.live.test.ts` (T4); `ChartView.test.tsx` keyboard (T10) |
| CHART-016 AC1 restart restores RELIANCE 15m | `chartContextsStore.test.ts` (T10); `DashboardChart.test.tsx` (T10) |
| CHART-016 AC2 Home → Scanners → Home | `chartContextsStore.test.ts` (T10) |
| CHART-016 AC3 Scalper | Deferred: P6 |
| CHART-017 AC1 ADANIENT 1 Day | `views/chart-full/ChartFullView.test.tsx` AC1 (T11); scanner entry point P3 |
| CHART-017 AC2 back with state intact | `ChartFullView.test.tsx` AC2 (T11); scanner results restoration P3 |
| CHART-017 AC3 F11 | `ChartFullView.test.tsx` AC3 ×2 (route in `rootChildren`, F11 back, T11); `DashboardChart.test.tsx` F11 (T11); `chart-watchlists.spec.ts` (T18) |
| WATCH-001 AC1 fresh "Watchlist 1" + hint | `views/watchlists/WatchlistPanel.test.tsx` (T16); `WatchlistRows.test.tsx` (T14) |
| WATCH-001 AC2 second tab, active after restart | `WatchlistPanel.test.tsx` (`SetActive` persisted in Go, T16); `store/watchlistsStore.test.ts` load uses Go's `activeId` (T13) |
| WATCH-001 AC3 60 instruments, no cap | `WatchlistRows.test.tsx` (T14) |
| WATCH-001 AC4 Alt+Up persists | `WatchlistRows.test.tsx` (T14); `watchlistsStore.test.ts` (T13) |
| WATCH-001 AC5 over budget → polled hint, ≤ 5 s | `market/subscriptions.test.ts` 250 tokens (T6); `WatchlistRows.test.tsx` polled (T14); Go P0-3 5 s poller test |
| WATCH-002 AC1 "Watchlist 3" | `names.test.ts`, `watchlistsStore.test.ts`, `WatchlistPanel.test.tsx` (T13, T16) |
| WATCH-002 AC2 duplicate name error | `watchlistsStore.test.ts`, `WatchlistPanel.test.tsx` (T13, T16) |
| WATCH-002 AC3 empty refused | `names.test.ts`, `WatchlistPanel.test.tsx` (T13, T16) |
| WATCH-003 AC1 Cancel keeps | `WatchlistPanel.test.tsx` (T16) |
| WATCH-003 AC2 last list → empty "Watchlist 1" | `watchlistsStore.test.ts`, `WatchlistPanel.test.tsx` (T13, T16) |
| WATCH-003 AC3 strategy keeps copies | Deferred: P1 (strategies copy tokens at import; Go `Delete` never cascades) |
| WATCH-004 AC1 sbin + Enter appends | `views/watchlists/WatchlistSearch.test.tsx` (T15); `chart-watchlists.spec.ts` (T18) |
| WATCH-004 AC2 check + "Already in Watchlist 1" | `WatchlistSearch.test.tsx` (T15); `watchlistsStore.test.ts` (T13) |
| WATCH-004 AC3 "nifty oct fut" | `WatchlistSearch.test.tsx` (T15); `chart-watchlists.spec.ts` (T18) |
| WATCH-005 AC1 click → chart + highlight | `WatchlistRows.test.tsx` (T14); `chart-watchlists.spec.ts` (T18) |
| WATCH-005 AC2 index B/S disabled | Deferred: P4 (B/S hidden while trading is not compiled, G-0) |
| WATCH-005 AC3 Undo restores position | `WatchlistRows.test.tsx`, `watchlistsStore.test.ts` (T13, T14) |
| WATCH-005 AC4 `(-0.99%)` red | `components/instrument/instrument.test.tsx` (T7); `market/quotes.test.ts` (T6) |
| WATCH-006 AC1 A→B within 30 s | `subscriptions.test.ts` (T6); `WatchlistRows.test.tsx` re-subscribe (T14) |
| WATCH-006 AC2 reconnect resubscribes | Go P0-2 ticker `Resubscribe` test; `market/MarketDataBridge.test.tsx` + `subscriptions.test.ts` resendAll (T6); Task 19 Step 4 #10 |
| WATCH-006 AC3 budget full of deployments → polling | `subscriptions.test.ts` (polled tracking) (T6); Go P0-2 `ErrBudget` priority test |
| WATCH-007 AC1 Nifty 50 chip | Deferred: P1 (universe store) |
| WATCH-007 AC2 max 100 refusal | `views/watchlists/InstrumentPicker.test.tsx` (T17) |
| WATCH-007 AC3 300 without max | `InstrumentPicker.test.tsx` (T17) |
| WATCH-007 AC4 Save as watchlist "Banks" | `InstrumentPicker.test.tsx` (T17) |
| WATCH-007 AC5 Escape | `InstrumentPicker.test.tsx` (T17) |
| WATCH-013 AC1 `RELIANCE OCT FUT` / `NFO:RELIANCE26OCTFUT` | `instruments/naming.test.ts` (T1); `instrument.test.tsx` (T7) |
| WATCH-013 AC2 `NIFTY 50` `INDICES` | `naming.test.ts` (T1); `instrument.test.tsx` (T7) |
| WATCH-013 AC3 one formatter | `e2e/naming-consistency.spec.ts` (T18) |
| WATCH-014 AC1 RELIANCE `NSE` everywhere | `naming.test.ts` (T1); `naming-consistency.spec.ts` (T18; scanner results join in P3) |
| WATCH-014 AC2 no instrument with two tags | `naming-consistency.spec.ts` (T18) |
| SHELL-004 AC1 tick → pill ≤ 500 ms | `app/IndexTickerStrip.test.tsx` (T12); `chart-watchlists.spec.ts` (T18) |
| SHELL-004 AC2 `22,600.00` `+100.00 (+0.44%)` ▲ | `IndexTickerStrip.test.tsx` (T12); `instrument.test.tsx` (T7) |
| SHELL-004 AC3 SENSEX click → Home chart | `IndexTickerStrip.test.tsx` (T12); `chart-watchlists.spec.ts` (T18) |
| SHELL-004 AC4 "—" never 0.00 | `IndexTickerStrip.test.tsx` (T12) |
| SHELL-004 AC5 cached after restart | `IndexTickerStrip.test.tsx` AC5 (T12); `subscriptions.test.ts` snapshot + `quotes.test.ts` snapshot ordering (T6). Contract §C: the `Watch` snapshot is in-memory for the session, not persisted, so right after a restart outside market hours the pills show `—` until Go's 5 s poll (or the first tick) answers. Open item for the consistency reviewer: FR-SHELL-004 AC5 needs either a persisted last-quote cache in P0-3 or an accepted deviation. |
| NFR-07 input latency < 100 ms | `chart-watchlists.spec.ts` tick storm (T18) |
| NFR-08 live bars from the app-clock builder, applied on event | Go P0-2 builder; `ChartDataController.test.ts` live (T5) |
| NFR-11 1280 px, keyboard, theme, formats | `layout-1280.spec.ts` (T18); `ChartView.test.tsx` keyboard (T10); `format.test.ts` (T3); Task 19 Step 4 #16 |
| NFR-15 no options anywhere | `naming.test.ts` (T1); `api/market.test.ts` (T2); `instrumentsStore.test.ts` (T7); Task 19 Step 8 |
| Review Focus 1–5 | T5 empty history; T5 race; T10 market closed; T4 late close; T6 + T14 budget |

---

## Self-review (done while writing; fixes applied inline)

1. **Spec coverage.** Every AC of FR-CHART-001/002/003/004/005/006/011/016/017, FR-WATCH-001…007/013/014 and FR-SHELL-004 appears in the Coverage table. ACs owned by later phases are marked "Deferred" with the phase. The ROADMAP §P0 exit criteria (demo, second demo, NFR-01/02/04/09/11/15, the AC list) map to Task 19 Steps 1–9. Two gaps were found and fixed in this pass. First, FR-WATCH-007 had no P0 entry point, so the "Add instruments…" button was added to the panel (Task 17). Second, an expired contract left in a watchlist would have shown "…" forever, so an "Instrument no longer available" row with a remove action was added (Task 14).
2. **Placeholder scan.** No placeholder markers or deferred-code notes remain. Every code step has full code. The only values left for the engineer to write are measurements and observations in the S0-5 doc and the exit record, which only exist after the runs.
3. **Type consistency.** Checked across tasks:
   - `ChartAdapter` members (T3 + T4) match `FakeAdapter` (T5) and `LightweightChartsAdapter`.
   - `ChartDataController(adapter, api, onState, onLiveBar?)` (T5) matches its use in `ChartView` (T10).
   - `CandlesPage.bars` and `Bar` (T2) are used the same way in T3–T10; `ChartDataApi` (T5) = the `api/chart.ts` helper signatures incl. `onBackfillProgress`.
   - `TickDTO` (P0-4) is the only quote shape: `useTicksStore` → `useQuote`/`quoteChange` (T6) → T7, T8, T10, T12 and T17; `QuoteConsumer`/`WatchResult` (T2) match T6.
   - `WatchlistsState`/`Watchlist`/`WatchlistItem`/`RemovedItem` (T2) flow unchanged through the store (T13), rows (T14), search (T15), panel (T16) and picker (T17); items render through `labelOfItem`/`itemTag` (T1).
   - `create(name, tokens?, activate?)` (T13) matches the picker (T17).
   - `showOnDashboard` (T10) matches T12 and T16.
   - `openFullViewChart` (T11) matches `DashboardChart` (T11).
   - `ChartView` renders `ChartHeader` with `onBack`, and T11 uses that.
   - Fixed in this pass: the legend assertion relied on whitespace between spans and now checks each part; the picker test hit an ambiguous "Select RELIANCE" (NSE + BSE) and now uses the first match or a segment-restricted list; `controller.onLiveBar` was added so the legend follows live bars.
4. **Review Focus.** Each of the five cases has a named test in its owning task (T5 ×2, T10, T4, T6 + T14). Other failure modes are also covered:
   - offline cached data (T5, T10)
   - a lazy-load failure loop (5 s back-off in T5)
   - stale search responses (T9)
   - an expired instrument in a watchlist (T14)
   - the attribution logo navigating the webview (Global Constraints, T3)
5. **Canonical contract.** Every P0-3/P0-4 name is the canonical one from the consistency review (contract §C–§G) and is listed once under "Consumes"; the ordering constraints are under "Ordering". Only `api/backend.ts`, `api/wails.ts` and `api/mock/*` touch the generated bindings. Open item: FR-SHELL-004 AC5 after a restart (see the Coverage row).
