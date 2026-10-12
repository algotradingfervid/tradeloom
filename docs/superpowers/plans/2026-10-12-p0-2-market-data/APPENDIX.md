## Acceptance-criteria coverage

| Requirement | AC | Proving test (task) | Notes |
|---|---|---|---|
| FR-DATA-001 | AC1 holiday | `TestHolidayLabelAndNoSession` (4) | Calendar reports Holiday, no session/anchor. "No scanner/deployment evaluation or replay" is enforced by their schedulers asking `IsTradingDay` — later sub-plans (P3/P4). |
| FR-DATA-001 | AC2 special session | `TestSpecialSessionAnchorsCandlesAndSquareOff` (4), `TestMuhuratCandlesBelongToTheirDate` (10), `TestSessionAnchorsHourlyAndSpecialSession` (19) | 5-minute candles 18:15 … 19:10–19:15; square-off candle contains `19:15 − N`. SO tables themselves are P2/P4. |
| FR-DATA-001 | AC3 post-closing | `TestPostClosingIsCashOnly` (4) | NSE 15:45 = post-closing, NFO 15:45 = closed. Order acceptance/refusal text is FR-ORD-015 (P4). |
| FR-DATA-001 | AC4 import "Row 7: invalid time" | `TestImportRefusesInvalidTimeAndNamesRow`, `TestParseRejections` (3); `TestCalendarOverridesRoundTrip` (5) | `ParseFile` returns no entries on error, so nothing is merged or persisted; the import UI is P0-4. |
| FR-DATA-001 | AC5 inferred non-trading day | `TestInferNonTradingDayWithoutNifty50Candle`, `TestInferLeavesListedAndCandleDaysAlone` (5); `TestDailyCheckForCalendarInference` (14) | Tooltip text "Non-trading day (inferred)" from `LabelAt`. P0-3 wires `Infer` with `backfill.DailyCheck(NIFTY 50 token)`. |
| FR-DATA-001 | AC6 stale | `TestStaleWhenNext30DaysNotCovered` (4) | `Stale(now)`; the once-per-day WARNING toast is P0-3 (service) + P0-4 (toast). |
| FR-DATA-002 | AC1 one fetch for two consumers | `TestSameWindowFromTwoConsumersIsFetchedOnce` (14) | 200-day 15-minute window, chart + backtest. |
| FR-DATA-002 | AC2 historical replaces tick-built | `TestHistoricalReplacesTickBuiltCandle` (14), `TestHistoricalReplacesTickBuiltButNotViceVersa` (11) | |
| FR-DATA-002 | AC3 purge guarded by runs | `TestPurgeRefusesReferencedRunsUntilConfirmed`, `TestClearAndStatsAndBudget` (11) | `RunsReferencedError.Runs` names the runs; the confirmation dialog is P0-4 (Settings → Data). |
| FR-DATA-002 | AC4 < 50 ms / < 5 ms at 500 × 2 y | `TestS02` (13, reference machine), `TestPerf_StoreQueriesAtCIScale` (13, CI at 10× tolerance) | |
| FR-DATA-002 | AC5 weekly golden, Muhurat date | `TestWeeklyGolden`, `TestMuhuratCandlesBelongToTheirDate` (10), `TestReadResampledWeekAndIncompleteTail` (11) | |
| FR-DATA-002 | AC6 one monthly checksum changes | `TestChecksumChangesOnlyForTouchedMonth` (11) | |
| FR-DATA-003 | AC1 interactive first, ≤ 3 historical/s | `TestPriorityThenFIFO`, `TestSlidingWindowNeverExceedsRate`, `TestInteractiveOvertakesQueuedBacktestsAndRateHolds` (6); `TestInteractiveJobOvertakesQueuedBacktests` (14) | |
| FR-DATA-003 | AC2 429 back-off on one class, counter | `TestBackoffDoublesToThirtySeconds`, `TestBackoffIsolatedToClass` (6); `TestRateLimitedHistoricalBacksOffThatClassOnly` (9) | Counter in `ratelimit.Usage.Classes[].HTTP429` on `limits.usage`; FR-ACCT-009 display is P0-4. |
| FR-DATA-003 | AC3 resume 15 of 40 windows | `TestResumeFetchesOnlyRemainingWindows` (14) | |
| FR-DATA-003 | AC4 TokenException pauses all | `TestPauseAllAndResume` (6), `TestTokenExceptionPausesAllQueuesAndNotifies` (9), `TestTokenExceptionMidBackfillRequeuesWindow` (14) | `Options.OnSessionError` → P0-3 session state; banner within 5 s is P0-3/P0-4. |
| FR-DATA-003 | AC5 overlapping requests → non-overlapping windows | `TestOverlappingRequestsProduceNonOverlappingWindows` (14) | |
| FR-DATA-003 | AC6 no blind PlaceOrder retry | `TestNonIdempotentCallsAreNeverRetried`, `TestReadClientNeverCallsOrderWriteRoutes` (9) | Client side only; the `GetOrders`-by-tag check is FR-ORD-006 (P4). |
| FR-DATA-004 | AC1 3 s fast → −3 s ± 0.5 s, candles at exchange time ± 0.5 s | `TestOffsetFromTicksWithSystemClock3sFast` (2), `TestFiveMinuteCandleFromTicksClosesAtBoundaryPlusGrace` (19), `TestS07` (21) | |
| FR-DATA-004 | AC2 UTC system zone → IST; 06:00 IST = 00:30 UTC | `TestISTIsKolkataEvenWhenLocalIsUTC`, `TestExpiryAt0600ISTFiresAt0030UTC` (1) | Log timestamps in IST: P0-1 logging handler. |
| FR-DATA-004 | AC3 7 s offset → suspend until < 2 s | `TestSkewHysteresisAndAcknowledge` (2) | `Offset.Skewed` on `clock.offset`; DI suspension is P4. |
| FR-DATA-004 | AC4 HTTP Date fallback with source | `TestOffsetFromHTTPDateWhenNoTicks`, `TestTicksWinOverHTTPAndExpireAfterWindow` (2), `TestDateHeaderFeedsClock` (9), `TestS07` (21) | `Offset.Source` for the tooltip (P0-4). |
| FR-WATCH-008 | AC1 once after 08:30, not on holidays | `TestDueOncePerTradingDayAfterDumpTime`, `TestRunDownloadsAtStartThenAtNextDumpTime` (16) | |
| FR-WATCH-008 | AC2 BANKNIFTY → NIFTY BANK top 3; no options | `TestSearchRanking` (15), `TestImportKeepsOnlyScopeAndDropsExpired` (15) | |
| FR-WATCH-008 | AC3 < 100 ms | `TestSearchUnder100msOnFullSizeMaster` (15) | 30 000 rows (above the filtered real dump). |
| FR-WATCH-008 | AC4 failed download keeps yesterday's list, WARNING with date | `TestFailedDownloadKeepsPreviousListAndWarnsWithDate` (16) | Toast rendering is P0-4. |
| FR-WATCH-008 | AC5 contract expired yesterday hidden | `TestExpiredYesterdayDisappearsFromStaleList` (15) | |
| FR-WATCH-008 | AC6 lot size 65 → 75; 2 lots = 150 | `TestLotSizeChangeFollowsNewDump` (15) | `UnitsForLots`; order ticket and Scalper preset are P4/P5. |
| FR-WATCH-013 | AC1 `RELIANCE OCT FUT`, `NFO:RELIANCE26OCTFUT` | `TestDisplayNamesAndTags` (15) | |
| FR-WATCH-013 | AC2 `NIFTY 50`, tag `INDICES` | `TestDisplayNamesAndTags` (15) | |
| FR-WATCH-013 | AC3 one formatter for every rendered name | backend: `Instrument.DisplayName` is set only in `instruments.Filter`/`deriveFutures` | Rendered-row check is P0-5. |
| FR-WATCH-014 | AC1/AC2 one tag per instrument everywhere | `TestDisplayNamesAndTags` (15) | Tag derived only by `tagOf(exchange, segment)`; rendered-row check is P0-5. |
| C-07 | 3 × 3000, refuse beyond | `TestBudgetRefusesWithBreakdownAndNeverEvicts` (17), `TestS06` (20) | |
| C-08 | windows, 3 req/s, resumable | `TestWindowSizesPerInterval`, `TestResumeFetchesOnlyRemainingWindows` (14), Task 6 tests | |
| C-05 / C-06 | order rates, market protection | — | No order code in P0 (G-0); `ratelimit.ClassOrders` default 8/s is the hook for P4's throttle. |
| C-09 | no HTTP outside Kite | Task 22 Step 4 greps; P0-1 depguard/forbidigo | |
| NFR-08 | schedules on the app clock; close + 1 s | Tasks 1, 2, 19 tests | |
| NFR-09 | reconnect + resubscribe; resumable backfill | `TestReconnectResubscribesAllTokensWithModes` (17), `TestManagerOverRealTickerAndMockServer` (18), `TestResumeFetchesOnlyRemainingWindows` (14) | |
| NFR-12 | raw candles | candles are stored exactly as Kite returns them (no adjustment code exists) | |
| NFR-15 | no option/MCX/currency row survives import | `TestImportKeepsOnlyScopeAndDropsExpired` (15), `TestRefreshPersistsScopedRows` (16) | |
| NFR-07 | backtest performance | — | P2; P0-2 supplies the read path measured in Task 13. |

