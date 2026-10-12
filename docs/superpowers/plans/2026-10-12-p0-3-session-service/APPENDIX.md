## Coverage

Every acceptance criterion of this plan's FRs, with its test or the phase that owns it.

| FR / AC | Covered by (task → test) | Notes |
|---|---|---|
| FR-ACCT-001 AC1 | T11 `TestProfile_MatchesKiteAndIsReadOnlyCopy`; T24 `/user/profile` route | Read-only by construction (copy); non-editable fields are P0-4 |
| FR-ACCT-001 AC2 | T17 `TestSettingsFacade` (persist without Save) | The toggles themselves ship with FR-ACCT-002 (P4) / FR-ACCT-003 (P3) |
| FR-ACCT-001 AC3 | T7 `TestLogin_LoopbackCallbackCompletesLogin` (09:02 → `expiresAtMs` = 06:00 next day) | "Expires 06:00 tomorrow" wording is P0-4 formatting |
| FR-ACCT-004 AC1 | T10 `TestLogout_InvalidatesDeletesAndStops`, `TestKcAuth_InvalidateSendsDelete`; T24 asserts `DELETE /session/token` | |
| FR-ACCT-004 AC2 | — | P4: deployments subscribe to `session.state` (`logged_out`) and pause; the dialog text is P0-4/P4 |
| FR-ACCT-004 AC3 | P0-1 `TestStart_FirstRunCreatesEverything` / `TestRun_SecondInstanceExits` (lock); T23 `TestForwardToRunningInstance_DeliversArgs`, `TestSecondInstance_RoutesRedirectToLogin`; T26 manual procedure | |
| FR-ACCT-005 AC1 | T6 `TestRestore_FreshInstall_NoCredentials` | Setup view first is P0-4 |
| FR-ACCT-005 AC2 | T6 `TestSaveCredentials_SecretOnlyInSecretStore`; T7 log assertion; T24 "secret never sent"; T23 `TestCaptureStdStreams_ScrubsQueryStrings` | |
| FR-ACCT-005 AC3 | T10 `TestRemoveCredentials` | |
| FR-ACCT-006 AC1 | T7 `TestLogin_LoopbackCallbackCompletesLogin`; T22 `TestApp_FocusesWindowOnLoginSuccess`; T24 end to end; T25 real account | "Home + Live within 3 s" is measured in P0-5's exit demo |
| FR-ACCT-006 AC2 | T7 `TestLogin_PasteWhenListenerBlocked`, `TestLogin_AfterListenerTimeout_PasteStillWorks` | |
| FR-ACCT-006 AC3 | T8 `TestExpiry_At0600_WithTickerRunning`, `TestExpiry_ClockJumpAfterSuspend`; T14 `TestRun_StopsSyncWhenSessionExpires` | Banner is P0-4 |
| FR-ACCT-006 AC4 | T6 `TestRestore_ValidTokenSkipsLogin` | |
| FR-ACCT-006 AC5 | T8 `TestTokenException_EntersExpiredOnce`; T6 `TestRestore_TokenRevokedElsewhere_IsExpired`; T11 `TestProfile_TokenExceptionExpiresSession` | |
| FR-ACCT-007 AC1 | T9 `TestProbe_PersonalPlan`, `TestIsNoDataPermission_Variants`; T6 `TestRestore_PersonalPlan_DoesNotStartFeed`; T16 `TestCandles_PersonalPlan_ServesCacheAndNeedsDataPlan`; T19 `TestSync_PersonalPlan_PollsLTPAndStopsOnPermissionError` | Status text is P0-4 |
| FR-ACCT-007 AC2 | T9 `TestProbe_ConnectWhenHistoricalOK`; T6 `TestRestore_ValidTokenSkipsLogin` (feed starts) | |
| FR-ACCT-007 AC3 | T16 `TestCandles_PersonalPlan_ServesCacheAndNeedsDataPlan` (cached candles served) | Backtest over cache is P2 |
| FR-ORD-001 AC1 | T12 `TestBuildBook_SectionsSourceAndOrder` (Source from tag) | Deployment-name sources are P4 (journal) |
| FR-ORD-001 AC2 | T19 `TestSync_OrderPlacedOnKiteWebAppearsWithin30s` (no ticker at all) | |
| FR-ORD-002 AC1 | T12 `TestBuildPositions_TotalAndLive` | |
| FR-ORD-002 AC2 | — | P4: `ConvertPosition` is an order write, excluded by G-0 |
| FR-ORD-003 AC1 | T12 `TestBuildHoldings_Empty` | Copy "No holdings" is P0-4 |
| FR-ORD-003 AC2 | T12 `TestBuildHoldings_RowValues` | |
| FR-ORD-014 AC1 | T19 `TestSync_OrderPlacedOnKiteWebAppearsWithin30s`, `TestSync_OrderUpdatesImmediateAndOutOfOrderIgnored`, `TestSync_PollReconcilesAndLogs` | |
| FR-HOME-002 AC1 | T12 `TestBuildHoldings_HomeCardAC1` | |
| FR-HOME-002 AC2 | T12 `TestBuildHoldings_LiveLTP`; T19 `TestSync_TickUpdatesHoldingsSummary`; T21 coalescing (≤ 100 ms) | |
| FR-HOME-002 AC3 | T12 `TestBuildHoldings_Empty` | |
| FR-HOME-002 AC4 | — | P0-4 (card click navigation) |
| FR-HOME-002 refresh rules | T19 `TestSync_HoldingsRefreshEveryFiveMinutes`, `TestSync_CNCFillRefetchesHoldingsAndPositions`, `TestSync_PersonalPlan_…` | |
| FR-SHELL-020 AC1/AC3 (data) | T6 `TestAckRisk`, `TestRestore_ValidTokenSkipsLogin`; T7 (`riskAckRequired` after login) | View is P0-4 |
| FR-WATCH-001…006 (backend) | T13 `TestWatchlistsRepo`; T15 `TestWatchlists_*`; T29 `TestWatchQuotes_LingerThenUnsubscribe`; T6 `TestFeed_RemembersAndReplaysAcrossLogins` (pre-login sets survive) | UI ACs are P0-5 |
| FR-WATCH-006 polled fallback / FR-SHELL-004 AC5 (last quotes after restart) | T29 `TestWatchQuotes_PolledFallbackPublishesTicks`, `TestWatchQuotes_SnapshotSurvivesRestart` | Index strip rendering is P0-4 |
| Chart ranges, manual refresh, live chart subscriptions (P0-5 `Chart.*`, `Instruments.*`) | T16 `TestCandles_TradingDaysResolvesFromCalendar`, `TestRefreshCandles_FetchesAndUpsertsManual`, `TestRefreshCandles_Refusals`, `TestOpenCloseLive_TracksBuilderOnce`, `TestInstruments_SearchTagsByTokensByIdentifier` | Chart UI is P0-5 |
| FR-ACCT-013 AC1–AC3 (secret storage choice, encrypted file, unlock, move to keyring) | T27 `TestSecretStorage_EncryptedFileSetupAndUnlock`, `TestSecretStorage_RefuseStorage`, `TestMigrateToKeyring`; T23 `secretSwitch` | Setup view and passphrase dialogs are P0-4 |
| FR-SHELL-019 (shortcuts, window geometry, About) | T28 `TestDefaultShortcuts_MatchContract`, `TestParseBinding`, `TestConflicts`, `TestShortcuts_RebindConflictAndReset`, `TestUiState_GetSet`; T22 `TestApp_RestoresAndSavesWindowGeometry`; T33 `TestParseNotices`, `TestAppInfo_Status_ReconnectFeed` | Shortcut provider, help view and About panel are P0-4 |
| FR-DATA-002 (data stats, clear candles, refresh instruments) | T30 `TestDataStats_ClearCandles_RefreshInstruments` | Settings → Data view is P0-4 |
| FR-DATA-005 AC1–AC4 (backup now, inspect, restore + restart, daily backup) | T30 `TestBackupNow_RunsStagesAndInspects`, `TestRestoreBackup_StagesAndRefusals`, `TestScheduledBackup_DailyAfter1600AndOnQuit`; T22 `TestApp_QuitsAfterRestoreJob`; T23 `TestRun_RestartExitCode` | Archive format, prune and restore internals are P0-1 |
| FR-DATA-001 AC1–AC3 (calendar import, overrides) UI backend | T31 `TestCalendarInfo_ShippedCalendar`, `TestImportCalendar_AllOrNothing`, `TestUpsertAndRemoveCalendarEntry` | Parsing and the merged calendar are P0-2; the view is P0-4 |
| FR-SHELL-018 AC1–AC3 (logs folder, support bundle, crash report) | T32 `TestDiagnostics_LogsFolderAndSupportBundle`, `TestDiagnostics_CrashReports`; T22 `TestDiagnostics_OpensLogsFolder` | Bundle contents and crash capture are P0-1 |
| Contract §C/§D (bindings and events verbatim) | T33 `TestContract_EventCatalogue`, `TestContract_BoundStructs`; T21 `TestEventSources_MatchCatalogue`; T22 `TestOptions` | |
| C-01 | T9, T16, T19, T29 (degraded mode, LTP polling for portfolio and quotes) | |
| C-02 | T1 `TestNextExpiry`; T8 | |
| C-03 | T19 `TestSync_ShowsCachedBookWhenKiteUnreachable`; reconciliation on every login | |
| C-04 / NFR-05 / G-0 | T24 `TestFullSessionCycle_SendsNoOrderWrites`, `TestIsWrite`, Step 5 `go tool nm` check | No order path exists in P0 |
| NFR-04 | T6, T7, T11 (avatar host filter, PII-safe `LogValue`), T23 (std-stream scrub), T24 | |
| NFR-09 | T19 (30 s poll regardless of feed, reconciliation, cache) | Ticker reconnect itself is P0-2 |