## Produces for P0-3 and P0-5 (summary)

P0-3 (service/app) wires: `clock.New` (+ `ObserveHTTPDate` as `kite.Options.DateSink`, `ObserveTick` as `ticker.Options.Observer`, `TopicOffset`); `calendar.NewDefault`, `SetOverrides`/`MergeOverrides`/`DeleteOverride` + `ToRecord`/`FromRecord` ↔ `store.CalendarOverrideRow`, `Infer`, `Stale`, `LabelAt`, `Version`; `ratelimit.New`/`Run`/`Resume`/`TopicUsage`; `kite.New`/`SetAccessToken`/`Call`/`ReadClient`/`Classify`/`TopicOrderUpdate` (and `OnSessionError` → session expired); `sqlite.Open` (candles.db) as `candles.Store` + `candles.JobStore`; `backfill.New`/`Start`/`Submit`/`Wait`/`Ensure`/`DailyCheck`/`WaitStopped`/`TopicProgress`; `instruments.NewMaster`/`NewRefresher`/`Load`/`Run`/`Refresh`/`Search`/`ByID`/`ByToken`/`ExchangeOf`/`TopicStatus`; `ticker.New(KiteDialer(apiKey, token, nil))`/`Start`/`Subscribe`/`Release`/`Raw`/`Usage`/`Reconnect`/`Stop`/`TopicTicks`/`TopicFeed`/`ErrBudget`; `builder.New`/`Track`/`Untrack`/`Run`/`TopicClosed`/`TopicForming`. Shutdown order (§3.3): `ticker.Manager.Stop` → cancel the backfill/limiter context → `backfill.WaitStopped` → close `candles.db`.