## Self-review

1. **Spec coverage.** Every §12 row in scope (3a, 4b, 5d backend) and every brief item maps to a task: credentials T2/T6/T10; login T3/T4/T7; lifecycle T8; probe and degraded mode T9/T16/T19; logout T10; profile T11; façade T14–T17/T20; bindings, event bridge and single instance T21–T23; `orders.Sync` T12/T18/T19; FR-HOME-002 summary T12/T19; zero-writes T24; S0-1 T25; S0-8 T26; secret storage T27; shortcuts and UI state T28; `Quotes.Watch` T29; data/backup/restore T30; calendar T31; diagnostics T32; `System.*`, regenerated bindings and the contract test T33. The Coverage table above lists every AC. ACs owned by P0-4/P0-5/P3/P4 are marked with their owner.
2. **Placeholders.** No step says "add validation" or "similar to". The only empty cells are the S0-1/S0-8 *results* tables, which a spike records by design, each with its exact command.
3. **Type consistency, checked by reading every task's Interfaces against its code:**
   - `kite.LoginStart`/`CredentialsInfo`/`LogoutResult`/`Profile` are aliased in `service` (T14) and used by `internal/app/api` (T22).
   - `orders.Scope` lives in T12, so T14's `PortfolioSync` compiles before T19; `kite.Position`/`Holding` (T12 `portfolio_types.go`) exist before T19's cache code names them.
   - `settings.Register` (T2 `registry.go`) exists before the first key declared through it (T2 `credentials.go`), and T17 consumes `Names`/`GetJSON`/`SetJSON` from the same file.
   - `storetest.Open` (T5) is used by every test outside `package store` (T6, T14, T19, T22, T24); `package store` tests of T5/T13/T18 are external (`store_test`) for the same reason.
   - `feed.Feed` (T6) satisfies `kite.FeedControl` (T6), `service.LiveSubscriber` (T14) and `orders.LiveTokens` (T19) — all three carry the P0-2 `owner string` parameter and `Release`.
   - `service.EvSessionState` (T21) is used by `app.emitter` (T22).
   - `loginViaPaste` (T7 test helper) is used by T8, T10 and T11.
   - Fixes applied during review: tasks reordered so dependencies come first (builders → watchlists repo → service core); `Scope` moved out of `sync.go`; `ratelimit` import path corrected to `internal/kite/ratelimit`; local variables named `sync` renamed so they no longer shadow the package; `initials` now counts runes, not bytes.
4. **Review Focus.** Each of the five items has a named test in its owning task (T7, T8/T14, T9/T16/T19, T23 + P0-1 T15).
5. **Consistency review 2026-10-12.** Every P0-1/P0-2 name this plan consumes was realigned to the sibling plans' "Produces" blocks (see "Consumes"): `bus.New(nil)`, `bus.SubOpts[T]`, `bus.Unsubscribe`; `settings.Load`/`Set(ctx, …)`/`Key[T]` literals + this plan's registry; `db.W`/`db.R` + `storetest`; `clock.Timer` everywhere a timer is awaited (stopped on cancel); `kite.Options`/`SetAPIKey`/`SessionErrorRelay`/`NewAllowListTransport`; the P0-2 ticker API behind `feed`; `candles.Candle.Start`, `candles.Range`, `Store.Read`; `builder.Event{Partial}`; `backfill.Job.OI`; `instruments.Instrument` real fields and `Master.Search(q, limit) []Result`; `kite.TopicOrderUpdate`; P0-1's `app.Start`/`store.AcquireLock` instead of a second lock.

**Decisions flagged for the consistency reviewer (P0-1/P0-2/P0-4/P0-5):**
- Event coalescing lives in `internal/service/events.go`, not `internal/app/bridge.go`, to keep rule 7 (T21). `session.ready`/`session.loggedOut`/`session.expired` from ARCH §3.2/§5.3 are all `session.state` with a `state` field.
- `ErrorFormatter` returns a JSON **string** (ARCH §8.4 literal). P0-4's `src/api` wrapper must `JSON.parse` the rejection value.
- `orders.Sync` polls orders and positions every 30 s throughout a ready session. This is a superset of FR-ORD-014's "while orders are open", chosen so a missed order update can never hide a Kite-web order (cost: 2 requests per 30 s).
- The single-instance lock and the last-exit marker are P0-1's (`store.AcquireLock` inside `app.Start`, `Boot.PreviousExit`/`CrashedLastTime`). This plan adds only the D-Bus hand-off of a second launch (T23); P0-1's `cmd/tradeloom/main_test.go` is unchanged and keeps passing.
- P0-1's `RedactHandler` must replace registered secrets that occur as **substrings** of a string value, not only exact matches: library lines such as gokiteconnect's `Request failed: …?access_token=…` embed them. T23 additionally strips URL query strings.
- The v1 schema has no holdings cache. Holdings are in memory only (re-fetched on login and every 5 min).
- `cmd/tradeloom/main.go` keeps P0-1's order (`app.Start` → flock before logging and store open) and never calls `wails.Run` in a second instance (T23 rationale, `frontend.go:295-303`). Exit code 3 (restart after a restore) is new; `main` re-execs with `syscall.Exec`.
- `.go-arch-lint.yml` is replaced in full (T23 Step 5): P0-2 declared no components, so its packages and vendors (`uuid`, `websocket`) are listed here together with this plan's (`api`, `service`, `feed`, `storetest`, `kitemock`, `orders`, `notify`, vendor `godbus`).
- P0-2's `ticker.Manager` is single-use per login and refuses nothing before `Start`; pre-login subscriptions (Personal plan, watchlists before the first login) are held by `feed.Feed` (T6) and replayed on every `Start`. `service.WatchQuotes` (T29) polls such tokens over REST until `feed.state` is `live`, and `OpenLive` (T16) treats `feed.ErrNotRunning` as success for the same reason.
- `(*kite.Client).SetAPIKey` (T6) rebuilds P0-2's inner gokiteconnect clients and therefore drops the access token, exactly like `kite.New`; the Session always sets the key before the token. P0-2's `APIKey()` reads the key without the mutex — a race only if the key changes during a call, which the Session's state machine prevents (credentials change only while logged out).
- The Session always talks to one `service.SecretSwitch` (T27): T23 `secretSwitch` builds it from P0-1's detection (keyring at once; an existing `secrets.enc` starts locked; "choose" starts as `secrets.NewNone()`), and `Session.UseEncryptedFile` / `Unlock` / `RefuseSecretStorage` / `MigrateToKeyring` swap the backend behind it at runtime. While locked, `Restore` is deferred and `session.state` reports `locked`.
- A finished restore ends with `job.progress{kind: restore, stage: restart}`; the App quits, `run` returns exit code 3 and `main` re-execs (T22/T23). P0-1's `boot.Shutdown` then runs over an already closed `app.db` (`CloseForRestore`); `(*sql.DB).Close` on a closed handle returns nil, and the process lock is still held, so the shutdown sequence completes.