P0-5 (chart, watchlists) consumes the JSON payloads `ticker.Tick`, `ticker.FeedState`, `builder.Event`, `candles.Candle`, `backfill.Progress`, `instruments.Instrument`/`Result`, `clock.Offset`, `calendar.Label` through P0-3's bindings.

## Self-review

- **Spec coverage:** every AC of FR-DATA-001…004, FR-WATCH-008, FR-WATCH-013 and FR-WATCH-014 maps to a test or to a named later sub-plan in the table above; ARCHITECTURE §12 rows 1d (Tasks 1–5), 2b (Tasks 6–9), 2c (Tasks 10–13), 3b (Task 14), 3c (Tasks 15–16), 4a (Tasks 17–19) and spikes S0-2 (13), S0-6 (20), S0-7 (21) each have a task.
- **Placeholders:** none. The spike records are produced by shell commands from harness output; the only human inputs are the developer's Kite credentials and the exchange-circular check in Task 3 Step 5 and Task 15 Step 5, each with an exact procedure.
- **Type consistency:** the code blocks are the files of a scratch module that compiled and passed `go vet` and `go test -race -short` (stressed with `-count=8`) on 2026-10-12 against stubs implementing exactly the "Consumes (from P0-1)" signatures; "Produces" blocks are extracted from those files.
- **Deviations from ARCHITECTURE, recorded for the reviewer:** (1) `backfill_windows.priority` and table `backfill_links` added to the §4.3 schema (FR-DATA-003 AC1/AC5); (2) `candles.Store` gains `UpsertCovered`, `Coverage`, `SetBudget`, `AddRunRef`, `Clear`, and `ReadResampled` marks an unfinished last period incomplete; (3) `kite.HistoricalPicker` added beside `ReadClient`; `ReadClient.Alerts` takes the `filters map[string]string` argument that gokiteconnect's `GetAlerts` requires; (4) instrument search ranks the in-memory master rather than FTS candidates (reason in Task 15); (5) the ticker Manager guards its state with a mutex rather than a mailbox actor, and owns reconnects itself (library facts above); (6) purge works in whole calendar months so monthly checksums stay exact; (7) `kite.Time` alias added so non-kite tests build `HistoricalData` without importing gokiteconnect.
- **Reconciled with P0-1 (its plan file, 2026-10-12):** `bus.New(*slog.Logger)`, generic `bus.SubOpts[T]`, `store.Open(ctx, path, Options)`, `DB.W`/`DB.R`, `NewMigrator(fs.FS) (*Migrator, error)` + `Apply(ctx, db, from)`, `clock.SystemNow` kept, modernc.org/sqlite v1.59.0.
- **Open items for P0-3 (its Consumes block assumed different P0-2 names):** P0-3 adapts its call sites to the Produces blocks here — notably `clock.Clock.At` returns a `Timer` (not a channel); `ticker.Manager` is built by `ticker.New(Options{Dial: KiteDialer(apiKey, token, nil), …})` per session and `Subscribe(c, owner, tokens, mode)` takes an owner id so two watchlists sharing a token do not unsubscribe each other (ARCHITECTURE §5.6 shows the signature without the owner); topics are `ticker.TopicTicks`/`TopicFeed`; `ticker.Tick` is the app's JSON tick, not `models.Tick`; `kite.New(kite.Options{…})` with an `OnSessionError` field; `candles.Store.Read` (not `Range`) returning `[]candles.Range`; `builder.Event`; `instruments.(*Master).Search(q, limit) []Result`.