**APIs verified against source** (`go mod download` into the scratchpad; file references in each task):
- gokiteconnect v4.4.3:
  - `GetLoginURL` and `GetLoginURLWithparams`;
  - `GenerateSession` (checksum computed inside), `InvalidateAccessToken` (DELETE with query parameters);
  - `GetUserProfile`, `GetUserMargins` (not used: no P0 FR needs funds);
  - `GetOrders`, `GetPositions`, `GetHoldings`, `GetHistoricalData`, `GetLTP`;
  - `Error{Code, ErrorType, Message, Data}` and the error-type constants;
  - `models.Time` parsing;
  - ticker `New`, `SetRootURL`, `SetAutoReconnect`, `SetReconnectMaxRetries`, `SetConnectTimeout`, `OnConnect`, `OnError`, `OnNoReconnect`, `ServeWithContext`, `Stop`, `Conn`, and the auto-reconnect-off nil-conn hazard.
- wails v2.16.0:
  - `options.App` fields, `ErrorFormatter`, `SingleInstanceLock`, `SecondInstanceData`;
  - runtime `EventsEmit`, `BrowserOpenURL`, `WindowShow`, `WindowUnminimise`;
  - the Linux single-instance D-Bus protocol and its ordering relative to `OnStartup`;
  - binding generation paths and Promise typing;
  - `wails generate module -tags`; `logger.INFO`/`ERROR`.
- gofrs/flock v0.13.1 `New`/`TryLock`/`Unlock` (used by P0-1's `store.AcquireLock`; this plan no longer imports it).
- godbus v5.2.3 `ConnectSessionBus`, `WithContext`, `Object`, `Call`, `CallWithContext`, `Store`, `Export`, `RequestName`.
- google/uuid v1.6.0 `NewV7`.
- Kite docs (fetched 2026-10-12): `redirect_params` is sent back to the redirect URL; `request_token` is one-time and lasts "a few minutes"; the checksum definition; the documented exception list has **no** `PermissionException`; no position P&L formula is documented.

**Verification performed on this plan (2026-10-12, before the consistency review):** every Go code block was extracted into a scratch module, together with minimal working stand-ins for the then-assumed "Consumes" API: a channel bus, a fake clock, in-memory settings, and a SQLite store with the §4.2 v1 tables for sessions, watchlists and caches. The consistency review then rewrote every call site against the real P0-1/P0-2 signatures (self-review item 5); the first `go build -tags webkit2_41,noorders ./...` against the merged tree is Task 23 Step 6.
- `GOOS=linux go vet -tags webkit2_41,noorders,kitelive ./...` is clean, including `cmd/tradeloom/wire_p03.go`.
- `go test -race -count=3` passes for `internal/store`, `internal/kite`, `internal/kite/mock` (zero-writes), `internal/orders`, `internal/service` and `internal/app/eventsgen`.
- `internal/app`, `internal/app/api` and `internal/notify/desktop` pass with godbus replaced by a no-bus fake. The two real-bus tests skip there and run under `dbus-run-session` in CI (T26).
- Two test races found this way are fixed in the plan: `orders` env cleanup ordering, and the `service.Run` supervisor test.

**Findings for other sub-plans:**
- `modernc.org/sqlite` **v1.60.1 declares `go 1.26.0`** in its go.mod (proxy.golang.org, checked 2026-10-12), which conflicts with the Go 1.25 pin (ARCH §2.1/§13.2). The newest release with `go 1.25.0` among those checked is v1.50.0. P0-1 must pick one: bump Go, or pin sqlite ≤ the last 1.25-compatible release.
- `github.com/godbus/dbus/v5` v5.2.3 does not compile for darwin (`conn_darwin.go:29: not enough return values`). That is harmless for the Linux-only target, but `internal/app` and `internal/notify` build only in the Linux container.

**Could not verify** (each is pinned by a test or a spike step rather than assumed silently):
- Whether the developer console accepts an `http://127.0.0.1` redirect (S0-1 row 1).
- Which query keys Kite appends beyond `request_token` (S0-1 row 3).
- The exact error a reused or expired `request_token` returns (S0-1 rows 5–6; `TestMapExchangeError`).
- The error a Personal-plan key gets from historical data and from the ticker (S0-1 rows 7–9; `TestIsNoDataPermission_Variants` covers every plausible spelling).
- Whether `GET /quote/ltp` works on a Personal key (FR-HOME-002 claims it; A07 §A1 says "no live market data"; T19 handles both).
- The API key/secret format (validation kept loose).
- NIFTY 50 token 256265 (S0-1 row 11).
- The `org.freedesktop.Notifications.GetServerInformation` reply shape, which comes from the freedesktop spec rather than a Go module (S0-8 results).
- That Wails' dev-mode logging never prints binding arguments. `LogLevelProduction=ERROR` is set; dev builds are developer-only.
- What P0-1's `store.Restore` leaves at `RestoreResult.ReplacedDir` (T30 asserts only that the path exists and that the restored `app.db` opens) and whether `store.InspectBackup` reports a truncated archive as `ErrBadArchive` or another error (T30 treats every inspect error as "not a Tradeloom backup").
- Whether `modernc.org/sqlite` lets `(*sql.DB).Close` be called twice without error on every version (T23 `CloseForRestore` + `boot.Shutdown`); database/sql documents it as idempotent.
