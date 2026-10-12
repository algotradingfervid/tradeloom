# Phase 0 cross-plan consistency review (2026-10-12)

Scope: the five P0 sub-plans (`2026-10-12-p0-1` … `p0-5`), the index and `docs/architecture/ARCHITECTURE.md`.
Goal: an executor can run P0-1 → P0-2 → P0-3 → P0-4 → P0-5 without hitting a missing name, a wrong signature or a
wrong binding/event. Precedence applied: P0-1 and P0-2 Produces win for Go (both were executed in a scratch repo
and pass); P0-3 wins for Wails bindings and events but was extended with everything P0-4/P0-5 legitimately need;
P0-4 Tasks 1–6/12/19 win for the shell components P0-5 consumes. Verification script:
`docs/superpowers/plans/check_p0_interfaces.py` (output in §5).

## 1. Version pins (ARCHITECTURE.md)

| Module | Was | Now | Evidence (proxy.golang.org `@v/<v>.mod`, fetched 2026-10-12) |
|---|---|---|---|
| `modernc.org/sqlite` | v1.60.1 | **v1.59.0** | v1.60.1 and v1.60.0 declare `go 1.26.0`; v1.59.0 declares `go 1.25.0` (last Go-1.25 release). P0-1/P0-2 already used v1.59.0. |
| `golang.org/x/crypto` | v0.58.0 | **v0.55.0** | v0.58.0, v0.57.0, v0.56.0 declare `go 1.26.0`; v0.55.0 declares `go 1.25.0` (last Go-1.25 release). P0-1 already used v0.55.0. |

The project's `go.mod` says `go 1.25.0` with `GOTOOLCHAIN=local`, so a dependency whose `go` directive is 1.26 refuses
to build. The reason is recorded next to each pin in ARCHITECTURE §4.1 and §9.1.

## 2. Canonical contract (summary — the full text each editor worked from is reproduced in §7)

- Go: P0-3 now uses P0-1's `bus.New(log)`, generic `bus.SubOpts[T]`, `bus.Unsubscribe`, `settings.Key[T]{…}` literals +
  `settings.Load(ctx, db.SettingsRepo(), bus)` + `settings.Set(ctx, …)` + `settings.Changed`/`settings.Topic`,
  `store.DB.W/R`, `store.Open(ctx, path, store.Options)`, `app.Start`/`app.Boot` (P0-1 Task 15) and `store.AcquireLock`/
  `store.ErrLocked`; and P0-2's `clock.Timer` (`At` returns a `Timer`, not a channel), `clock.New(clock.Options)`,
  `kite.Options`/`kite.New`, `ratelimit.New(clk, Options)`, `ticker.New(Options{Dial: KiteDialer(...)})` with the
  `owner` parameter on `Subscribe/Unsubscribe` and `Release`, `ticker.TopicTicks`/`TopicFeed`, `kite.TopicOrderUpdate`,
  `ticker.Tick` (the app's JSON tick), `candles.Candle{Start…}`/`Range`/`Store.Read`, `builder.Event`,
  `backfill.Job{…, OI}`/`Progress`, `instruments.Instrument` (real fields) and `Master.Search(q, limit) []Result`.
- P0-3 additions that the realignment required (all real code in P0-3): `settings/registry.go` (`Register`, `Names`,
  `GetJSON`, `SetJSON`, `ErrUnknownKey`), `store/storetest` (`Open(t)`), `kite/portfolio_types.go` (`Position`, `Holding`
  aliases), `kite/client_key.go` (`(*Client).SetAPIKey`), `kite.SessionErrorRelay`, `kite/netguard.go`
  (`NewAllowListTransport`, `AllowedHosts`), package `kite/feed` (one `ticker.Manager` per login; remembers
  subscriptions; `ErrNotRunning`).
- Bindings (11 bound structs): `Session`, `Portfolio`, `Settings`, `Chart`, `Instruments`, `Quotes`, `Watchlists`,
  `Data`, `Calendar`, `Diagnostics`, `System`. Events: the 13 of P0-3 plus `order.update`, `market.status`,
  `job.progress`, `app.navigate`. Settings keys are Go-declared, snake_case (`appearance.follow_system`,
  `ui.last_route`, `chart.default_timeframe`, …). Shortcut registry in `internal/settings/shortcuts.go` with
  P0-4's action ids. Full tables: §7 C–E.
- Frontend: P0-5 consumes P0-4's real exports (`Backend.on`, `parseBackendError`, `useSetting(key, fallback)`,
  `useSessionStore`, `useStatusStore`, `useTicksStore` (the single tick store), `formatSigned/formatPercent/formatCompact`,
  `Skeleton`, `pushToast`, `useShortcut(id, handler)`, `useTheme().tokens`, `shellChildren`/`rootChildren`,
  `ShellLayout` `ticker` prop, `slots.tsx`), extends `Backend` with `chart/instruments/quotes/watchlists`, and mounts
  `/chart-full` outside the shell. P0-4 adds the `lint` and `dev:mock` scripts, `personalSeed` and the `ui-e2e` CI job
  (P0-1 has none).

## 3. Changes made outside the per-plan passes

| File | Where | Before → After |
|---|---|---|
| `2026-10-12-p0-1-platform-core/MANIFEST.md` | Task 1 Step 3 `cmd/tradeloom/main.go` and Task 15 Step 3 `cmd/tradeloom/main.go` (`options.App`) | `Width 1280, Height 800, MinWidth 1024, MinHeight 700` → `Width 1440, Height 900, MinWidth 1280, MinHeight 720` (P0-4/P0-5 lay out against a 1280 px floor; P0-3 Task 22 `Options` uses the same values) |
| `2026-10-12-p0-1-platform-core/MANIFEST.md` | Task 15 Interfaces → Produces | added the window-options line; replaced "P0-3 … moves `Start` into `OnStartup`" with the agreed design: `app.Start` stays before `wails.Run`, P0-3 replaces `internal/app/app.go`/`app_test.go` (`New()`, `Hello`) with `app.New(app.Deps{Boot, Service, …})`, `store.AcquireLock` stays authoritative (`store.ErrLocked` → D-Bus forward) |
| `ARCHITECTURE.md` | §4.1 driver line | `modernc.org/sqlite` v1.60.1 → v1.59.0 with the Go-1.26 reason |
| `ARCHITECTURE.md` | §9.1 `secrets/encfile` | `golang.org/x/crypto` v0.58.0 → v0.55.0 with the Go-1.26 reason |
| `ARCHITECTURE.md` | §3.4 topic table | added rows `orders.changed`/`positions.changed`/`holdings.changed`, `watchlists.changed`, `settings.changed`, `instruments.status`, `market.status`, `job.progress`, `app.navigate` |
| `ARCHITECTURE.md` | §3.4 "UI bridge" paragraph | bridge location `internal/app/bridge.go` → coalescing sources in `internal/service/events.go` (rule 7), `internal/app` supplies only `Emit`; immediate-topic list; `session.ready`/`session.loggedOut` are `session.state` values |
| `ARCHITECTURE.md` | §8.4 | bound structs enumerated (11); events declared in `internal/service/events.go` (`EventCatalogue`), generator hook in `internal/app/events.go` + `internal/app/eventsgen` |
| `2026-10-12-p0-index.md` | "Sub-plans and order" | replaced the one-line P0-4/P0-5 parallelism note with the six ordering constraints (§4); added the precedence rule and the version pins to the shared contract |
| `check_p0_interfaces.py` | new | verification script (§5) |

## 4. Cross-plan ordering constraints (recorded in the index)

1. P0-1 → P0-2 → P0-3 strictly (P0-3 Tasks 27–33 add the bindings both frontend plans consume).
2. P0-4 Task 1 before any P0-5 task.
3. P0-4 Task 2 before P0-5 Tasks 2, 6, 7, 13.
4. P0-4 Tasks 3, 4, 6 before P0-5 Tasks 8–17.
5. P0-4 Task 12 before P0-5 Tasks 10 and 16.
6. P0-4 Task 19 before P0-5 Tasks 18 and 19.
Only P0-5 Tasks 1, 3, 4, 5 may run in parallel with P0-4 Tasks 2–19.

## 5. Verification (script output, final run)

`python3 -I docs/superpowers/plans/check_p0_interfaces.py --ledger <out>` — checks: (1) every Consumes identifier /
TS binding / event / key is produced somewhere or is a library name, (2) no Go function or method is produced with two
different signatures (package + receiver + name), (3) no TS binding has two parameter lists, (4) every TS interface
named like a P0-3 binding DTO has exactly the Go struct's JSON field names, (5) no placeholder markers.

```text
MISSING (0): every consumed Go name, TS binding and event is produced somewhere (or is a library name).
SIGNATURE DRIFT (0): none
DTO FIELD DRIFT (0): every TS interface named like a P0-3 binding DTO has exactly its JSON field names.
PLACEHOLDERS (0): no TBD / TODO / implement later / similar to Task in any Consumes/Produces/code block.
ledger written to /private/tmp/claude-501/-Users-narendhupati-Documents-SoftwareFactory/21e804a3-d544-4473-a9b8-53669f49e163/scratchpad/ledger-final.md
RESULT: PASS
```

`grep -nE "TBD|TODO|implement later|similar to Task" docs/superpowers/plans/2026-10-12-p0-[1-5]-*.md` → no output
(exit 1). The single pre-existing hit (P0-1 self-review sentence "No TBD/TODO") was reworded. Fence counts are even in
all five plans (250 / 260 / 364 / 482 / 288); every task has its commit step with the
`Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` trailer (18 / 22 / 33 / 19 / 19 tasks).

Version pins verified with `curl https://proxy.golang.org/<module>/@v/<v>.mod` (2026-10-12): modernc.org/sqlite
v1.59.0 `go 1.25.0`, v1.60.0 and v1.60.1 `go 1.26.0`; golang.org/x/crypto v0.55.0 `go 1.25.0`, v0.56.0–v0.58.0
`go 1.26.0`.

## 6. Changes made, per plan (before → after)

### 6.1 P0-3 `2026-10-12-p0-3-session-service/MANIFEST.md` — pass A (Go realignment to P0-1/P0-2), 26 tasks

| Task | Before → After |
|---|---|
| Header | "Consumes — assumptions for the consistency reviewer" (stand-in stubs) → "Consumes (from P0-1/P0-2)" with the real signatures copied from the sibling Produces blocks; File Structure lists the files P0-3 adds (`settings/registry.go`, `store/storetest`, `kite/{client_key,session_relay,netguard,portfolio_types}.go`, `kite/feed`); `settings_Change` → `settings_Changed` in the events table |
| 2 | `settings.NewKey[T](name, def, validate)` → `settings.Register(settings.Key[T]{Name, Default, Validate})`; ADDED `internal/settings/registry.go` + test (`Register`, `Names`, `GetJSON`, `SetJSON`, `ErrUnknownKey`; P0-1's five keys registered in `init()`) |
| 5 | `store.OpenTemp(t)` → ADDED `internal/store/storetest.Open(t)` over `store.Open(ctx, path, store.Options{…, Migrations: store.EmbeddedMigrations()})`; `db.Writer()/Reader()` → `db.W/db.R` (also Tasks 13, 18) |
| 6 | `bus.New()` → `bus.New(nil)`; `settings.Open(db, b)` → `settings.Load(ctx, db.SettingsRepo(), b)`; `settings.Set(s,k,v)` → `settings.Set(ctx,s,k,v)`; `Clock.At` channel → `clock.Timer`; `TokenSink.SetAPIKey` → ADDED `kite/client_key.go` `(*Client).SetAPIKey` (rebuilds both inner kiteconnect clients like P0-2's `New`); `Client.OnSessionError(fn)` → `kite.Options.OnSessionError` + ADDED `kite.SessionErrorRelay{Set, Handle}`; `kite.NewHTTPClient(clk)` → ADDED `kite/netguard.go` (`AllowedHosts`, `ErrHostNotAllowed`, `NewAllowListTransport`); `FeedControl` over `ticker.Manager.Start(ctx, key, token)` → ADDED package `internal/kite/feed` (`New`, `Start`, `Stop`, `Reconnect`, `Subscribe/Unsubscribe/Release` with owner, remembered and replayed on `Start`, `ErrNotRunning`, `Usage`, `Ticks`) |
| 7, 8, 10 | `<-clk.At(t)` → `tm := clk.At(t); <-tm.C()` with `Stop()`; `settings.Set(ctx, …)`; `HandleTokenError` is the relay target |
| 12 | ADDED `kite/portfolio_types.go` (`Position`, `Holding` aliases); (pass B) `orders.PositionRow` gains `Multiplier int64 "multiplier"` |
| 14 | `CandleReader{Range(...) ([]Candle); Missing(...) ([]Span)}` → `{Read(...) ([]candles.Candle); Missing(...) ([]candles.Range)}`; `InstrumentFinder{Search(ctx,q,limit) ([]Instrument, error); ByToken}` → `{Search(q, limit) []instruments.Result; ByToken; ByID}`; `LiveSubscriber{Subscribe(c, tokens, mode); Unsubscribe}` → `{Subscribe(c, owner, tokens, mode); Unsubscribe(c, owner, tokens); Release(c, owner)}`; `bus.SubOpts{}`/`sub.Close()` → `bus.SubOpts[T]{}`/`bus.Unsubscribe` |
| 15 | `settings.NewKey` → `settings.Register(...)`; four `settings.Set(ctx, …)` |
| 16 | `candles.Candle{Time}`/`Span`/`Store.Range` → `{Start}`/`Range`/`Store.Read`; `ticker.Tick = models.Tick` → `ticker.Tick{Token, LTP, Close, Change, Volume, ExchangeTS, Received}`; `builder.CandleEvent` → `builder.Event` (+`Partial`); `InstrumentDTO` → real P0-2 fields (`Type`, `Tag`, `LotSize int64`, `IsFuture`, `Underlying`, `TickSizePaise`); `backfill.Job` gains `OI`; `TickDTO` gains `Change`, `Polled` |
| 17 | `(*Settings).Names/GetJSON/SetJSON` → package-level `settings.Names()/GetJSON(s, name)/SetJSON(ctx, s, name, raw)` |
| 19 | `ticker.TopicOrderUpdate`/`TopicTickBatch` → `kite.TopicOrderUpdate`/`ticker.TopicTicks`; `Tick{InstrumentToken, LastPrice}` → `Tick{Token, LTP}`; `LiveTokens` with owner (`PortfolioOwner`) + `Release`; `clock.Timer` loop; tests on `storetest`, `bus.New(nil)` |
| 21 | `settings.TopicChanged/Change` → `settings.Topic/Changed`; `ticker.TopicFeedState/TopicTickBatch` → `ticker.TopicFeed/TopicTicks`; `builder.CandleEvent` → `builder.Event`; generic `SubOpts`; `bus.Unsubscribe`; `clock.Timer` coalescing window |
| 22 | `app.New(Deps)` gains `Boot *app.Boot`; `Options(a, assets, uniqueID)` → `Options(a, assets, uniqueID, smoke bool)` (`OnDomReady = runtime.Quit`); window `1440×900`, min `1280×720`; Files block states P0-1's `app.go`/`app_test.go` (`New()`, `Hello`) are replaced, `app.Version` kept; `RequestRestart/RestartRequested` |
| 23 | DROPPED `AcquireInstanceLock`, `InstanceLock`, `ErrAlreadyRunning`, `PreviousExitClean` (P0-1 `app.Start` → `store.AcquireLock`/`store.ErrLocked` is authoritative); kept `UniqueID`, `ForwardToRunningInstance`, `SecondInstance`, `CaptureStdStreams`; `logging.Setup`/`resolveDataDir`/`buildPlatform` placeholders → `cmd/tradeloom/main.go` is P0-1 Task 15's `run(args, getenv, stderr) int` extended (`ErrLocked` → D-Bus forward, exit 3 = restart + `syscall.Exec` re-exec, `finalExit`, `exit_test.go`); `wire_p03.go` `buildPlatform(ctx, boot)` written with the real P0-2 constructors (`clock.New`, `ratelimit.New`, `kite.New(kite.Options{…})`, `calendar.NewDefault` + overrides, `sqlite.Open`, `instruments.NewMaster/NewRefresher`, `backfill.New`, `feed.New`, `builder.New`) and the §3.3 stop order; `.go-arch-lint.yml` rewritten with every P0-2/P0-3 component (P0-2 added none) and vendors `godbus`, `uuid`, `websocket` |
| 24 | `storetest`, `bus.New(nil)`, `settings.Load`, `settings.Set(ctx, …)`, `nopLive` with owner/`Release` |
| 26, coverage, self-review | lock references point to P0-1; realignment recorded |

### 6.2 P0-3 — pass B (bindings/events P0-4 and P0-5 need), Tasks 27–33 added + in-place edits

| Task | Change |
|---|---|
| 12 | `orders.PositionRow` + `Multiplier` (0 → 1), tests |
| 16 | `Candles` only → `CandlesRequest.TradingDays` (calendar-resolved `FromMs`), `RefreshCandles(ctx, RefreshRequest)` (HistoricalPick at Interactive, `candles.SourceManual`), `OpenLive/CloseLive` (`CandleTracker` = `*builder.Builder`, `ConsumerChart` owner = subscription id), `SearchInstruments(q, tags, limit)`, `InstrumentsByTokens`, `InstrumentByIdentifier` (`not_found`); REMOVED `Instrument(ctx, token)` and `SetLiveTokens` |
| 17 | ADDED `internal/settings/keys_ui.go`: `appearance.theme`, `appearance.follow_system`, `appearance.sidebar_collapsed`, `ui.last_route`, `chart.default_timeframe`, `calendar.stale_toast_day` |
| 21 | 13 → 17 events: `order.update` (`kite.TopicOrderUpdate`), `market.status` (`marketStatusSource` on calendar + app clock), `job.progress`, `app.navigate`; polled-tick merge into `tick.batch`; `kite_Order` alias |
| 22 | 7 → 11 bound structs (`Session, Portfolio, Settings, Chart, Instruments, Quotes, Watchlists, Data, Calendar, Diagnostics, System`); `api.Session` + `SecretStorage/UseEncryptedFile/RefuseSecretStorage/Unlock/MigrateToKeyring/ReconnectFeed`; `app.Runtime` + `Quit`, dialogs, window geometry; geometry restore/save (`window.geometry`); restore-job `stage: "restart"` → quit → exit 3; `SecondInstance --view <route>` → `app.navigate`; `len(Bind) == 11`; 22 generated files |
| 23 | `secretsStore` (`NewNone`) → `SecretSwitch`; Task 27–33 deps wired; arch-lint `licences`, `api→settings`, `service→secrets, calendar` |
| 27 (new) | Secret storage (FR-ACCT-013): `service.SecretSwitch` (implements `secrets.Store`), `SecretStorageInfo`, `SessionStatus.Secrets` + state `locked`, `SecretStorage`, `UseEncryptedFile`, `RefuseSecretStorage`, `Unlock`, `MigrateToKeyring` |
| 28 (new) | `settings/shortcuts.go` (`ShortcutAction`, `DefaultShortcuts` = P0-4's A8 ids, `ParseBinding`, `Conflicts`), `store.ShortcutsRepo`, `store.WindowStateRepo`, service `Shortcuts/Rebind/ResetShortcuts/GetUiState/SetUiState` |
| 29 (new) | `Quotes.Watch`: `WatchQuotes(ctx, consumer, tokens) (WatchResult, error)` with 30 s linger, polled fallback (5 s `LTP`+`OHLC` poll on `*ticker.ErrBudget`, `feed.ErrNotRunning`, plan `none`), `TickDTO.Polled`, last-quote cache persisted in `window_state` key `quotes.last` (FR-SHELL-004 AC5) |
| 30 (new) | Data: `DataStats`, `ClearCandles`, `RefreshInstruments`, `BackupNow`/`RestoreBackup` jobs on `job.progress`, `InspectBackup` → `BackupInfo`, daily `ScheduledBackup` after 16:00 IST + on quit (`backup.last_scheduled`) |
| 31 (new) | Calendar: `CalendarInfo`, `ImportCalendar` (all-or-nothing, `Row N: msg`), `UpsertCalendarEntry`, `RemoveCalendarEntry` ↔ `calendar_overrides` |
| 32 (new) | Diagnostics: `OpenLogsFolder`, `CreateSupportBundle` (job), `LastCrash`, `OpenCrashReport`, `DismissCrash` |
| 33 (new) | System: `AppInfo` (licences parsed from the embedded `THIRD_PARTY_NOTICES.md`), `StatusSnapshot`, `ReconnectFeed`; `make bindings` regeneration (65 functions, 17 events), `internal/app/contract_test.go`; "Produces (for P0-4 / P0-5)" rewritten as the canonical catalogue (bindings, DTOs, events, settings keys, UI-state keys, shortcut ids) |

### 6.3 P0-4 `2026-10-12-p0-4-frontend-shell/MANIFEST.md` (19 tasks)

| Task | Before → After |
|---|---|
| Consumes (P0-1) | "MinWidth 1280 — assumption" → confirmed; P0-1 scripts are `dev/build/typecheck/test` only; no `ui-e2e` job exists in P0-1 |
| Consumes (P0-3) | assumed table (`Session.State/Status→StatusSnapshot`, `SaveCredentials(key, secret)`, `LoginWithRedirect`, `AckRisk`, `Reconnect`, `Portfolio.Orders`, `Settings.All → Record`, `Data/Calendar/Diagnostics/System.Info` assumptions, A1–A8) → canonical tables (§7 C–E); `phase` → `state` (+`logging_in`), `riskAcked` → `riskAckRequired`, `secretsBackend` → `secrets.{mode,backend}`, `offsetMs` → `millis`, `nextRetryAt/lastTickAt` → RFC3339 `nextRetry/lastTick`, feed states `stopped|connecting|live|reconnecting|offline` |
| 1 | ADDED `lint` (`eslint src e2e`), `dev:mock` (`vite --mode mock`), `e2e` scripts, ESLint devDependencies, `eslint.config.js`; Playwright chromium + webkit projects |
| 2 | model types → canonical DTOs (`SessionStatus`, `CredentialsInfo`, `SecretStorageInfo`, `BookSnapshot/OrderRow/SectionCounts`, `PositionsSnapshot/PositionRow`, `HoldingsSnapshot/HoldingRow/HoldingsSummary`, `TickDTO`, `FeedState`, `ClockOffset`, `MarketStatus`, `StatusSnapshot`, `SettingValue`, `ShortcutAction`, `DataStats`, `BackupInfo`, `JobProgress`, `CalendarInfo/…`, `CrashInfo`, `AppInfo/Licence`, `Navigate`); `Backend` methods renamed to the bindings; `EventMap` = 12 canonical events; `wails.ts` maps to `wailsjs/go/api/<Struct>` + `models.ts` namespaces; mock/fixtures/seeds in the new shapes; ADDED `personalSeed` (`?seed=personal`); mock `completeLogin()` helper → `simulateBrowserLogin()` |
| 6 | `ui.lastRoute` → `ui.last_route`; `appearance.sidebarCollapsed` → `appearance.sidebar_collapsed` |
| 7 | five canonical feed states; logged-out / no-data-plan from `session.state`; clock `millis`/`warn`; `market.status` payload; `session.reconnectFeed()` |
| 8 | `profile.userName` → `name`/`initials` |
| 9 | gate on `state` + `riskAckRequired`; setup from `secretStorage()` choices; `login()` returns `LoginStart` (`listenerError` + paste box when `listenerOk` false), Cancel button |
| 10–12 | client-side arithmetic over raw Kite rows → Go snapshots in paise (`rupees()`), live LTP overlay from `useTicksStore`; `*.changed` snapshot events update the store; `order.update` only toasts and bumps versions; Refresh → `portfolio.refresh(scope)`; `PositionRow.multiplier` used in the P&L overlay (post-pass fix) |
| 13 | canonical `Profile` fields; logout handles `remoteInvalidated: false` |
| 14 | Kite connection via `credentials()`, `secretStorage()`, `setRedirectPort` (`NumberCommitField`); keys `appearance.follow_system`, `chart.default_timeframe`, `kite.redirect_port`, `log.level` (`info|debug`) |
| 15 | shortcuts via `settings.shortcuts()/rebind()/resetShortcuts()`; `CrashInfo{path, atMs}` |
| 16 | `DataStats`, `BackupInfo`, `backup.include_candles` setting; restore ends on `stage: "restart"` |
| 17 | `CalendarInfo` entries with `phases[]`, `remove(exchange, date)`, `coversUntil` string |
| 19 | ADDED the `ui-e2e` CI job (Playwright chromium + WebKit against `VITE_BACKEND=mock`, report upload) and `npm run lint` in the `frontend` job; personal-seed guard test |
| post-pass | `SessionStatus.userName` (TS-only) removed — Go sends `userId`; the name comes from `Session.Profile` |

### 6.4 P0-5 `2026-10-12-p0-5-chart-watchlists/MANIFEST.md` (19 tasks)

| Task | Before → After |
|---|---|
| Consumes | `Chart.History(HistoryRequest)`/`Refresh`/`OpenLive`/`CloseLive`, `Instruments.Search(q, segments, limit)` with alphabetical empty query, `Watchlists.{AddItem,InsertItem,RemoveItem,GetActive}`/`WatchlistDTO`, `Quotes.Watch` with persisted snapshot, `api.*` namespace, `tick.batch {prevClose, ts}`, `feed.state` with `no_data_plan|logged_out` → canonical tables (`Chart.Candles` + `tradingDays`, `Chart.Refresh`, `OpenLive/CloseLive`, `Instruments.*` (empty query → empty list), `Watchlists.{List,Create,Rename,Delete,SetActive,Add,Remove,Reorder,ReorderLists}` over `WatchlistsState`, `Quotes.Watch` (snapshot persisted by Go), `service.*` namespace, `TickDTO {close, tsMs, polled}`, feed states from P0-2); P0-4 assumptions (`onEvent`, `parseBindingError`, `loadUiState/saveUiState`, `useSetting(key)`, `useSession`, `useFeed`, `useShell`, `formatSignedPrice/formatSignedPercent/formatQuantity`, `Spinner`, `toast.info`, `appRoutes`, `data-slot`, `registerMockHandlers/emitMockEvent/__tlMock/?scenario=`) → P0-4's real exports (§7 F); "## Renamed in the consistency review" note; "## Ordering" section (§4) |
| 1 | `Instrument{segment, tickSize, expiryMs|null}` → `InstrumentDTO` fields (`tag`, `tickSizePaise`, `expiryMs` 0 = none, `isFuture`, `underlying`); `isInScope/exchangeTag` read `tag`; fixtures updated |
| 2 | module-level wrappers + `registerMockHandlers` → P0-4's `Backend` extended with `chart/instruments/quotes/watchlists` in `backend.ts`, `wails.ts`, new `api/mock/market.ts` wired into `createMockBackend` seeds; helpers go through `setBackend/currentBackend`; errors via `parseBackendError` |
| 3 | chart colours from `tokens.css` → `readChartTheme(tokens)` from `useTheme().tokens` + chart-only CSS vars |
| 5 | `History` wait-in-Go → `Candles` + backfill-wait loop on `backfill.progress` (30 s cap), `needsDataPlan` → `no_data_plan`, `reachedStart = complete && no candles` |
| 6 | second tick store `useTicks` → P0-4's `useTicksStore`; `useQuote/quoteChange/usePolled` as selectors; `MarketDataBridge` on `useStatusStore` |
| 7–9 | formatter/`IconButton`/`Skeleton` names; search dialog never sends an empty query |
| 10 | `loadUiState/saveUiState` → new `src/api/uiState.ts` over `settings.getUiState/setUiState` (key `chart.dashboard`); `useSession` → `useSessionStore`; `useSetting('chart.default_timeframe', '60minute')`; chart fills `HomeChartArea` in `slots.tsx` |
| 11 | `useShell.chromeHidden` → `/chart-full` in `rootChildren` (outside the shell) |
| 12 | `data-slot="ticker-strip"` → `ticker={<IndexTickerStrip />}` in `ShellLayout.tsx` |
| 13–17 | watchlists store over `WatchlistsState`; seeded create = `Create` + `Add(…, -1)`; `already_in_list`; rows render from item data (`missing` → unavailable); undo via `pushToast`; panel fills `HomeWatchlistArea` |
| 18–19 | `window.__tradeloomMock.emit`, `?seed=personal`, `npm run dev:mock`; `make docker-release`, `_noorders.AppImage`, eight CI jobs incl. `ui-e2e` |

### 6.5 Unresolved / noted for the implementer

1. P0-3 Task 30 asserts only what P0-1's Produces guarantee for `store.Restore` (`ReplacedDir`) and `InspectBackup` on a truncated archive; the implementer confirms the exact error values when the two packages meet. `boot.Shutdown()` after a restore relies on `database/sql`'s idempotent `Close`.
2. P0-3 Task 6 feed test asserts replayed tokens reach the fake `ticker.Conn` after `OnConnect`, relying on P0-2's documented resubscribe-on-connect behaviour (not executed in the scratch repo).
3. `.go-arch-lint.yml` edges were derived from the imports visible in the plans; P0-3 Task 23 Step 6 tells the implementer to add a missing edge if the linter names one.
4. `order.update` stays the raw gokiteconnect `Order` (snake_case JSON) by decision; P0-4 types the subset it reads (`KiteOrder`).
5. The P0-4 settings store keeps decoded JSON values (`Record<string, unknown>`) rather than raw strings; `Settings.All()` returns `SettingValue[]` and the store hydrates from it — consistent, just noted.
6. FR-SHELL-004 AC5 is now satisfied by P0-3 Task 29's persisted `quotes.last` cache (was P0-5's open point); after a restart outside market hours the strip shows the cached values until the first live/polled tick.

## 7. Canonical contract (full text the per-plan passes worked from)


Precedence: P0-1 and P0-2 Produces are AUTHORITATIVE for Go (their code was executed and passes). P0-3 is authoritative for
Wails bindings and events, but must ADD what P0-4/P0-5 legitimately need (listed in §C/§D). P0-4 Tasks 1–3 are frozen
for the shell components P0-5 consumes. Names below are final; every plan must use them verbatim.

Build tags everywhere: `-tags webkit2_41,noorders`. Go 1.25. modernc.org/sqlite v1.59.0, golang.org/x/crypto v0.55.0
(v1.60.1 / v0.58.0 declare `go 1.26.0` on proxy.golang.org, verified 2026-10-12).

### A. Go names P0-3 must use from P0-1 (replace its "Consumes" assumptions)

| P0-3 assumed | Canonical (P0-1) |
|---|---|
| `bus.New()` | `bus.New(log *slog.Logger)` (tests: `bus.New(nil)`) |
| `bus.SubOpts{Name, Buffer}` | `bus.SubOpts[T]{Name, Buffer, Critical, Coalesce}` (generic) |
| `sub.Close()` | `bus.Unsubscribe(b, sub)` |
| `(t Topic[T]) Name()` | same |
| `settings.NewKey[T](name, def, validate)` | struct literal `settings.Key[T]{Name: "…", Default: …, Validate: …}`; validators `settings.OneOf`, `settings.IntRange`, `settings.AbsPathOrEmpty` |
| `settings.Open(db, b)` | `settings.Load(ctx, db.SettingsRepo(), b)` |
| `settings.Set(s, k, v)` | `settings.Set(ctx, s, k, v)`; `settings.Get(s, k)` unchanged; `settings.Reset(ctx, s, k)` |
| `settings.Change{Name, Value}` / `settings.TopicChanged` | `settings.Changed{Key string; Value json.RawMessage}` (JSON `key`, `value`) / `settings.Topic` (name "settings.changed") |
| `(*Settings).Names/GetJSON/SetJSON`, `settings.ErrUnknownKey` | **do not exist** → P0-3 adds `internal/settings/registry.go`: `func Register[T any](k Key[T]) Key[T]` (panics on duplicate), `func Names() []string` (sorted), `func GetJSON(s *Settings, name string) (json.RawMessage, error)`, `func SetJSON(ctx context.Context, s *Settings, name string, raw json.RawMessage) error` (unmarshals into T, then `Set`), `var ErrUnknownKey`. P0-3 registers P0-1's five keys (`settings.LogLevel`, `BackupScheduleEnabled`, `BackupScheduleFolder`, `BackupScheduleKeep`, `BackupIncludeCandles`) in `registry.go`'s `init()`; its own keys are declared as `var SettingAPIKey = settings.Register(settings.Key[string]{…})`. P0-1 key names: `log.level`, `backup.schedule.enabled`, `backup.schedule.folder`, `backup.schedule.keep`, `backup.include_candles`. |
| `(*store.DB).Writer()` / `Reader()` | fields `db.W` / `db.R` (`*sql.DB`) |
| `store.OpenTemp(t)` | **does not exist** → P0-3 adds package `internal/store/storetest` with `func Open(t testing.TB) *store.DB` (migrated v1 app.db in `t.TempDir()`, `t.Cleanup(db.Close)`), built on `store.Open(ctx, filepath.Join(dir, "app.db"), store.Options{AppVersion: "test", BackupDir: filepath.Join(dir, "backups"), Now: func() time.Time{…}, Migrations: store.EmbeddedMigrations()})`. Add arch-lint component `storetest: { in: internal/store/storetest }` with `mayDependOn: [store]`. |
| `store.Open(ctx, dataDir)` | `store.Open(ctx, path string, o store.Options)`; `Options{AppVersion, BackupDir, Now, Migrations, Log}`; `(*DB).Path()`, `SchemaVersion()`, `Close()` |
| `logging.Setup(dataDir)` | **does not exist**; the process uses `app.Start(ctx, app.BootOptions{DataDir, AppVersion, PID, Now: clock.SystemNow}) (*app.Boot, error)` (P0-1 Task 15) which gives `Boot.Layout store.Layout`, `Boot.Lock *store.Lock`, `Boot.Log *logging.Logger` (embeds `*slog.Logger`), `Boot.DB`, `Boot.Bus`, `Boot.Settings`, `Boot.Secrets secrets.Detection`, `Boot.PreviousExit store.ExitState`, `Boot.CrashReports []string`, `Boot.CrashedLastTime bool`; `(*Boot).Shutdown() error`; `app.ExitMessage(dataDir, err) string`. |
| `app.AcquireInstanceLock`, `InstanceLock`, `ErrAlreadyRunning`, `PreviousExitClean` | **dropped**. `app.Start` already takes `store.AcquireLock(layout, pid)` (gofrs/flock) and returns `store.ErrLocked` when another instance runs. `cmd/tradeloom.run` checks `errors.Is(err, store.ErrLocked)` → `app.ForwardToRunningInstance(app.UniqueID(dataDir), args, wd)` (exit 0) else prints `Tradeloom is already running (data dir <dir>)` (exit 1). Keep P0-3's `UniqueID`, `ForwardToRunningInstance`, `(*App).SecondInstance`, `CaptureStdStreams`. |
| `app.New()` (P0-1 Task 1, with `Hello()`) | replaced by P0-3 `app.New(app.Deps{Boot *app.Boot; Service *service.Service; Log *slog.Logger; StartPlatform func(ctx) error; StopPlatform func(ctx)})`; P0-1's `internal/app/app.go` + `app_test.go` (`Hello`) are rewritten by P0-3 Task 22; `app.Version` stays. |
| `cmd/tradeloom/main.go` | P0-1's `run(args []string, getenv func(string) string, stderr io.Writer) int` is kept (exit 0 ok / 1 refused / 2 bad flags, `--smoke` flag, `TRADELOOM_DATA_DIR` via `store.ResolveDataDir`) and extended: exit 3 = restart requested (after a successful restore `main()` re-execs `os.Executable()` with the same args). Order: flags → `store.ResolveDataDir` → `app.Start` (ErrLocked → forward) → `app.CaptureStdStreams(boot.Log.Logger)` → `buildPlatform(boot)` (P0-2 wiring, below) → `wireP03` → `wails.Run(app.Options(a, frontend.Assets(), app.UniqueID(dataDir), smoke))`. `app.Options` gains a `smoke bool` parameter that sets `OnDomReady = runtime.Quit`. Window options: `Width: 1440, Height: 900, MinWidth: 1280, MinHeight: 720` (P0-1 is edited to the same values). |
| `secrets.Store`, `secrets.Key`, `KeyAPISecret`, `KeyAccessToken`, `ErrNotFound` | same in P0-1 ✓; plus P0-1 has `secrets.Detect(secretsFile, probe) Detection{Mode, KeyringAvailable, FileExists, ProbeError, Choices}`, `ModeKeyring/ModeFile/ModeChoose`, `ChoiceEncryptedFile/ChoiceNoStorage`, `NewKeyring()`, `NewNone()`, `CreateEncFile(path, passphrase []byte)`, `OpenEncFile(path, passphrase []byte)`, `(*EncFile).Close()/Destroy()`, `Migrate(from, to Store) error`, `ErrWrongPassphrase`, `ErrPassphraseTooShort`, `MinPassphraseRunes = 8`, `BackendKeyring/BackendFile/BackendNone`. |
| `logging.Go`, `logging.RegisterSecret` | same ✓; also `logging.CreateSupportBundle(BundleOptions{LogDir, ExportDir, Files, Now, Redactor})`, `(*store.DB).SupportFiles(ctx)`, `logging.UnseenCrashReports(dir)`, `logging.MarkCrashSeen(path)`, `logging.ParseLevel`, `Logger.Level *slog.LevelVar`. |
| archtest rule 2 | P0-1 `rules.go` matches `internal/app` by prefix (`under(self, "internal/app")`), so `internal/app/api` may import Wails but must not import engine packages (rule 7 also prefix-matches); `internal/app/api` imports only `internal/service` (aliases `service.CredentialsInfo = kite.CredentialsInfo` etc. need no kite import). `.go-arch-lint.yml`: P0-1 declares components `cmd, frontend, app, bus, clock, logging, settings, store, secrets, keyringprobe, orderwrite, archtest`; P0-2 added none, so P0-3 Task 23 adds `api: { in: internal/app/api }`, `service`, `kite`, `ratelimit: internal/kite/ratelimit`, `ticker: internal/kite/ticker`, `feed: internal/kite/feed`, `kitemock: internal/kite/mock`, `calendar`, `candles`, `candlesql: internal/candles/sqlite`, `backfill: internal/candles/backfill`, `builder: internal/candles/builder`, `instruments`, `orders`, `storetest`, `notify: internal/notify/desktop`, vendors `godbus`, with `deps` edges matching ARCHITECTURE §2.4 (app → service, api, bus, settings, store, secrets, logging; api → service; service → everything engine-side; `cmd` → frontend, app, api, service, store, kite…, clock, bus, settings, secrets, logging). |
| `store` v1 tables | P0-1 v1 has `settings, sessions, shortcuts(action_id PK, binding, scope), window_state(k PK, v), instruments, instrument_imports, instrument_fts, watchlists, watchlist_items, order_journal, orders_cache, positions_cache, order_counters, notifications, calendar_overrides` — exactly as P0-3 assumed for sessions/watchlists/orders_cache; `shortcuts` and `window_state` are used by the new P0-3 tasks. |
| P0-1 backup/restore engine | `(*DB).Backup(ctx, store.BackupOptions{Dir, IncludeCandles, CandlesDB, AppVersion, CalendarVersion, LibraryVersion, Now}) (string, error)`; `(*DB).ScheduledBackup(ctx, o, keep) (string, []string, error)`; `store.PruneBackups`; `store.InspectBackup(path) (store.Manifest, error)`; `store.Restore(ctx, store.RestoreRequest{Archive, DataDir, Confirmation, DeploymentsFlat, Now}) (store.RestoreResult, error)`; `store.RestoreConfirmPhrase = "RESTORE"`; `store.Manifest{Format, CreatedAt string, AppVersion, Schema, CalendarVersion, LibraryVersion, IncludesCandles, Counts map[string]int}`; `store.SupportedSchema()`. |

### B. Go names P0-3 must use from P0-2

| P0-3 assumed | Canonical (P0-2) |
|---|---|
| `clock.Clock.At(t) <-chan struct{}` | `At(target time.Time) clock.Timer` / `After(d) Timer`; `Timer{ C() <-chan time.Time; Stop() bool }`. Pattern: `tm := clk.At(t); defer tm.Stop(); select { case <-tm.C(): … }`. Re-arm by calling `At` again. |
| `clock.New(b)` | `clock.New(clock.Options{Bus: b, Log: log}) *clock.Real`; `(*Real).ObserveHTTPDate`, `ObserveTick`, `Acknowledge`; `clock.TopicOffset` ("clock.offset", payload `clock.Offset{Millis int64 "millis"; Source "source" (none/ticks/http-date); Samples; Warn; Skewed; UpdatedAt}`); `clock.IST`, `clock.NewFake(now)`, `(*Fake).Advance/Set/WaitTimers/PendingTimers`. |
| `kite.Position`, `kite.Holding` | missing in P0-2 → P0-3 adds `internal/kite/portfolio_types.go`: `type ( Position = kiteconnect.Position; Holding = kiteconnect.Holding )`. `Positions`, `Holdings`, `Order`, `Orders`, `QuoteLTP`, `QuoteOHLC`, `UserProfile`, `HistoricalData`, `Time` exist. |
| `kite.Kind int; KindSession = iota+1` | `kite.Kind uint8` (`KindNone, KindSession, KindPermission, KindRateLimited, KindNetwork, KindInput, KindOrder, KindServer, KindCanceled, KindUnknown`); `kite.Classify(err) Kind`; `*kite.Err{Kind, Op, Code, Type, Message}`. |
| `kite.Config{APIKey, BaseURI, HTTP, Limiter}` / `kite.New(cfg)` | `kite.New(kite.Options{APIKey, BaseURI, Limiter *ratelimit.Limiter, Clock clock.Clock, DateSink func(date, sent, received time.Time), OnSessionError func(e *kite.Err), Transport http.RoundTripper, Jitter, Log}) *kite.Client`; inner clients have 10 s (api) / 30 s (hist) timeouts. |
| `(*kite.Client).SetAPIKey` | missing → P0-3 adds `internal/kite/client_key.go`: `func (c *Client) SetAPIKey(key string)` — under `c.mu.Lock()`, sets `c.o.APIKey = key` and rebuilds `c.api`/`c.hist` exactly as `New`'s `mk` does (`kiteconnect.New(key)`, `SetHTTPClient(&http.Client{Timeout, Transport: c.rec})`, `SetBaseURI` when set). `APIKey()` and `SetAccessToken` exist. |
| `(*kite.Client).OnSessionError(fn)` | field `Options.OnSessionError func(*kite.Err)`; wired late through `kite.SessionErrorRelay` (new, P0-3): `type SessionErrorRelay struct{ mu sync.Mutex; fn func(error) }`, `func (r *SessionErrorRelay) Set(fn func(error))`, `func (r *SessionErrorRelay) Handle(e *Err)`; `kite.New(Options{OnSessionError: relay.Handle})` then `relay.Set(sess.HandleTokenError)`. |
| `kite.NewHTTPClient(clk)` | missing → P0-3 adds `internal/kite/netguard.go`: `var AllowedHosts = []string{"api.kite.trade", "kite.zerodha.com", "ws.kite.trade"}`; `var ErrHostNotAllowed`; `func NewAllowListTransport(next http.RoundTripper, allowed []string) http.RoundTripper` (refuses any other host with `ErrHostNotAllowed`; tests pass the mock server host in `allowed`). Session auth uses `kite.NewAuth(&http.Client{Timeout: 10 * time.Second, Transport: transport}, baseURI)`; the same transport is `kite.Options.Transport`. |
| `ratelimit.New(clk, b)` | `ratelimit.New(clk clock.Clock, ratelimit.Options{Rates, Margin, Bus, Log}) *Limiter`; `(*Limiter).Run(ctx)`, `PauseAll(reason)`, `Resume()`, `Snapshot() Usage`; `ratelimit.Priority uint8`, `ratelimit.Interactive`; `ratelimit.WithPriority(ctx, p)`; `ratelimit.TopicUsage` ("limits.usage", `ratelimit.Usage`). |
| `ticker.NewManager(b, clk, log)`, `Manager.Start(ctx, apiKey, token)` | `ticker.New(ticker.Options{Dial: ticker.KiteDialer(apiKey, accessToken, root *url.URL), Clock, Bus, Log, ClockRef: kite.NiftyIndexToken, Observer: realClock, …}) *Manager`; `(*Manager).Start(ctx) error`, `Stop()`, `Reconnect()`, `Subscribe(c Consumer, owner string, tokens []uint32, mode Mode) error` (`*ticker.ErrBudget` on refusal), `Unsubscribe(c, owner, tokens)`, `Release(c, owner)`, `Usage() Usage`, `Raw(buffer) (<-chan Tick, func())`. One Manager per login → P0-3 adds package `internal/kite/feed` (imports ticker; ticker imports kite, so it cannot live in `kite`): `type Feed struct{…}`; `func New(o feed.Options{Ticker ticker.Options /* Dial ignored */; Root *url.URL}) *Feed`; `(*Feed).Start(ctx, apiKey, accessToken) error` (builds a Manager with `KiteDialer`, replays remembered subscriptions); `Stop()`; `Reconnect()`; `Subscribe/Unsubscribe/Release` (remembered per (consumer, owner) and forwarded when running; `feed.ErrNotRunning` when stopped); `Usage()`. `*feed.Feed` satisfies `kite.FeedControl` and `service.LiveSubscriber`. |
| `ticker.Mode string "quote"` | `ticker.Mode uint8`: `ModeLTP=1, ModeQuote=2, ModeFull=3`. |
| `ticker.Consumer` | `ConsumerClock, ConsumerDeployment, ConsumerLiveScanner, ConsumerChart, ConsumerWatchlist, ConsumerWidget, ConsumerRecorder` (uint8). |
| `ticker.Tick = models.Tick` | `ticker.Tick{Token uint32 "token"; Mode "mode"; LTP "ltp"; Change "change"; Open; High; Low; Close "close" (prev session close); Volume uint32 "volume"; LastQty; OI; ExchangeTS time.Time "exchangeTs"; LastTradeTime; Received time.Time "received"; IsIndex}`. |
| `ticker.TopicTickBatch` / `TopicFeedState` / `ticker.TopicOrderUpdate` | `ticker.TopicTicks` ("tick.batch", `[]ticker.Tick`) / `ticker.TopicFeed` ("feed.state", `ticker.FeedState{State "state" ∈ stopped|connecting|live|reconnecting|offline; Attempt; NextRetry time.Time "nextRetry"; LastTick "lastTick"; Connections; Tokens}`) / `kite.TopicOrderUpdate` ("order.update", `kite.Order`). |
| `candles.Candle{Time,…}` / `candles.Span` / `Store.Range` | `candles.Candle{Start time.Time "t"; Open "o"; High "h"; Low "l"; Close "c"; Volume int64 "v"; OI "oi"; HasOI "hasOi"; Source "src"; Complete bool "complete"}`; `candles.Range{From, To}`; `candles.Store.Read(ctx, token uint32, iv Interval, from, to) ([]Candle, error)`, `Missing(...) ([]Range, error)`, `Upsert(ctx, token, iv, cs, src Source)`, `Stats(ctx) (Stats, error)`, `Clear(ctx, confirmRuns []string) error`; `candles.SourceManual`; `candles.ParseInterval`; `candles.Native`; `(Interval).IsNative/IsIntraday/Duration`. `candles.JobStore.OpenJobs(ctx) ([]candles.Job, error)`. Concrete store: `sqlite.Open(ctx, path, sqlite.Options{Clock, Calendar}) *sqlite.Store` (package `internal/candles/sqlite`). |
| `builder.CandleEvent` | `builder.Event{Token uint32 "token"; Interval candles.Interval "interval"; Candle candles.Candle "candle"; Partial bool "partial"}`; `builder.TopicForming` ("candle.forming"), `builder.TopicClosed` ("candle.closed"); `(*builder.Builder).Track(token, ivs ...candles.Interval)`, `Untrack(...)`; `builder.New(builder.Options{Clock, Calendar, Store, Bus, ExchangeOf, Grace, Log})`, `Run(ctx, ticks <-chan ticker.Tick)`. |
| `backfill.Job`, `Progress`, `Submit` | `backfill.Job{Token uint32; Interval candles.Interval; From, To time.Time; Requester string; Priority ratelimit.Priority; OI bool}`; `backfill.Progress{JobID "jobId"; Token "token"; Interval string "interval"; Done; Failed; Total; ETA time.Duration "etaNs"; State "state" ∈ running|done|failed}`; `backfill.TopicProgress` ("backfill.progress"); `(*Backfill).Submit(ctx, j) (string, error)`, `Wait(ctx, id) (Progress, error)`, `Ensure`, `DailyCheck(token)`, `Start(ctx)`, `WaitStopped()`; `backfill.ErrFailed`. |
| `instruments.Instrument`, `Search(ctx, q, limit) ([]Instrument, error)` | `instruments.Instrument{Token uint32 "token"; ExchangeToken; Exchange "exchange"; Segment "segment"; Tag instruments.Tag "tag" (INDICES/NSE/BSE/NFO-FUT/BFO-FUT); Tradingsymbol; Name; Type "instrumentType" (EQ/FUT); Expiry time.Time "expiry"; TickSizePaise int64; LotSize int64; DisplayName; Underlying; IsIndex; IsWeekly}`; `(Instrument).ID()` = `EXCHANGE:TRADINGSYMBOL`, `IsFuture()`; `(*Master).Search(q string, limit int) []instruments.Result{Instrument, Rank, AlsoOnNSE}` (empty query → nil); `ByToken(token) (Instrument, bool)`; `ByID(id) (Instrument, bool)`; `ImportedAt() time.Time`; `instruments.NewRefresher(instruments.Options{Client, Master, DB, Calendar, Clock, Bus, DataDir, Log})`, `(*Refresher).Load/Refresh/Run`; `instruments.TopicStatus` ("instruments.status", `instruments.Status{OK, ImportedAt, Kept, Message}`). |
| calendar | `calendar.NewDefault() (*Service, error)`; `(*Service).LabelAt(ex, t) Label{Text, Tooltip}`, `PhaseAt(ex, t) (Phase, Day)`, `Day(ex, d)`, `IsTradingDay`, `PrevTradingDay(ex, d) (Date, bool)`, `NextTradingDay`, `Session(ex, d)`, `CoversUntil() Date`, `Stale(now) bool`, `Version() string`, `Overrides() []Entry`, `MergeOverrides(es) []Entry`, `SetOverrides(es)`, `DeleteOverride(ex, d)`; `calendar.ParseFile(r io.Reader, source string) (File, error)` (`*RowError{Row, Msg}`); `calendar.ToRecord(e) Record` / `FromRecord(r) (Entry, error)` ↔ `store.CalendarOverrideRow{Exchange, Date int, Type, Phases, Source}` via `(*store.DB).CalendarOverrides/UpsertCalendarOverrides/DeleteCalendarOverride`; `calendar.Date{Year, Month, Day}`, `DateOf`, `ParseDate("2006-01-02")`, `(Date).String()`, `Int()`; `calendar.Exchange` (`NSE, BSE, NFO, BFO`), `calendar.Exchanges`; `calendar.PhaseKind` (pre-open, continuous, closing, post-closing, closed); `DayType` (trading, holiday, special, weekend, inferred-non-trading); `HM`, `ParseHM("15:30")`; `PhaseSpec{Kind, Start, End}`; `Entry{Date, Exchange, Type, Phases []PhaseSpec, Source}`. |

Platform wiring (P0-3 Task 23 `cmd/tradeloom/wire_p03.go`, `buildPlatform(boot *app.Boot) (p03Platform, error)`): `clk := clock.New(clock.Options{Bus: boot.Bus, Log})` → `limiter := ratelimit.New(clk, ratelimit.Options{Bus, Log})` + `logging.Go("ratelimit", func(){ limiter.Run(ctx) })` → `transport := kite.NewAllowListTransport(http.DefaultTransport, kite.AllowedHosts)` → `relay := &kite.SessionErrorRelay{}` → `client := kite.New(kite.Options{APIKey: settings.Get(boot.Settings, kite.SettingAPIKey), Limiter, Clock: clk, DateSink: clk.ObserveHTTPDate, OnSessionError: relay.Handle, Transport: transport, Log})` → `cal, _ := calendar.NewDefault()` + load overrides from `boot.DB.CalendarOverrides` → `cstore := sqlite.Open(ctx, boot.Layout.CandlesDB, sqlite.Options{Clock: clk, Calendar: cal})` → `master := instruments.NewMaster(clk)`; `refresher := instruments.NewRefresher(instruments.Options{Client: client, Master: master, DB: boot.DB, Calendar: cal, Clock: clk, Bus: boot.Bus, DataDir: boot.Layout.Root, Log})`; `refresher.Load(ctx)` → `bf := backfill.New(backfill.Options{Store: cstore, Jobs: cstore, Fetcher: client, Clock: clk, Bus: boot.Bus, Log})` → `fd := feed.New(feed.Options{Ticker: ticker.Options{Clock: clk, Bus: boot.Bus, Log, ClockRef: kite.NiftyIndexToken, Observer: clk}})` → `bld := builder.New(builder.Options{Clock: clk, Calendar: cal, Store: cstore, Bus: boot.Bus, ExchangeOf: master.ExchangeOf, Log})` (its `Run` is fed by `fd` raw ticks when the feed starts) → secrets store from `boot.Secrets` (keyring → `secrets.NewKeyring()`; file → locked until `Unlock`; choose → `secrets.NewNone()` until setup). StartPlatform: `limiter.Run`, `bf.Start`, `refresher.Run`. StopPlatform (§3.3 order): `fd.Stop()` → cancel ctx → `bf.WaitStopped()` → `cstore.Close()`; then `boot.Shutdown()`.

### C. Wails bindings (canonical; `internal/app/api/*.go`; generated to `frontend/wailsjs/go/api/<Struct>.{js,d.ts}`; models in `frontend/wailsjs/go/models.ts` under the Go package namespaces `service`, `kite`, `orders`, `ticker`, `clock`, `settings`, `backfill`, `calendar`)

Every DTO has explicit camelCase `json` tags; times are Unix ms `int64` fields named `…Ms` except P0-2 payloads that already carry `time.Time` (serialised RFC3339 strings: `ticker.FeedState.nextRetry/lastTick`, `clock.Offset.updatedAt`, `instruments` `expiry`). Errors: a rejected promise carries the JSON string `{"code","stage","reason"}` (`service.ErrRefused` through `app.ErrorFormatter`); plain errors are wrapped as code `internal`.

| Struct | Method (Go → TS) | Notes |
|---|---|---|
| `Session` | `Status() service.SessionStatus` | session only (not feed/market/clock — see `System.Status`) |
| | `Credentials() kite.CredentialsInfo{Configured, APIKeyMasked, Port, RedirectURL, SecretBackend}` | |
| | `SaveCredentials(in service.CredentialsInput{APIKey, APISecret}) (SessionStatus, error)` | |
| | `RemoveCredentials() (SessionStatus, error)`; `SetRedirectPort(port int) (CredentialsInfo, error)` | |
| | `Login() (kite.LoginStart{LoginURL, RedirectURL, ListenerOK, ListenerError}, error)` | opens the system browser; result arrives via `session.state` |
| | `CompleteLogin(pasted string) (SessionStatus, error)`; `CancelLogin() SessionStatus` | paste fallback |
| | `Logout() (kite.LogoutResult{RemoteInvalidated, Message}, error)` | |
| | `AckRiskDisclosure() (SessionStatus, error)`; `Profile() (kite.Profile, error)` | |
| | NEW `SecretStorage() service.SecretStorageInfo{Mode "mode" (keyring/file/choose/none), Backend "backend" (keyring/file/none/locked), KeyringAvailable, FileExists, Choices []string}` | FR-ACCT-013 setup; P0-1 `secrets.Detect` |
| | NEW `UseEncryptedFile(passphrase string) (SessionStatus, error)` (`passphrase_too_short`) ; `RefuseSecretStorage() (SessionStatus, error)`; `Unlock(passphrase string) (SessionStatus, error)` (`bad_passphrase`); `MigrateToKeyring() (SecretStorageInfo, error)` (`keyring_unavailable`) | |
| | NEW `ReconnectFeed() error` | `feed.Reconnect()` |
| `Portfolio` | `OrderBook() orders.BookSnapshot`; `Positions() orders.PositionsSnapshot`; `Holdings() orders.HoldingsSnapshot`; `Refresh(scope string) error` | snapshots as P0-3 Task 12 (paise ints, `AsOfMs`, `Stale`) |
| `Settings` | `All() ([]service.SettingValue{Name, ValueJSON}, error)`; `Set(name, valueJSON string) (SettingValue, error)` | `kite.*` keys refused with `use_credentials_flow` |
| | NEW `Shortcuts() ([]settings.ShortcutAction{ID "id", Label, Scope ("global"/"view"/"chart"/"builder"), Default "defaultBinding", Binding "binding"}, error)`; `Rebind(actionID, binding string) (ShortcutAction, error)` (conflict → code `conflict`, reason = conflicting action's label; bad binding → `invalid`); `ResetShortcuts() ([]ShortcutAction, error)` | `internal/settings/shortcuts.go` registry + `store.ShortcutsRepo` over `shortcuts` |
| | NEW `GetUiState(key string) (string, error)` ("" when absent); `SetUiState(key, value string) error` | `store.WindowStateRepo` over `window_state` |
| `Chart` | `Candles(req service.CandlesRequest{Token uint32; Interval string; FromMs, ToMs int64; TradingDays int}) (service.CandlesResult{Candles []CandleDTO; Complete bool; BackfillJobID string; NeedsDataPlan, Offline bool}, error)` | `TradingDays > 0` → Go resolves `FromMs` = start of the N-th trading day before `ToMs` (0 = now) via `calendar.PrevTradingDay`; missing ranges → `Interactive` backfill job, id returned; UI waits on `backfill.progress` for that id and calls again |
| | NEW `Refresh(req service.RefreshRequest{Token uint32; Interval string; FromMs, ToMs int64}) (CandlesResult, error)` | re-fetch via `kite.HistoricalPicker.HistoricalPick` at `Interactive`, upsert with `candles.SourceManual` (FR-CHART-006); `no_data_plan` / `offline` refusals |
| | NEW `OpenLive(token uint32, interval string) (string, error)`; `CloseLive(id string) error` | registers ticker consumer `ConsumerChart` (owner = id, quote mode) and `builder.Track(token, iv)`; returns a subscription id; CloseLive untracks when no other live subscription uses (token, interval). Replaces `SetLiveTokens` for charts. |
| `Instruments` | `Search(q string, tags []string, limit int) []service.InstrumentDTO` (`tags` empty = all; values = P0-2 `instruments.Tag`); `ByTokens(tokens []uint32) []InstrumentDTO` (unknown omitted); `ByIdentifier(id string) (InstrumentDTO, error)` (`not_found`) | empty query → empty list |
| `Quotes` | `Watch(consumer string, tokens []uint32) (service.WatchResult{Live []uint32; Polled []uint32; Snapshot []TickDTO}, error)` | consumer ∈ `chart`/`watchlist`/`widget`; replaces that consumer's whole set (30 s linger on removal, FR-WATCH-006); tokens the feed cannot carry (`*ticker.ErrBudget`, `feed.ErrNotRunning`, DataPlan `none`) are polled by Go every 5 s via `kite.ReadClient.LTP` + `OHLC` (prev close) and published on `tick.batch` with `polled: true`; `Snapshot` = last tick per token from the in-memory last-quote cache (not persisted). `invalid_consumer` otherwise. |
| `Watchlists` | `List() (service.WatchlistsState{Lists []Watchlist{ID, Name, Items []WatchlistItem{Token, Exchange, Symbol, DisplayName, Name, IsIndex, Missing}}; ActiveID}, error)`; `Create(name) (WatchlistsState, error)`; `Rename(id, name)`; `Delete(id)`; `SetActive(id)`; `Add(id string, token uint32, position int) (WatchlistsState, error)` (position < 0 = append; duplicate → `already_in_list`); `Remove(id, token) (service.RemovedItem{ListID, Token, Position}, error)`; `Reorder(id, tokens []uint32)`; `ReorderLists(ids []string)` | name errors: `name_invalid`, `name_exists` |
| `Data` | NEW `Stats() (service.DataStats{DataDir; AppDBBytes; CandleBytes; CandleBudgetBytes; InstrumentsImportedAtMs; BackfillOpenJobs int}, error)`; `ClearCandles() error` (`candles.Store.Clear`; `runs_referenced` when refused); `RefreshInstruments() error`; `ChooseFolder() (string, error)` / `ChooseBackupFile() (string, error)` ("" when cancelled; `runtime.OpenDirectoryDialog` / `OpenFileDialog` in `internal/app`); `BackupNow(folder string, includeCandles bool) (string, error)` (job id); `InspectBackup(path string) (service.BackupInfo{Path; OK; Reason; Schema; SupportedSchema; AppVersion; CreatedAt; Counts map[string]int; IncludesCandles; SizeBytes}, error)`; `Restore(path, confirmation string) (string, error)` (job id; `not_confirmed`, `bad_archive`, `schema_too_new`, `deployments_active`) | jobs report on `job.progress`; a finished restore sets `stage: "restart"` and the app quits with exit code 3 → `main` re-execs |
| `Calendar` | NEW `Info() (service.CalendarInfo{Version; CoversUntil string (YYYY-MM-DD); Stale bool; Entries []CalendarEntryDTO{Exchange; Date (YYYY-MM-DD); Type; Phases []PhaseDTO{Kind; Start; End}; Source}}, error)`; `ChooseImportFile() (string, error)`; `Import(path string) (CalendarInfo, error)` (all-or-nothing; refusal reason = `Row N: msg`); `Upsert(e service.CalendarEntryInput{Exchange; Date; Type; Phases []PhaseDTO}) (CalendarInfo, error)`; `Remove(exchange, date string) (CalendarInfo, error)` | overrides persisted in `calendar_overrides` with source `user`/`import` |
| `Diagnostics` | NEW `OpenLogsFolder() error` (`runtime.BrowserOpenURL("file://…")`); `CreateSupportBundle() (string, error)` (path); `LastCrash() (service.CrashInfo{Path; AtMs int64}, error)` (empty Path = none); `OpenCrashReport() error`; `DismissCrash() error` (`logging.MarkCrashSeen`) | |
| `System` | NEW `Info() service.AppInfo{Version; Build (go version + tags); SchemaVersion int; DataDir; Licences []Licence{Name; Version; Licence}}`; `Status() service.StatusSnapshot{Feed ticker.FeedState; Market service.MarketStatus; Clock clock.Offset}` | licences from the embedded `THIRD_PARTY_NOTICES.md` |

Removed from P0-3's original list: `Chart.Search`, `Chart.Instrument`, `Chart.SetLiveTokens` (replaced by `Instruments.*`, `Quotes.Watch`, `Chart.OpenLive/CloseLive`).

`service.SessionStatus` (JSON): `state` (`no_credentials|locked|logged_out|logging_in|ready|expired`), `sessionId`, `userId`, `userName`, `loggedInAtMs`, `expiresAtMs`, `dataPlan` (`unknown|connect|none`), `offline`, `riskAckRequired`, `errorCode`, `errorMessage`, `credentials` (`kite.CredentialsInfo`: `configured`, `apiKeyMasked`, `port`, `redirectUrl`, `secretBackend`), `secrets` (`service.SecretStorageInfo`). `locked` = encrypted-file backend not yet unlocked this start.

`service.InstrumentDTO` (JSON): `token`, `exchange`, `segment`, `tag`, `tradingsymbol`, `name`, `displayName`, `instrumentType`, `expiryMs` (0 = none), `tickSizePaise`, `lotSize`, `isIndex`, `isFuture`, `underlying`.
`service.TickDTO` (JSON): `token`, `ltp`, `close` (previous close), `change`, `changePct`, `volume`, `tsMs`, `polled`.
`service.CandleDTO` (JSON): `t` (ms, candle start), `o`, `h`, `l`, `c`, `v`, `oi`, `complete`.
`service.CandleEventDTO`: `token`, `interval`, `candle`, `partial`.

### D. Events (canonical; `frontend/src/api/events.gen.ts` exports `EventNames` and `EventPayloads`)

| Event | Payload | Emission |
|---|---|---|
| `session.state` | `service.SessionStatus` | immediate |
| `feed.state` | `ticker.FeedState` | latest / 100 ms |
| `tick.batch` | `service.TickDTO[]` | merged by token / 100 ms |
| `candle.forming` | `service.CandleEventDTO[]` | latest per (token, interval) / 100 ms |
| `candle.closed` | `service.CandleEventDTO` | immediate |
| `clock.offset` | `clock.Offset` | latest / 100 ms |
| `backfill.progress` | `backfill.Progress[]` | latest per job / 100 ms |
| `limits.usage` | `ratelimit.Usage` | latest / 100 ms |
| `orders.changed` / `positions.changed` / `holdings.changed` | `orders.BookSnapshot` / `PositionsSnapshot` / `HoldingsSnapshot` | latest / 100 ms |
| `order.update` | `kite.Order` (NEW, from `kite.TopicOrderUpdate`) | immediate |
| `watchlists.changed` | `service.WatchlistsState` | immediate |
| `settings.changed` | `settings.Changed` (`key`, `value`) | immediate |
| `market.status` | `service.MarketStatus{Exchange "exchange"; Phase "phase" (pre-open/continuous/closing/post-closing/closed); DayType "dayType"; Label "label"; Tooltip "tooltip"; AtMs}` (NEW; `service.TopicMarketStatus`; NSE at app-clock now, re-evaluated at each phase boundary, published on change and at startup) | immediate |
| `job.progress` | `service.JobProgress{JobID "jobId"; Kind "kind" (backup/restore/support_bundle); Stage "stage"; Done; Total; Finished bool; Error; ResultPath}` (NEW; `service.TopicJobProgress`) | immediate |
| `app.navigate` | `service.Navigate{Route "route"}` (NEW; `service.TopicNavigate`; published by `App.SecondInstance` when the second instance was launched with `--view <route>`) | immediate |

Feed state names: `stopped|connecting|live|reconnecting|offline` (P0-2). "Logged out" and "no data plan" are derived by the UI from `session.state` (`state`, `dataPlan`), never from `feed.state`.

### E. Settings keys (Go-declared, snake_case segments like P0-1)

P0-1: `log.level` (info|debug), `backup.schedule.enabled` (bool), `backup.schedule.folder`, `backup.schedule.keep` (1..365), `backup.include_candles` (bool).
P0-3: `kite.api_key` (hidden from `Settings.All`), `kite.redirect_port` (1024..65535, 7420), `watchlist.active`, NEW `appearance.theme` ("light"|"dark", "light"), `appearance.follow_system` (bool, false), `appearance.sidebar_collapsed` (bool, false), `ui.last_route` (string, "/home"), `chart.default_timeframe` (one of `minute,3minute,5minute,10minute,15minute,30minute,60minute,day`; default "60minute"), `calendar.stale_toast_day` (string, "").
UI state (not settings): `Settings.GetUiState/SetUiState` keys `chart.dashboard` (JSON `{identifier, timeframe}`), `window.geometry` (written by Go `BeforeClose`).

P0-4 renames: `appearance.followSystem`→`appearance.follow_system`, `appearance.sidebarCollapsed`→`appearance.sidebar_collapsed`, `ui.lastRoute`→`ui.last_route`, `chart.defaultTimeframe`→`chart.default_timeframe`, `kite.loopbackPort`→`kite.redirect_port`, `diagnostics.logLevel`→`log.level` (values `info|debug`, lower-case), `data.backup.scheduled`→`backup.schedule.enabled`, `data.backup.folder`→`backup.schedule.folder`, `data.backup.keep`→`backup.schedule.keep`, `calendar.staleToastDay`→`calendar.stale_toast_day`.

Shortcut registry (P0-3, `internal/settings/shortcuts.go`): ids/defaults exactly P0-4's A8 list: `nav.1`…`nav.8` (`Alt+1`…`Alt+8`, global), `toasts.focus` (`Alt+T`, global), `search.open` (`Ctrl+K`, global), `help.open` (`F1`, global), `kill.switch` (`Ctrl+Shift+K`, global), `chart.fullView` (`F11`, global), `chart.trendLine` (`Alt+T`, chart), `order.new` (`Shift+T`, chart), `builder.undo` (`Ctrl+Z`, builder), `builder.redo` (`Ctrl+Shift+Z`, builder), `watchlist.moveUp`/`watchlist.moveDown` (`Alt+ArrowUp`/`Alt+ArrowDown`, view). Binding grammar: `+`-joined `Ctrl`, `Alt`, `Shift`, `Meta` + key. Conflict = same binding in the same scope or in `global`.

### F. P0-4 → P0-5 frozen names (P0-4 Tasks 1–6, 12, 19 win; P0-5 adapts)

| P0-5 assumed | Canonical (P0-4) |
|---|---|
| `api/events.ts onEvent(name, cb)` | `Backend.on(name, cb): () => void` (P0-4 Task 2 `src/api/backend.ts`); P0-5 code receives the backend via `useBackend()` or a passed `Backend` |
| `api/errors.ts parseBindingError` | `parseBackendError(e): BackendError{code, stage, reason}` in `src/api/errors.ts` |
| `api/uiState.ts loadUiState/saveUiState` | P0-5 creates `src/api/uiState.ts` over `backend.settings.getUiState/setUiState` |
| `store/settingsStore.ts useSetting<T>(key)` | `useSetting<T>(key, fallback)` + `writeSetting(backend, key, value)` (`src/store/settings.ts`) |
| `store/sessionStore.ts useSession(sel)` | `useSessionStore` (`session: SessionStatus \| null`, `profile`) |
| `store/feedStore.ts useFeed` | `useStatusStore` (`feed`, `market`, `clock`, `setFeed`, …; zustand, `.subscribe` works) |
| `formatSignedPrice` / `formatSignedPercent` / `formatQuantity` | `formatSigned(v, digits)` / `formatPercent(v)` (already signed, `+0.44%`) / `formatCompact(v)` (`2.54L`, `1.32Cr`) in `src/i18n/format.ts` |
| `IconButton({label, onClick, icon, pressed?})` | `IconButton({label, icon, tooltip?, size?, ...button props})` (use `aria-pressed`) |
| `Spinner({label})` | `Skeleton({rows?, label})` |
| `toast.info(message, {action: {label, onClick}})` | `pushToast({level: 'info', message, action?: {label, onSelect}})` from `src/store/toasts.ts` |
| `useShortcut(id, handler, opts)` | `useShortcut(actionId, handler: () => void)` from `src/shortcuts/` |
| `useShell({chromeHidden})` | dropped; `/chart-full` is added to `rootChildren` in `src/app/routes.tsx` (outside the shell) so no chrome is rendered |
| `useTheme(): 'light'\|'dark'` | `useTheme(): { theme: ThemeName; tokens: ColourTokens }` (`src/app/theme.tsx`); chart adapter calls `setTheme(tokens)` |
| `appRoutes` | `shellChildren`, `rootChildren`, `buildRoutes()` in `src/app/routes.tsx` |
| `TopBar` `data-slot="ticker-strip"` | `ShellLayout.tsx` passes `ticker={<IndexTickerStrip />}` to `TopBar` (P0-5 edits only that prop) |
| `HomeView <HomeLayout chart= watchlist= />` | `HomeLayout({topRow?, main, below?, right})`; P0-5 replaces the bodies of `HomeChartArea` and `HomeWatchlistArea` in `src/views/home/slots.tsx` (P0-4 Task 12) |
| `design/tokens.css --up/--down/--flat` | CSS vars `--up`, `--down`, `--flat` etc. are set on `:root` by `applyTheme` from `src/design/tokens.ts` (`cssVarName`) — same names ✓ |
| `registerMockHandlers`, `emitMockEvent`, `window.__tlMock`, `?scenario=personal`, `npm run dev:mock` | P0-5 extends `Backend` with domains `chart`, `instruments`, `quotes`, `watchlists` in `src/api/backend.ts`, implements them in `src/api/wails.ts` and `src/api/mock/index.ts` (`MockState` gains instruments/candles/watchlists/quotes), mock events via `window.__tradeloomMock.emit(name, payload)`, seeds via `?seed=<name>` (`seedByName`; P0-4 adds `personalSeed` = ready session with `dataPlan: 'none'`), dev server `VITE_BACKEND=mock npm run dev` (P0-4 `.env.mock`; P0-4 Task 2 adds script `"dev:mock": "vite --mode mock"`) |
| `Backend.on` tick store | ONE tick store: P0-4 `useTicksStore` (`latest`, `applyBatch`) + `useLtp(token)`; P0-5 builds `useQuote`/`quoteChange` over it and does not create a second store |
| CI job `ui-e2e` | P0-1 has no such job → P0-4 Task 19 adds job `ui-e2e` to `.github/workflows/ci.yml` (Playwright chromium + webkit against `VITE_BACKEND=mock`); P0-1 `frontend` job runs `npm run typecheck && npm test && npm run build` (no `lint` script exists → P0-4 Task 1 adds `"lint": "eslint src e2e"`) |
| `make package`, AppImage name | P0-1 targets `deb`, `appimage`, `docker-release`; artefacts `dist/tradeloom_<v>_amd64_noorders.deb`, `dist/Tradeloom-<v>-x86_64_noorders.AppImage`; CI jobs `go-unit, ta, arch-lint, frontend, licence, linux-package, install-smoke` (+ `ui-e2e` from P0-4) |

### G. Cross-plan ordering

1. P0-1 → P0-2 → P0-3 strictly.
2. P0-4 Task 1 before any P0-5 task (theme tokens, `formatPrice`).
3. P0-4 Task 2 before P0-5 Tasks 2, 6, 7, 13 (Backend/mock/stores).
4. P0-4 Tasks 3, 4, 6 before P0-5 Tasks 8–17 (primitives, toasts, shortcuts).
5. P0-4 Task 12 before P0-5 Tasks 10, 16 (`slots.tsx`).
6. P0-4 Task 19 before P0-5 Tasks 18, 19 (`e2e/helpers.ts`, `ui-e2e` job).
P0-5 Tasks 1, 3, 4, 5 may run in parallel with P0-4 Tasks 2–19.

## 8. Interface ledger (generated by `check_p0_interfaces.py --ledger`)

Kinds: `binding` = Wails bound-struct method (`Struct.Method`), `event` = bus topic / Wails event, `key` = Go-declared
settings key or secret key, `go/ts` = Go identifier or TS export. Producer = plan and task whose Produces block or code
defines it (`hdr` = the plan's top-level Produces section); consumers = plan:task whose Consumes block references it.
1089 rows listed; 0 produced-but-never-consumed `go/ts` names are omitted (they are internal to their plan).

| Name | Kind | Producer (plan:task) | Consumers |
|---|---|---|---|
| `Calendar.ChooseImportFile` | binding | P0-3 hdr | — |
| `Calendar.Import` | binding | P0-3 hdr | — |
| `Calendar.Info` | binding | P0-3 hdr | — |
| `Calendar.Remove` | binding | P0-3 hdr | — |
| `Calendar.Upsert` | binding | P0-3 hdr | — |
| `Chart.Candles` | binding | P0-3 T22, P0-3 hdr, P0-5 T5 | P0-5 hdr |
| `Chart.CloseLive` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Chart.OpenLive` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Chart.Refresh` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Data.BackupNow` | binding | P0-3 hdr | — |
| `Data.ChooseBackupFile` | binding | P0-3 hdr | — |
| `Data.ChooseFolder` | binding | P0-3 hdr | — |
| `Data.ClearCandles` | binding | P0-3 hdr | — |
| `Data.InspectBackup` | binding | P0-3 hdr | — |
| `Data.RefreshInstruments` | binding | P0-3 hdr | — |
| `Data.Restore` | binding | P0-3 hdr | — |
| `Data.Stats` | binding | P0-3 hdr | — |
| `DB.SettingsRepo` | binding | P0-1 T10 | P0-1 T11 |
| `Diagnostics.CreateSupportBundle` | binding | P0-3 hdr | — |
| `Diagnostics.DismissCrash` | binding | P0-3 hdr | — |
| `Diagnostics.LastCrash` | binding | P0-3 hdr | — |
| `Diagnostics.OpenCrashReport` | binding | P0-3 hdr | — |
| `Diagnostics.OpenLogsFolder` | binding | P0-3 hdr | — |
| `Instruments.ByIdentifier` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Instruments.ByTokens` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Instruments.Search` | binding | P0-3 T22, P0-3 hdr, P0-5 T9 | P0-5 hdr |
| `Portfolio.Holdings` | binding | P0-3 hdr | — |
| `Portfolio.OrderBook` | binding | P0-3 hdr | — |
| `Portfolio.Positions` | binding | P0-3 hdr | — |
| `Portfolio.Refresh` | binding | P0-3 hdr | — |
| `Quotes.Watch` | binding | P0-3 T16, P0-3 T21, P0-3 T22, P0-3 hdr, P0-5 hdr | P0-5 hdr |
| `Session.AckRiskDisclosure` | binding | P0-3 hdr | — |
| `Session.CancelLogin` | binding | P0-3 hdr | — |
| `Session.CompleteLogin` | binding | P0-3 hdr | — |
| `Session.Credentials` | binding | P0-3 hdr | — |
| `Session.Login` | binding | P0-3 hdr | — |
| `Session.Logout` | binding | P0-3 hdr | — |
| `Session.MigrateToKeyring` | binding | P0-3 hdr | — |
| `Session.Profile` | binding | P0-3 hdr | — |
| `Session.ReconnectFeed` | binding | P0-3 hdr | — |
| `Session.RefuseSecretStorage` | binding | P0-3 hdr | — |
| `Session.RemoveCredentials` | binding | P0-3 hdr | — |
| `Session.SaveCredentials` | binding | P0-3 hdr | — |
| `Session.SecretStorage` | binding | P0-3 hdr | — |
| `Session.SetRedirectPort` | binding | P0-3 hdr | — |
| `Session.Status` | binding | P0-3 hdr | — |
| `Session.Unlock` | binding | P0-3 hdr | — |
| `Session.UseEncryptedFile` | binding | P0-3 hdr | — |
| `Settings.All` | binding | P0-3 hdr | — |
| `Settings.GetUiState` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Settings.Rebind` | binding | P0-3 hdr | — |
| `Settings.ResetShortcuts` | binding | P0-3 hdr | — |
| `Settings.Set` | binding | P0-3 hdr | — |
| `Settings.SetUiState` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Settings.Shortcuts` | binding | P0-3 hdr | — |
| `System.Info` | binding | P0-3 hdr | — |
| `System.Status` | binding | P0-3 hdr | — |
| `Watchlists.Add` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Create` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Delete` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.List` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Remove` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Rename` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Reorder` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.ReorderLists` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.SetActive` | binding | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `app.navigate` | event | P0-3 T21, P0-3 hdr | P0-5 hdr |
| `backfill.progress` | event | P0-2 T14, P0-3 T21, P0-3 hdr | P0-3 hdr, P0-5 T2, P0-5 hdr |
| `candle.closed` | event | P0-2 T19, P0-3 T21, P0-3 hdr | P0-3 hdr, P0-5 T2, P0-5 hdr |
| `candle.forming` | event | P0-2 T19, P0-3 T21, P0-3 hdr | P0-3 hdr, P0-5 T2, P0-5 hdr |
| `clock.offset` | event | P0-2 T2, P0-3 T21, P0-3 hdr | P0-3 hdr |
| `feed.state` | event | P0-2 T17, P0-3 T21, P0-3 hdr | P0-3 hdr, P0-5 T6, P0-5 hdr |
| `holdings.changed` | event | P0-3 T19, P0-3 T21, P0-3 hdr | — |
| `instruments.status` | event | P0-2 T16 | — |
| `job.progress` | event | P0-3 T21, P0-3 hdr | — |
| `limits.usage` | event | P0-2 T6, P0-3 T21, P0-3 hdr | P0-3 hdr |
| `market.status` | event | P0-3 T21, P0-3 hdr | — |
| `my.event` | event | P0-1 T3 | — |
| `order.update` | event | P0-2 T7, P0-3 T21, P0-3 hdr | P0-3 T19, P0-3 hdr |
| `orders.changed` | event | P0-3 T19, P0-3 T21, P0-3 hdr | — |
| `positions.changed` | event | P0-3 T19, P0-3 T21, P0-3 hdr | — |
| `session.state` | event | P0-3 T1, P0-3 T21, P0-3 hdr | P0-5 hdr |
| `settings.changed` | event | P0-1 T10, P0-3 T21, P0-3 hdr | P0-3 hdr |
| `tick.batch` | event | P0-3 T21, P0-3 hdr | P0-3 T19, P0-3 hdr, P0-5 T2, P0-5 hdr |
| `tick.test` | event | P0-1 T3 | — |
| `watchlists.changed` | event | P0-3 T15, P0-3 T21, P0-3 hdr | — |
| `activeList` | go/ts | P0-5 T13 | P0-5 T15, P0-5 T16 |
| `Add` | go/ts | P0-3 T22, P0-3 hdr, P0-5 T13, P0-5 T2 | P0-5 hdr |
| `Advance` | go/ts | P0-2 T1 | P0-3 T8 |
| `advanceUntilEvent` | go/ts | P0-3 T21, P0-3 T29 | P0-3 T29 |
| `AllKeys` | go/ts | P0-1 T12 | P0-1 T14 |
| `already_in_list` | go/ts | P0-3 T15, P0-3 hdr | P0-5 hdr |
| `App` | go/ts | P0-1 T1, P0-1 T15, P0-3 T22, P0-3 hdr, P0-4 T1, P0-4 T2 | P0-3 T22 |
| `app.Boot` | go/ts | P0-1 T15, P0-3 T22 | P0-3 T22, P0-3 T23 |
| `app.db` | go/ts | P0-1 T8, P0-1 T9, P0-2 T16, P0-3 T23, P0-3 T26, P0-3 T30 | P0-2 hdr |
| `app.ExitMessage` | go/ts | P0-1 T15 | P0-3 T23 |
| `app.navigate` | go/ts | P0-3 T21, P0-3 hdr, P0-4 T19 | P0-5 hdr |
| `app.New` | go/ts | P0-1 T1, P0-1 T15, P0-3 T22, P0-3 hdr | P0-1 T15, P0-3 T23 |
| `App.OnStartup` | go/ts | P0-1 T1 | P0-1 T15 |
| `app.Options` | go/ts | P0-3 T22 | P0-3 T23, P0-3 T33 |
| `App.RestartRequested` | go/ts | P0-3 T22 | P0-3 T23 |
| `app.Start` | go/ts | P0-1 T15, P0-3 T23, P0-3 T26 | P0-3 T23 |
| `app.Version` | go/ts | P0-1 T1, P0-1 T15, P0-3 T22 | P0-3 T22, P0-3 T33 |
| `AppDB` | go/ts | P0-1 T6 | P0-3 T30 |
| `appearance.follow_system` | go/ts | P0-3 hdr | P0-4 T14 |
| `appearance.sidebar_collapsed` | go/ts | P0-3 hdr | P0-4 T14 |
| `appearance.theme` | go/ts | P0-3 hdr | P0-4 T14 |
| `AppearanceFollowSystem` | go/ts | P0-3 T17, P0-3 hdr | P0-3 hdr |
| `AppearanceSidebarCollapsed` | go/ts | P0-3 T17, P0-3 hdr | P0-3 hdr |
| `appimage` | go/ts | P0-1 T16 | P0-5 T19, P0-5 hdr |
| `Apply` | go/ts | P0-1 T7 | P0-2 hdr |
| `applyBatch` | go/ts | P0-4 T2 | P0-5 T6, P0-5 hdr |
| `applyTheme` | go/ts | P0-4 T1, P0-5 T3 | P0-5 T3, P0-5 hdr |
| `AppRouter` | go/ts | P0-4 T1 | P0-4 T2 |
| `archiveEntries` | go/ts | P0-1 T8 | P0-1 T9 |
| `archtest` | go/ts | P0-1 T16 | P0-2 T22 |
| `asRefused` | go/ts | P0-3 T14 | P0-3 T28 |
| `assetserver.Options` | go/ts | P0-1 T4, P0-1 T7, P0-2 T11, P0-2 T14, P0-2 T16, P0-2 T17, P0-2 T19, P0-2 T2, P0-2 T20, P0-2 T21, P0-2 T6, P0-2 T9, P0-2 hdr, P0-3 T22, P0-3 T6 | P0-3 T22 |
| `At` | go/ts | P0-1 T11, P0-1 T5, P0-2 T1, P0-2 T2, P0-2 T3, P0-2 T6 | P0-3 T6 |
| `Avatar` | go/ts | P0-4 T8 | P0-4 T13 |
| `avatarUrl` | go/ts | P0-3 T11, P0-3 hdr | P0-4 T8 |
| `Backend` | go/ts | P0-1 T10, P0-1 T12, P0-1 T13, P0-3 T24, P0-3 T27, P0-3 T6, P0-4 T2 | P0-5 T2, P0-5 hdr |
| `backend.ts` | go/ts | P0-5 T2 | P0-5 hdr |
| `BackendFile` | go/ts | P0-1 T12 | P0-1 T13, P0-3 T27 |
| `BackendKeyring` | go/ts | P0-1 T12 | P0-3 T27 |
| `BackendNone` | go/ts | P0-1 T12 | P0-3 T27 |
| `BackendProvider` | go/ts | P0-4 T2 | P0-5 T10, P0-5 T6, P0-5 hdr |
| `backfill.Job` | go/ts | P0-2 T14 | P0-3 T16 |
| `backfill.New` | go/ts | P0-2 T14, P0-2 hdr | P0-3 T23 |
| `backfill.Progress` | go/ts | P0-2 T14, P0-2 hdr, P0-3 hdr | P0-3 T21 |
| `backfill.progress` | go/ts | P0-2 T14, P0-3 T16, P0-3 hdr, P0-5 T5 | P0-5 T2, P0-5 hdr |
| `Backfill.Submit` | go/ts | P0-2 T14 | P0-3 T14 |
| `backfill.TopicProgress` | go/ts | P0-2 T14, P0-3 hdr | P0-3 T21 |
| `backfillJobId` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T5 | P0-5 hdr |
| `BackfillProgress` | go/ts | P0-5 T2 | P0-5 T5 |
| `BackfillSubmitter` | go/ts | P0-3 T14 | P0-3 T16, P0-3 hdr |
| `Backoff` | go/ts | P0-2 T6 | P0-2 T9 |
| `backtest_runs` | go/ts | P0-1 T8, P0-3 hdr | P0-4 T16 |
| `backup.include_candles` | go/ts | P0-3 hdr | P0-4 T16 |
| `backup.schedule.enabled` | go/ts | P0-3 T30, P0-3 hdr | P0-4 T16 |
| `backup.schedule.folder` | go/ts | P0-3 T30, P0-3 hdr | P0-4 T16 |
| `backup.schedule.keep` | go/ts | P0-3 T30, P0-3 hdr | P0-4 T16 |
| `BackupIncludeCandles` | go/ts | P0-1 T10, P0-3 hdr | P0-3 T2, P0-3 T30 |
| `BackupInfo` | go/ts | P0-3 T30, P0-3 hdr, P0-4 T2 | P0-4 T16 |
| `BackupOnQuit` | go/ts | P0-3 T30, P0-3 hdr | P0-3 T22 |
| `backupOpts` | go/ts | P0-1 T8 | P0-1 T9 |
| `BackupScheduleEnabled` | go/ts | P0-1 T10, P0-3 hdr | P0-3 T2, P0-3 T30 |
| `BackupScheduleFolder` | go/ts | P0-1 T10, P0-3 hdr | P0-3 T2, P0-3 T30 |
| `BackupScheduleKeep` | go/ts | P0-1 T10, P0-3 hdr | P0-3 T2, P0-3 T30 |
| `BackupsDir` | go/ts | P0-1 T6 | P0-3 T30 |
| `Bar` | go/ts | P0-5 T2 | P0-5 T3 |
| `beginJob` | go/ts | P0-3 T30 | P0-3 T32 |
| `binding` | go/ts | P0-3 T28, P0-3 hdr | P0-3 T22 |
| `BindingError` | go/ts | P0-5 T2 | P0-5 T13, P0-5 T5 |
| `bindingFromEvent` | go/ts | P0-4 T6 | P0-4 T15 |
| `bookOf` | go/ts | P0-4 T2 | P0-4 T11 |
| `BookSnapshot` | go/ts | P0-3 T12, P0-3 T20, P0-3 hdr, P0-4 T2 | P0-4 T10 |
| `Boot.CrashedLastTime` | go/ts | P0-1 T15 | P0-3 T23 |
| `Boot.CrashReports` | go/ts | P0-3 T32 | P0-3 T23, P0-3 T32 |
| `Boot.Layout` | go/ts | P0-1 T15, P0-1 T6, P0-3 T30 | P0-3 T23 |
| `Boot.Secrets` | go/ts | P0-3 T27 | P0-3 T23 |
| `BrowserOpenURL` | go/ts | P0-3 T22 | P0-3 T22 |
| `BSE` | go/ts | P0-2 T3 | P0-5 hdr |
| `build` | go/ts | P0-1 T16, P0-3 T33, P0-3 hdr | P0-1 T16, P0-3 T33 |
| `BUILD_PHASE` | go/ts | P0-4 T1 | P0-4 T14 |
| `builder.Event` | go/ts | P0-2 T19, P0-2 hdr | P0-3 T16, P0-3 T21 |
| `builder.New` | go/ts | P0-2 T19, P0-2 hdr | P0-2 T20, P0-3 T23 |
| `builder.TopicClosed` | go/ts | P0-2 T19, P0-3 hdr | P0-2 T20, P0-3 T21 |
| `builder.TopicForming` | go/ts | P0-2 T19, P0-3 hdr | P0-3 T21 |
| `builder.Track` | go/ts | P0-2 T19 | P0-5 hdr |
| `buildRoutes` | go/ts | P0-4 T1, P0-4 T9 | P0-5 hdr |
| `bus.Bus` | go/ts | P0-1 T10, P0-1 T15, P0-1 T3 | P0-1 T10, P0-2 T2 |
| `bus.New` | go/ts | P0-1 T3 | P0-1 T15, P0-2 T2, P0-3 T14, P0-3 T22, P0-3 T24, P0-3 T6 |
| `bus.NewTopic` | go/ts | P0-1 T10, P0-1 T3 | P0-1 T10, P0-2 T2, P0-2 T7, P0-3 T1 |
| `bus.Publish` | go/ts | P0-1 T3 | P0-1 T10, P0-2 T2, P0-3 T6 |
| `bus.SubOpts` | go/ts | P0-1 T3 | P0-1 T10, P0-2 T2, P0-3 T14, P0-3 T19, P0-3 T21 |
| `bus.Subscribe` | go/ts | P0-1 T3 | P0-1 T10, P0-1 T15, P0-2 T2 |
| `bus.Unsubscribe` | go/ts | P0-1 T3 | P0-3 T14, P0-3 T19, P0-3 T21 |
| `BusObject.Call` | go/ts | P0-2 T9, P0-2 hdr | P0-3 T23 |
| `Button` | go/ts | P0-4 T3 | P0-4 T12, P0-4 T13, P0-4 T18, P0-4 T5, P0-4 T7, P0-4 T9, P0-5 T10, P0-5 T16, P0-5 T17, P0-5 hdr |
| `ByID` | go/ts | P0-2 T15, P0-2 hdr, P0-3 T14 | P0-3 T14 |
| `ByIdentifier` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `ByToken` | go/ts | P0-2 T15, P0-2 hdr, P0-3 T14 | P0-3 T14 |
| `ByTokens` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `c` | go/ts | P0-2 T10, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `Calendar` | go/ts | P0-2 T11, P0-2 T16, P0-2 T19, P0-3 T16, P0-3 T22, P0-3 T23 | P0-4 T2 |
| `calendar.DailyCandleCheck` | go/ts | P0-2 T5 | P0-2 T14 |
| `calendar.Date` | go/ts | P0-2 T3, P0-2 T4, P0-2 T5 | P0-2 T10, P0-2 T14, P0-2 T15 |
| `calendar.DateOf` | go/ts | P0-2 T3 | P0-2 T10, P0-3 T16, P0-3 T21 |
| `calendar.DayTrading` | go/ts | P0-2 T3 | P0-3 T31 |
| `calendar.Entry` | go/ts | P0-2 T3 | P0-3 T31 |
| `calendar.Exchanges` | go/ts | P0-2 T3 | P0-3 T16, P0-3 T31 |
| `calendar.FromRecord` | go/ts | P0-2 T4 | P0-3 T23 |
| `Calendar.Import` | go/ts | P0-3 T22, P0-3 hdr | P0-4 T17 |
| `calendar.NewDefault` | go/ts | P0-2 T4, P0-2 hdr | P0-2 T10, P0-3 T16, P0-3 T22, P0-3 T23 |
| `calendar.NSE` | go/ts | P0-2 T3 | P0-3 T16, P0-3 T21 |
| `calendar.ParseDate` | go/ts | P0-2 T3 | P0-3 T31 |
| `calendar.ParseFile` | go/ts | P0-2 T3 | P0-3 T31 |
| `calendar.ParseHM` | go/ts | P0-2 T3 | P0-3 T31 |
| `calendar.Phase` | go/ts | P0-2 T3 | P0-2 T19 |
| `calendar.PhaseClosed` | go/ts | P0-2 T3 | P0-3 T21 |
| `calendar.PhaseContinuous` | go/ts | P0-2 T3 | P0-3 T16 |
| `calendar.PhaseSpec` | go/ts | P0-2 T3 | P0-3 T31 |
| `calendar.ToRecord` | go/ts | P0-2 T4 | P0-3 T31 |
| `calendar_overrides` | go/ts | P0-3 T31 | P0-2 T5 |
| `calendardata.Shipped` | go/ts | P0-2 T3 | P0-2 T4 |
| `CalendarEntryDTO` | go/ts | P0-3 T31, P0-3 hdr, P0-4 T2 | P0-4 T17 |
| `CalendarEntryInput` | go/ts | P0-3 T31, P0-3 hdr, P0-4 T2 | P0-4 T17 |
| `calendarInfo` | go/ts | P0-3 T31 | P0-3 T32 |
| `CalendarInfo` | go/ts | P0-3 T31, P0-3 T32, P0-3 hdr, P0-4 T2 | P0-4 T17 |
| `CalendarStaleToastDay` | go/ts | P0-3 T17, P0-3 hdr | P0-3 hdr |
| `Call.Store` | go/ts | P0-1 T12, P0-1 T13, P0-1 T14, P0-2 T10, P0-2 T11, P0-2 T13, P0-2 T14, P0-2 T19, P0-2 hdr | P0-3 T23 |
| `CallbackPath` | go/ts | P0-3 T2 | P0-3 T4 |
| `callsTo` | go/ts | P0-4 T2 | P0-5 T13, P0-5 hdr |
| `candle` | go/ts | P0-2 T19, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `candle.closed` | go/ts | P0-3 hdr | P0-5 T2, P0-5 hdr |
| `candle.forming` | go/ts | P0-3 hdr | P0-5 T2, P0-5 hdr |
| `CandleAdmin` | go/ts | P0-3 T30 | P0-3 hdr |
| `CandleDTO` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `CandleEvent` | go/ts | P0-5 T2 | P0-5 T5 |
| `CandleEventDTO` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `CandleEventFrom` | go/ts | P0-3 T16 | P0-3 T21 |
| `CandleReader` | go/ts | P0-3 T14 | P0-3 T16, P0-3 hdr |
| `candles` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-2 T11, P0-3 T23, P0-5 T4 |
| `Candles` | go/ts | P0-3 T14, P0-3 T16, P0-3 T22, P0-3 T23, P0-3 hdr, P0-5 T5 | P0-5 hdr |
| `candles.Candle` | go/ts | P0-2 T10, P0-2 hdr | P0-2 T19, P0-3 T16 |
| `candles.IntervalStats` | go/ts | P0-2 T10 | P0-3 T30 |
| `candles.Job` | go/ts | P0-2 T10 | P0-2 T12, P0-3 T30 |
| `candles.JobStore` | go/ts | P0-2 T10, P0-2 hdr | P0-2 T14 |
| `candles.ParseInterval` | go/ts | P0-2 T10 | P0-3 T16 |
| `candles.Range` | go/ts | P0-2 T10 | P0-3 T14, P0-3 T16 |
| `candles.SourceManual` | go/ts | P0-2 T10, P0-3 T16 | P0-3 T16 |
| `candles.Stats` | go/ts | P0-2 T10 | P0-3 T30 |
| `candles.Store` | go/ts | P0-2 T10, P0-2 T13, P0-2 hdr | P0-2 T14 |
| `CandlesDB` | go/ts | P0-1 T6, P0-1 T8 | P0-3 T30 |
| `CandlesPage` | go/ts | P0-5 T2 | P0-5 T5 |
| `CandlesRequest` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `CandlesResult` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `CandleTracker` | go/ts | P0-3 T16 | P0-3 hdr |
| `CandleUpserter` | go/ts | P0-3 T16 | P0-3 hdr |
| `change` | go/ts | P0-2 T17, P0-3 T16, P0-3 hdr, P0-5 T6 | P0-5 hdr |
| `changePct` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T6 | P0-5 hdr |
| `chart` | go/ts | P0-3 T29, P0-3 hdr, P0-5 T8, P0-5 hdr | P0-5 T3, P0-5 hdr |
| `Chart` | go/ts | P0-3 T22 | P0-5 T2 |
| `Chart.Candles` | go/ts | P0-3 T22, P0-3 hdr, P0-5 T5 | P0-5 hdr |
| `Chart.CloseLive` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `chart.dashboard` | go/ts | P0-3 T28, P0-3 hdr | P0-5 hdr |
| `chart.default_timeframe` | go/ts | P0-3 hdr, P0-4 T12, P0-4 T14 | P0-4 T14, P0-5 hdr |
| `chart.fullView` | go/ts | P0-3 hdr | P0-5 hdr |
| `Chart.OpenLive` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Chart.Refresh` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `ChartAdapter` | go/ts | P0-5 T3, P0-5 T4, P0-5 hdr | P0-5 T5 |
| `ChartDefaultTimeframe` | go/ts | P0-3 T17, P0-3 hdr | P0-3 hdr |
| `ChartTheme` | go/ts | P0-5 T2 | P0-5 T3 |
| `ChartView` | go/ts | P0-5 T10, P0-5 hdr | P0-5 T11 |
| `check` | go/ts | P0-2 T2, P0-4 T19 | P0-5 hdr |
| `ChoiceEncryptedFile` | go/ts | P0-1 T14 | P0-3 T27 |
| `ChoiceNoStorage` | go/ts | P0-1 T14 | P0-3 T27 |
| `ci.yml` | go/ts | P0-1 T16 | P0-4 T19, P0-5 T19 |
| `Classify` | go/ts | P0-2 T7, P0-2 hdr | P0-2 T9 |
| `Client.Instruments` | go/ts | P0-2 T9 | P0-2 T15 |
| `Client.SetAPIKey` | go/ts | P0-3 T6 | P0-3 hdr |
| `clock` | go/ts | P0-3 T21, P0-3 T33, P0-3 hdr, P0-4 T2 | P0-2 T1, P0-5 hdr |
| `Clock` | go/ts | P0-1 T2, P0-2 T1, P0-2 T11, P0-2 T14, P0-2 T16, P0-2 T17, P0-2 T19, P0-2 T9, P0-3 T14, P0-3 T19, P0-3 T23, P0-3 T33, P0-3 T6 | P0-2 T2 |
| `clock.BucketStart` | go/ts | P0-2 T1 | P0-2 T10, P0-2 T19, P0-2 T4 |
| `clock.Clock` | go/ts | P0-1 T2, P0-2 T1 | P0-2 T11, P0-2 T14, P0-2 T15, P0-2 T16, P0-2 T17, P0-2 T19, P0-2 T6, P0-2 T9, P0-3 T6 |
| `clock.Clock.At` | go/ts | P0-1 T11, P0-1 T5, P0-2 T1, P0-2 T2, P0-2 T3, P0-2 T6 | P0-3 T8 |
| `clock.Fake` | go/ts | P0-2 T1 | P0-2 T11, P0-2 T14, P0-2 T15, P0-2 T17, P0-2 T19, P0-2 T6 |
| `clock.FormatDateIST` | go/ts | P0-2 T1 | P0-2 T16 |
| `clock.IST` | go/ts | P0-2 T1 | P0-2 T10, P0-2 T3, P0-2 T4, P0-2 T8, P0-3 T1 |
| `clock.MidnightIST` | go/ts | P0-2 T1 | P0-2 T10, P0-2 T15, P0-2 T19 |
| `clock.New` | go/ts | P0-2 T2, P0-2 hdr | P0-2 T20, P0-2 T21, P0-2 T9, P0-3 T23 |
| `clock.NewFake` | go/ts | P0-2 T1 | P0-3 T6 |
| `clock.NextBoundary` | go/ts | P0-2 T1 | P0-2 T4 |
| `clock.Offset` | go/ts | P0-2 T1, P0-2 T2, P0-2 hdr, P0-3 hdr | P0-3 T21, P0-3 T22 |
| `clock.Sleep` | go/ts | P0-2 T1 | P0-2 T17, P0-2 T6, P0-2 T9 |
| `clock.SystemNow` | go/ts | P0-1 T2 | P0-1 T15, P0-1 T18, P0-2 T2, P0-3 T23 |
| `clock.Timer` | go/ts | P0-2 T1 | P0-3 T19, P0-3 T21, P0-3 T29, P0-3 T6, P0-3 T8 |
| `clock.TopicOffset` | go/ts | P0-2 T2, P0-3 hdr | P0-3 T21 |
| `ClockOffset` | go/ts | P0-4 T2 | P0-4 T7 |
| `ClockRef` | go/ts | P0-2 T17 | P0-2 T21 |
| `close` | go/ts | P0-1 T15, P0-1 T3, P0-2 T17, P0-2 T6, P0-3 T16, P0-3 hdr, P0-4 T3, P0-5 T6 | P0-5 T14, P0-5 T17, P0-5 hdr |
| `CloseLive` | go/ts | P0-3 T16, P0-3 T22, P0-3 hdr | P0-5 hdr |
| `closeLive` | go/ts | P0-5 T2, P0-5 T5, P0-5 hdr | P0-5 T10 |
| `colourTokens` | go/ts | P0-4 T1 | P0-5 T3, P0-5 hdr |
| `ColourTokens` | go/ts | P0-4 T1 | P0-5 T3, P0-5 hdr |
| `Column` | go/ts | P0-4 T5 | P0-4 T10 |
| `ConfirmDialog` | go/ts | P0-4 T3 | P0-4 T13, P0-4 T16, P0-5 T16, P0-5 hdr |
| `conflict` | go/ts | P0-3 hdr | P0-4 T15 |
| `Conflicts` | go/ts | P0-3 T28, P0-3 hdr | P0-3 hdr |
| `Conn` | go/ts | P0-2 T17, P0-3 T9 | P0-2 T18 |
| `Connects` | go/ts | P0-2 T8 | P0-2 T18 |
| `ConsumerChart` | go/ts | P0-2 T17, P0-3 T16, P0-3 T29 | P0-5 hdr |
| `ConsumerWatchlist` | go/ts | P0-2 T17 | P0-3 T29 |
| `ConsumerWidget` | go/ts | P0-2 T17, P0-3 T19 | P0-3 T29 |
| `counts` | go/ts | P0-1 T8, P0-3 T12, P0-3 T16, P0-3 T30, P0-3 T6, P0-3 hdr, P0-4 T16 | P0-4 T16 |
| `CoversUntil` | go/ts | P0-2 T4, P0-3 T31 | P0-3 T31 |
| `CrashInfo` | go/ts | P0-3 T32, P0-3 hdr, P0-4 T2 | P0-4 T15 |
| `Create` | go/ts | P0-3 T13, P0-3 T22, P0-3 hdr, P0-5 T13 | P0-5 hdr |
| `createAppRouter` | go/ts | P0-4 T1 | P0-4 T2, P0-5 hdr |
| `createdAt` | go/ts | P0-3 T30, P0-3 hdr | P0-4 T16 |
| `CreateEncFile` | go/ts | P0-1 T13, P0-1 T14 | P0-1 T14, P0-1 T18, P0-3 T27 |
| `createMockBackend` | go/ts | P0-4 T2 | P0-5 T10, P0-5 T13, P0-5 T14, P0-5 T15, P0-5 T16, P0-5 T17, P0-5 T2, P0-5 T6, P0-5 hdr |
| `createWailsBackend` | go/ts | P0-4 T2 | P0-5 T2, P0-5 hdr |
| `CredentialsInfo` | go/ts | P0-3 T14, P0-3 T2, P0-3 T22, P0-3 T6, P0-3 hdr, P0-4 T2 | P0-4 T14 |
| `currentBackend` | go/ts | P0-5 T2, P0-5 hdr | P0-5 T10 |
| `DASH` | go/ts | P0-4 T1 | P0-4 T7, P0-5 hdr |
| `Data` | go/ts | P0-1 T11, P0-2 T8, P0-3 T22 | P0-4 T2 |
| `data` | go/ts | P0-2 T8, P0-4 T14 | P0-5 T4 |
| `dataPlan` | go/ts | P0-3 T1, P0-3 T14, P0-3 T27, P0-3 hdr | P0-4 T7 |
| `DataStats` | go/ts | P0-3 T30, P0-3 hdr, P0-4 T2 | P0-4 T16 |
| `DataTable` | go/ts | P0-4 T5 | P0-4 T10, P0-4 T17 |
| `Date` | go/ts | P0-2 T2, P0-2 T21, P0-2 T3, P0-2 T4, P0-2 T5, P0-2 T9, P0-3 T31 | P0-2 T4 |
| `DateFromInt` | go/ts | P0-2 T3 | P0-2 T15 |
| `DateOf` | go/ts | P0-2 T3 | P0-2 T15 |
| `DateSink` | go/ts | P0-2 T9 | P0-2 T9 |
| `Day` | go/ts | P0-2 T10, P0-2 T3, P0-2 T4, P0-3 T12 | P0-3 T16, P0-3 T21 |
| `DayHoliday` | go/ts | P0-2 T3 | P0-3 T31 |
| `DaySpecial` | go/ts | P0-2 T3 | P0-3 T31 |
| `DB` | go/ts | P0-1 T15, P0-1 T7, P0-2 T16, P0-3 T30, P0-4 T2 | P0-1 T11, P0-1 T8 |
| `DB.Backup` | go/ts | P0-1 T8, P0-3 T30 | P0-3 T30 |
| `DB.CalendarOverrides` | go/ts | P0-2 T5 | P0-3 T23, P0-3 T31 |
| `DB.Close` | go/ts | P0-1 T7 | P0-1 T15, P0-2 T5 |
| `DB.DeleteCalendarOverride` | go/ts | P0-2 T5 | P0-3 T31 |
| `DB.R` | go/ts | P0-1 T7 | P0-1 T10, P0-2 T15, P0-2 T16, P0-2 T5, P0-2 hdr |
| `DB.ScheduledBackup` | go/ts | P0-1 T8 | P0-3 T30 |
| `DB.SchemaVersion` | go/ts | P0-1 T7 | P0-3 T30, P0-3 T33, P0-3 T5 |
| `DB.SettingsRepo` | go/ts | P0-1 T10 | P0-1 T11 |
| `DB.SupportFiles` | go/ts | P0-1 T11 | P0-3 T32 |
| `DB.UpsertCalendarOverrides` | go/ts | P0-2 T5 | P0-3 T31 |
| `DB.W` | go/ts | P0-1 T7, P0-3 T22 | P0-1 T10, P0-2 T15, P0-2 T5, P0-2 hdr, P0-3 T13, P0-3 T18, P0-3 T28 |
| `DB.writeTx` | go/ts | P0-2 T5 | P0-2 T15 |
| `deb` | go/ts | P0-1 T16 | P0-5 T19, P0-5 hdr |
| `Default` | go/ts | P0-1 T10, P0-1 T4, P0-3 T2, P0-3 T28 | P0-1 T5 |
| `DEFAULT_DASHBOARD_SYMBOL` | go/ts | P0-5 T10 | P0-5 T11 |
| `defaultBackend` | go/ts | P0-4 T2 | P0-5 T18, P0-5 hdr |
| `DefaultKDF` | go/ts | P0-1 T13, P0-1 T18 | P0-3 T27 |
| `DefaultShortcuts` | go/ts | P0-3 T28, P0-3 hdr | P0-3 hdr |
| `Delete` | go/ts | P0-1 T12, P0-1 T13, P0-3 T22, P0-3 T24, P0-3 T27, P0-3 T6, P0-3 hdr | P0-5 hdr |
| `DeleteOverride` | go/ts | P0-2 T4, P0-2 hdr | P0-3 T31 |
| `demoSeed` | go/ts | P0-4 T2 | P0-5 T10, P0-5 T13, P0-5 T14, P0-5 T15, P0-5 T16, P0-5 T17, P0-5 T2, P0-5 T6, P0-5 hdr |
| `deployments` | go/ts | P0-1 T8, P0-3 hdr | P0-4 T16 |
| `Deps.Calendar` | go/ts | P0-2 T11, P0-2 T16, P0-2 T19, P0-3 T16, P0-3 T22, P0-3 T23 | P0-3 T31 |
| `Deps.DB` | go/ts | P0-1 T15, P0-1 T7, P0-2 T16, P0-3 T30, P0-4 T2 | P0-3 T31 |
| `Destroy` | go/ts | P0-1 T13 | P0-1 T14 |
| `Detect` | go/ts | P0-1 T14, P0-4 T9 | P0-1 T18 |
| `Diagnostics` | go/ts | P0-3 T22 | P0-4 T2 |
| `Dialer` | go/ts | P0-2 T17 | P0-2 T18 |
| `Dialog` | go/ts | P0-4 T18, P0-4 T3 | P0-4 T16, P0-4 T18, P0-4 T7, P0-5 T17, P0-5 T9, P0-5 hdr |
| `DialTicker` | go/ts | P0-3 T9 | P0-3 T25 |
| `displayName` | go/ts | P0-2 T15, P0-3 T15, P0-3 T16, P0-3 hdr, P0-5 T1, P0-5 T17, P0-5 hdr | P0-5 T12, P0-5 T14, P0-5 T17, P0-5 hdr |
| `DropConnections` | go/ts | P0-2 T8 | P0-2 T18 |
| `dsn` | go/ts | P0-1 T7 | P0-1 T8, P0-1 T9 |
| `emit` | go/ts | P0-3 T21, P0-4 T2, P0-5 T5 | P0-4 T19, P0-5 T2, P0-5 hdr |
| `EncFile` | go/ts | P0-1 T13 | P0-1 T14 |
| `EncFile.Close` | go/ts | P0-1 T13 | P0-3 T27 |
| `endSession` | go/ts | P0-3 T6 | P0-3 T8 |
| `ensureInstruments` | go/ts | P0-5 T7 | P0-5 T17 |
| `Entry` | go/ts | P0-2 T3 | P0-2 T4 |
| `equityTokens` | go/ts | P0-2 T20 | P0-2 T21 |
| `Err` | go/ts | P0-1 T10, P0-2 T7, P0-3 T22 | P0-2 T9 |
| `ErrCorruptFile` | go/ts | P0-1 T12 | P0-3 T27 |
| `ErrDuplicate` | go/ts | P0-3 T5 | P0-3 T13 |
| `ErrFileExists` | go/ts | P0-1 T12 | P0-3 T27 |
| `ErrNoRow` | go/ts | P0-3 T5 | P0-3 T13 |
| `ErrNotFound` | go/ts | P0-1 T12 | P0-3 T27, P0-3 T6 |
| `Error` | go/ts | P0-1 T10, P0-1 T2, P0-1 T7, P0-2 T10, P0-2 T17, P0-2 T3, P0-2 T7, P0-2 T9, P0-3 T1, P0-3 T14, P0-3 T21 | P0-3 T1 |
| `ErrPassphraseTooShort` | go/ts | P0-1 T12 | P0-3 T27 |
| `ErrStorageRefused` | go/ts | P0-1 T12 | P0-3 T27 |
| `ErrWrongPassphrase` | go/ts | P0-1 T12 | P0-3 T27 |
| `EventCatalogue` | go/ts | P0-3 T21, P0-3 hdr | P0-3 T33 |
| `EventMap` | go/ts | P0-4 T2 | P0-5 T2, P0-5 hdr |
| `EventName` | go/ts | P0-4 T2 | P0-5 T2 |
| `Events` | go/ts | P0-2 T17 | P0-2 T18 |
| `events.gen.ts` | go/ts | P0-4 T2 | P0-5 hdr |
| `EvJobProgress` | go/ts | P0-3 T21 | P0-3 T22 |
| `EvSessionState` | go/ts | P0-3 T21 | P0-3 T22 |
| `Exchange` | go/ts | P0-2 T15, P0-2 T3, P0-2 T4, P0-2 T5, P0-3 T12, P0-3 T15, P0-3 T16, P0-3 T21, P0-3 T31 | P0-2 T15 |
| `exchange` | go/ts | P0-2 T15, P0-2 T3, P0-3 T12, P0-3 T15, P0-3 T16, P0-3 T21, P0-3 T31, P0-3 T7, P0-3 hdr | P0-5 hdr |
| `EXCHANGE_TAGS` | go/ts | P0-5 T1 | P0-5 T15, P0-5 T17, P0-5 T2, P0-5 T9 |
| `ExchangeTag` | go/ts | P0-5 T1 | P0-5 T17, P0-5 T2 |
| `expiresAtMs` | go/ts | P0-3 T1, P0-3 T14, P0-3 T27, P0-3 hdr | P0-4 T13, P0-4 T7 |
| `expiryMs` | go/ts | P0-3 T16, P0-3 hdr | P0-5 hdr |
| `ExportsDir` | go/ts | P0-1 T11, P0-1 T6 | P0-3 T32 |
| `Fact` | go/ts | P0-4 T14 | P0-4 T16 |
| `failNext` | go/ts | P0-4 T2 | P0-5 hdr |
| `Failure` | go/ts | P0-2 T8 | P0-2 T9 |
| `Fake.HistoricalFunc` | go/ts | P0-2 T8 | P0-2 T14 |
| `Fake.Limiter` | go/ts | P0-2 T6, P0-2 T8, P0-2 T9, P0-3 T23, P0-3 T6 | P0-2 T14 |
| `Fake.Set` | go/ts | P0-2 T1 | P0-3 T8 |
| `fastKDF` | go/ts | P0-1 T13, P0-1 T14 | P0-1 T14 |
| `feed` | go/ts | P0-3 T33, P0-3 T6, P0-3 hdr, P0-4 T2 | P0-3 hdr, P0-5 hdr |
| `feed.ErrNotRunning` | go/ts | P0-3 T16, P0-3 T29, P0-3 T6 | P0-3 T16, P0-3 T29, P0-3 hdr, P0-5 hdr |
| `feed.New` | go/ts | P0-3 T6 | P0-3 T23, P0-3 hdr |
| `Feed.Reconnect` | go/ts | P0-3 T6 | P0-3 T33 |
| `feed.state` | go/ts | P0-2 T17, P0-3 T29, P0-3 hdr, P0-4 T7 | P0-5 T6, P0-5 hdr |
| `Feed.Subscribe` | go/ts | P0-3 T6 | P0-3 T14 |
| `FeedController` | go/ts | P0-3 T33 | P0-3 hdr |
| `FeedState` | go/ts | P0-2 T17, P0-2 hdr, P0-3 T21, P0-3 hdr, P0-4 T2 | P0-3 T29, P0-3 T33, P0-4 T7, P0-5 T6, P0-5 hdr |
| `fetchHistory` | go/ts | P0-5 T2, P0-5 T5, P0-5 hdr | P0-5 T10 |
| `Filter` | go/ts | P0-2 T15 | P0-2 T16 |
| `filterRows` | go/ts | P0-4 T5 | P0-4 T10 |
| `focusNewestToast` | go/ts | P0-4 T4, P0-4 T6 | P0-4 T6 |
| `formatBinding` | go/ts | P0-4 T18, P0-4 T6 | P0-4 T15, P0-4 T18 |
| `formatCompact` | go/ts | P0-4 T1 | P0-5 T10, P0-5 hdr |
| `formatDateLong` | go/ts | P0-4 T1 | P0-4 T17 |
| `formatDateTimeLong` | go/ts | P0-4 T1 | P0-4 T13 |
| `formatExpiry` | go/ts | P0-4 T1 | P0-4 T13, P0-4 T7 |
| `formatPercent` | go/ts | P0-4 T1, P0-4 T12 | P0-5 T10, P0-5 T12, P0-5 T7, P0-5 hdr |
| `formatPrice` | go/ts | P0-4 T1 | P0-5 T10, P0-5 T12, P0-5 T3, P0-5 T7, P0-5 hdr |
| `formatSigned` | go/ts | P0-4 T1, P0-5 T12, P0-5 T7 | P0-4 T7, P0-5 T10, P0-5 T12, P0-5 T7, P0-5 hdr |
| `formatTime` | go/ts | P0-4 T1 | P0-4 T7 |
| `fromChartTime` | go/ts | P0-5 T3 | P0-5 T4 |
| `fromMs` | go/ts | P0-3 T16, P0-3 hdr | P0-5 hdr |
| `fromRow` | go/ts | P0-2 T15 | P0-2 T16 |
| `frontend` | go/ts | P0-1 T16 | P0-3 T23, P0-4 T1, P0-4 T19, P0-4 T2, P0-5 T19, P0-5 hdr |
| `frontend.Assets` | go/ts | P0-1 T1 | P0-1 T15, P0-3 T23 |
| `GenerateSession` | go/ts | P0-3 T6, P0-3 T7 | P0-3 T6 |
| `GetOrders` | go/ts | P0-1 T2 | P0-3 T24 |
| `GetUiState` | go/ts | P0-3 T22, P0-3 T28, P0-3 hdr | P0-3 T22, P0-5 hdr |
| `Go` | go/ts | P0-1 T5, P0-2 T2 | P0-3 T6 |
| `go` | go/ts | P0-4 T19 | P0-3 T22, P0-4 T2, P0-5 T18, P0-5 hdr |
| `GROUP_COMPONENTS` | go/ts | P0-4 T14 | P0-4 T15 |
| `h` | go/ts | P0-2 T10, P0-3 T16, P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Handle` | go/ts | P0-1 T4, P0-3 T6, P0-5 T2 | P0-5 T3 |
| `Header` | go/ts | P0-2 T3 | P0-2 T4 |
| `HistoricalFetcher` | go/ts | P0-3 T16 | P0-3 hdr |
| `HistoricalPick` | go/ts | P0-2 T8, P0-2 T9, P0-3 T16 | P0-3 T16 |
| `HistoricalPicker` | go/ts | P0-2 T7 | P0-2 T7, P0-2 T9 |
| `HM` | go/ts | P0-2 T3 | P0-2 T4 |
| `Holding` | go/ts | P0-3 T12 | P0-3 T12, P0-3 T19 |
| `HoldingRow` | go/ts | P0-3 T12, P0-3 hdr, P0-4 T2 | P0-4 T10 |
| `holdingRow` | go/ts | P0-3 T12, P0-4 T10 | P0-4 T12 |
| `Holdings` | go/ts | P0-2 T7, P0-2 T8, P0-2 T9, P0-3 T14, P0-3 T19, P0-3 T20, P0-3 T22, P0-3 T24, P0-3 hdr | P0-3 T19 |
| `HoldingsSnapshot` | go/ts | P0-3 T12, P0-3 T20, P0-3 hdr, P0-4 T2 | P0-4 T10 |
| `HoldingsSummary` | go/ts | P0-3 T12, P0-3 hdr, P0-4 T2 | P0-4 T10, P0-4 T12 |
| `HomeChartArea` | go/ts | P0-4 T12, P0-5 T10 | P0-5 T10, P0-5 hdr |
| `HomeLayout` | go/ts | P0-4 T1 | P0-4 T12, P0-5 hdr |
| `HomeWatchlistArea` | go/ts | P0-4 T12, P0-5 T16 | P0-5 T16, P0-5 hdr |
| `Icon` | go/ts | P0-4 T1 | P0-4 T3, P0-4 T4, P0-4 T5, P0-5 T8, P0-5 hdr |
| `IconButton` | go/ts | P0-4 T3 | P0-4 T18, P0-4 T4, P0-4 T5, P0-5 T14, P0-5 T16, P0-5 T17, P0-5 T8, P0-5 hdr |
| `IconName` | go/ts | P0-4 T1 | P0-4 T3 |
| `initials` | go/ts | P0-3 T11, P0-3 hdr | P0-4 T8 |
| `Instrument` | go/ts | P0-2 T15, P0-2 T7, P0-2 hdr, P0-5 T1, P0-5 T14, P0-5 T7, P0-5 hdr | P0-5 T2, P0-5 T5 |
| `instrument_fts` | go/ts | P0-2 T15 | P0-2 T15 |
| `instrument_imports` | go/ts | P0-2 T15 | P0-2 T15 |
| `instrumentByIdentifier` | go/ts | P0-5 T2, P0-5 hdr | P0-5 T7 |
| `InstrumentDTO` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T1, P0-5 T2 | P0-5 hdr |
| `InstrumentFinder` | go/ts | P0-3 T14 | P0-3 T16, P0-3 hdr |
| `InstrumentFinder.ByToken` | go/ts | P0-2 T15, P0-2 hdr, P0-3 T14 | P0-3 T15 |
| `InstrumentName` | go/ts | P0-4 T10 | P0-4 T11 |
| `InstrumentRefresher` | go/ts | P0-3 T30 | P0-3 hdr |
| `instruments` | go/ts | P0-2 T8, P0-5 hdr | P0-2 T15, P0-5 hdr |
| `Instruments` | go/ts | P0-2 T7, P0-2 T8, P0-2 T9, P0-3 T14, P0-3 T22 | P0-5 T2 |
| `Instruments.ByIdentifier` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Instruments.ByTokens` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `instruments.Filter` | go/ts | P0-2 T15 | P0-2 T20 |
| `instruments.Instrument` | go/ts | P0-2 T15, P0-2 hdr | P0-3 T16 |
| `instruments.NewMaster` | go/ts | P0-2 T15, P0-2 hdr | P0-3 T23 |
| `instruments.NewRefresher` | go/ts | P0-2 T16 | P0-3 T23 |
| `instruments.Result` | go/ts | P0-2 T15 | P0-3 T16 |
| `Instruments.Search` | go/ts | P0-3 T22, P0-3 hdr, P0-5 T9 | P0-5 hdr |
| `instruments.Tag` | go/ts | P0-2 T15, P0-3 T16 | P0-3 T16 |
| `instrumentsByTokens` | go/ts | P0-5 T2, P0-5 hdr | P0-5 T7 |
| `InstrumentSpec` | go/ts | P0-5 T2 | P0-5 T3, P0-5 T4 |
| `InstrumentsReadyAt` | go/ts | P0-2 T4 | P0-2 T16 |
| `instrumentType` | go/ts | P0-2 T15, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `Interactive` | go/ts | P0-2 T6, P0-3 T16 | P0-5 hdr |
| `internal` | go/ts | P0-3 hdr | P0-2 T1, P0-2 T22, P0-2 hdr, P0-3 T12, P0-3 T22, P0-3 T23, P0-3 hdr |
| `interval` | go/ts | P0-2 T10, P0-2 T14, P0-2 T19, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `invalid_consumer` | go/ts | P0-3 hdr | P0-5 hdr |
| `isEnabled` | go/ts | P0-4 T1, P0-4 T13, P0-4 T8 | P0-4 T8 |
| `IsFuture` | go/ts | P0-2 T15, P0-3 T16 | P0-3 T16 |
| `isFuture` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T1 | P0-5 hdr |
| `isIndex` | go/ts | P0-2 T15, P0-2 T17, P0-3 T15, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `isInScope` | go/ts | P0-5 T1, P0-5 hdr | P0-5 T2 |
| `IST` | go/ts | P0-2 T1 | P0-2 T2 |
| `istDayKey` | go/ts | P0-4 T1 | P0-4 T17 |
| `IsTradingDay` | go/ts | P0-2 T4 | P0-3 T31 |
| `IsWrite` | go/ts | P0-3 T24 | P0-3 T24 |
| `itemIdentifier` | go/ts | P0-5 T1, P0-5 hdr | P0-5 T16 |
| `jobId` | go/ts | P0-2 T14, P0-3 T21, P0-3 hdr, P0-5 T5 | P0-5 hdr |
| `JobProgress` | go/ts | P0-2 T10, P0-3 T21, P0-3 hdr, P0-4 T2 | P0-2 T12, P0-3 T22, P0-3 T30, P0-4 T16 |
| `JobStore` | go/ts | P0-2 T10, P0-2 hdr | P0-2 T12 |
| `JobSupportBundle` | go/ts | P0-3 T30 | P0-3 T32 |
| `Key` | go/ts | P0-1 T10, P0-1 T12, P0-3 T12, P0-3 T2, P0-3 T28 | P0-1 T13 |
| `KeyAPISecret` | go/ts | P0-1 T12 | P0-3 T27 |
| `KindSession` | go/ts | P0-2 T7 | P0-3 T6 |
| `kite` | go/ts | P0-1 T16, P0-2 T17, P0-3 T6 | P0-3 hdr |
| `kite.AllowedHosts` | go/ts | P0-3 T6 | P0-3 T23, P0-3 hdr |
| `kite.Classify` | go/ts | P0-2 T7 | P0-2 T14, P0-3 T6 |
| `kite.Client` | go/ts | P0-2 T9 | P0-3 T6 |
| `kite.Err` | go/ts | P0-2 T7 | P0-2 T14 |
| `kite.ErrHostNotAllowed` | go/ts | P0-3 T6 | P0-3 hdr |
| `kite.HistoricalData` | go/ts | P0-2 T7 | P0-2 T14, P0-3 T16 |
| `kite.HistoricalPicker` | go/ts | P0-2 T7 | P0-2 T14, P0-2 T8, P0-3 T16 |
| `kite.HistoricalRequest` | go/ts | P0-2 T7 | P0-2 T14, P0-2 T8, P0-3 T16 |
| `kite.Holding` | go/ts | P0-3 T12 | P0-3 T12, P0-3 hdr |
| `kite.Holdings` | go/ts | P0-2 T7, P0-2 T9 | P0-3 T12 |
| `kite.Instrument` | go/ts | P0-2 T7 | P0-2 T15 |
| `kite.Instruments` | go/ts | P0-2 T7, P0-2 T9 | P0-2 T15 |
| `kite.IsNoDataPermission` | go/ts | P0-3 T9 | P0-3 T19, P0-3 T29 |
| `kite.New` | go/ts | P0-2 T9, P0-2 hdr | P0-2 T15, P0-2 T20, P0-3 T23 |
| `kite.NewAllowListTransport` | go/ts | P0-3 T6 | P0-3 T23, P0-3 hdr |
| `kite.NiftyIndexToken` | go/ts | P0-3 T9 | P0-3 T23 |
| `kite.Order` | go/ts | P0-2 T7, P0-3 hdr | P0-3 T12, P0-3 T19, P0-3 T21 |
| `kite.Orders` | go/ts | P0-2 T7, P0-2 T9 | P0-3 T12 |
| `kite.Position` | go/ts | P0-3 T12 | P0-3 T12, P0-3 hdr |
| `kite.Positions` | go/ts | P0-2 T7, P0-2 T9 | P0-3 T12 |
| `kite.QuoteLTP` | go/ts | P0-2 T7 | P0-3 T29 |
| `kite.QuoteOHLC` | go/ts | P0-2 T7 | P0-3 T29 |
| `kite.ReadClient` | go/ts | P0-2 T7 | P0-2 T16, P0-2 T8 |
| `kite.ReadClient.LTP` | go/ts | P0-1 T3, P0-2 T17, P0-2 T8, P0-2 T9, P0-3 T16, P0-3 T19, P0-3 T24, P0-3 T29 | P0-3 T29 |
| `kite.redirect_port` | go/ts | P0-3 hdr | P0-4 T14 |
| `kite.Session` | go/ts | P0-3 T6 | P0-3 T24 |
| `kite.SessionErrorRelay` | go/ts | P0-3 T6 | P0-3 T23, P0-3 hdr |
| `kite.Time` | go/ts | P0-2 T7 | P0-2 T14 |
| `kite.TopicOrderUpdate` | go/ts | P0-2 T7, P0-3 T21, P0-3 hdr | P0-2 T17, P0-3 T19, P0-3 T21 |
| `kite.TopicSessionState` | go/ts | P0-3 T1, P0-3 hdr | P0-3 T21 |
| `kiteconnect.Error` | go/ts | P0-1 T10, P0-1 T2, P0-1 T7, P0-2 T10, P0-2 T17, P0-2 T3, P0-2 T7, P0-2 T9, P0-3 T1, P0-3 T14, P0-3 T21 | P0-3 T1 |
| `kiteconnect.New` | go/ts | P0-3 T23 | P0-3 T6 |
| `kiteconnect.UserProfile` | go/ts | P0-2 T7 | P0-3 T11 |
| `KiteDialer` | go/ts | P0-2 T18 | P0-2 T20, P0-2 T21 |
| `kiteFixture` | go/ts | P0-2 T15 | P0-2 T16 |
| `KiteInterval` | go/ts | P0-5 T1 | P0-5 T2 |
| `KiteOrder` | go/ts | P0-4 T2 | P0-4 T10 |
| `kiteOrder` | go/ts | P0-4 T2 | P0-4 T12 |
| `l` | go/ts | P0-2 T10, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `LabelAt` | go/ts | P0-2 T4, P0-2 hdr | P0-2 T5, P0-3 T21 |
| `LabelInstrument` | go/ts | P0-5 T1 | P0-5 T14, P0-5 T7 |
| `labelOfItem` | go/ts | P0-5 T1, P0-5 T7, P0-5 hdr | P0-5 T14 |
| `LastQuotes` | go/ts | P0-3 T21 | P0-3 T29 |
| `lastTick` | go/ts | P0-2 T17, P0-3 hdr | P0-4 T7 |
| `latest` | go/ts | P0-3 T21 | P0-5 T6 |
| `Layout` | go/ts | P0-1 T15, P0-1 T6, P0-3 T30 | P0-1 T7, P0-1 T8 |
| `Layout.AppDB` | go/ts | P0-1 T6 | P0-1 T7 |
| `Layout.BackupsDir` | go/ts | P0-1 T6 | P0-1 T7 |
| `Layout.Ensure` | go/ts | P0-1 T6, P0-2 T14, P0-2 hdr | P0-1 T15, P0-1 T9, P0-3 T22 |
| `LegalPage` | go/ts | P0-4 T9 | P0-4 T18 |
| `LegendData` | go/ts | P0-5 T2 | P0-5 T4 |
| `licence` | go/ts | P0-1 T16, P0-3 T33, P0-3 hdr | P0-5 T19, P0-5 hdr |
| `licences` | go/ts | P0-3 T33, P0-3 hdr | P0-3 hdr |
| `licences.Notices` | go/ts | P0-3 T33 | P0-3 T23, P0-3 hdr |
| `Limiter.Do` | go/ts | P0-2 T6 | P0-2 T9 |
| `Limiter.PauseAll` | go/ts | P0-2 T6 | P0-2 T14 |
| `List` | go/ts | P0-3 T13, P0-3 T22, P0-3 hdr | P0-5 hdr |
| `ListBody` | go/ts | P0-4 T5 | P0-4 T10 |
| `listenerCount` | go/ts | P0-4 T2 | P0-5 hdr |
| `listenerError` | go/ts | P0-3 T7, P0-3 hdr | P0-4 T9 |
| `listenerOk` | go/ts | P0-3 T7, P0-3 hdr | P0-4 T9 |
| `ListPage` | go/ts | P0-4 T5 | P0-4 T10, P0-4 T13, P0-4 T18 |
| `ListStatus` | go/ts | P0-4 T5 | P0-4 T10 |
| `listStatusOf` | go/ts | P0-4 T10 | P0-4 T11 |
| `live` | go/ts | P0-3 T29, P0-3 hdr | P0-3 T29 |
| `liveEnv` | go/ts | P0-2 T20 | P0-2 T21 |
| `LiveSubscriber` | go/ts | P0-3 T14, P0-3 T6 | P0-3 T16, P0-3 T29, P0-3 hdr |
| `LiveTokens` | go/ts | P0-3 T19, P0-3 T6 | P0-3 T19 |
| `Load` | go/ts | P0-1 T10, P0-1 T2, P0-2 T16, P0-2 hdr, P0-3 T2, P0-3 T28 | P0-3 T23 |
| `Lock.MarkClean` | go/ts | P0-1 T6 | P0-1 T15 |
| `Lock.PreviousExit` | go/ts | P0-1 T6 | P0-1 T15 |
| `Lock.Release` | go/ts | P0-1 T6 | P0-1 T15 |
| `log.level` | go/ts | P0-3 hdr, P0-4 T15 | P0-4 T15 |
| `logger.ERROR` | go/ts | P0-3 T22 | P0-3 T22 |
| `Logger.Level` | go/ts | P0-1 T4 | P0-1 T15 |
| `logging.BundleFile` | go/ts | P0-1 T11 | P0-3 T32 |
| `logging.BundleOptions` | go/ts | P0-1 T11 | P0-3 T32 |
| `logging.CreateSupportBundle` | go/ts | P0-1 T11 | P0-3 T32 |
| `logging.Default` | go/ts | P0-1 T4, P0-3 T2 | P0-3 T32 |
| `logging.Default.IsSensitiveKey` | go/ts | P0-3 T2 | P0-1 T10 |
| `logging.Default.RedactString` | go/ts | P0-1 T4 | P0-1 T10 |
| `logging.Go` | go/ts | P0-1 T5, P0-2 T2 | P0-1 T12, P0-1 T15, P0-2 T14, P0-2 T17, P0-2 T6, P0-2 T8, P0-3 T23, P0-3 T30, P0-3 T4 |
| `logging.InstallCrashReporter` | go/ts | P0-1 T5 | P0-1 T15 |
| `logging.MarkCrashSeen` | go/ts | P0-1 T5, P0-3 T32 | P0-3 T32 |
| `logging.New` | go/ts | P0-1 T4 | P0-1 T15 |
| `logging.ParseLevel` | go/ts | P0-1 T4 | P0-1 T15 |
| `logging.RegisterSecret` | go/ts | P0-1 T12, P0-1 T4 | P0-1 T10, P0-1 T12, P0-1 T13, P0-3 T27, P0-3 T6 |
| `logging.UnseenCrashReports` | go/ts | P0-1 T5 | P0-1 T15 |
| `LoginError` | go/ts | P0-3 T1 | P0-3 T3 |
| `LoginStart` | go/ts | P0-3 T14, P0-3 T7, P0-3 hdr, P0-4 T2 | P0-4 T9 |
| `loginViaPaste` | go/ts | P0-3 T10, P0-3 T11, P0-3 T7, P0-3 T8 | P0-3 T8 |
| `LogLevel` | go/ts | P0-1 T10, P0-3 hdr | P0-3 T2 |
| `LogoutResult` | go/ts | P0-3 T10, P0-3 T14, P0-3 hdr, P0-4 T2 | P0-4 T13 |
| `LogsDir` | go/ts | P0-1 T11, P0-1 T6 | P0-3 T32 |
| `lookup` | go/ts | P0-2 T4, P0-3 T2 | P0-2 T5 |
| `lotSize` | go/ts | P0-2 T15, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `ltp` | go/ts | P0-2 T17, P0-3 T16, P0-3 hdr, P0-4 T2 | P0-5 hdr |
| `main` | go/ts | P0-1 T1, P0-1 T15, P0-1 T16, P0-1 T18, P0-3 T21, P0-3 T22, P0-3 T23, P0-3 T30, P0-3 hdr, P0-4 T16 | P0-4 T16 |
| `Manager` | go/ts | P0-2 T17, P0-2 hdr | P0-2 T18 |
| `Manager.Raw` | go/ts | P0-2 T17 | P0-2 T19 |
| `Manager.Subscribe` | go/ts | P0-2 T17 | P0-2 T20 |
| `Manifest` | go/ts | P0-1 T8, P0-1 T9 | P0-1 T9 |
| `ManifestFormat` | go/ts | P0-1 T8 | P0-1 T9 |
| `market` | go/ts | P0-3 T33, P0-3 hdr, P0-4 T2 | P0-5 hdr |
| `MarketStatus` | go/ts | P0-3 T21, P0-3 hdr, P0-4 T2 | P0-3 T33, P0-4 T7 |
| `MaskUserID` | go/ts | P0-3 T1 | P0-3 T25 |
| `Master` | go/ts | P0-2 T15, P0-2 T16, P0-3 T23 | P0-2 T16 |
| `Master.Search` | go/ts | P0-2 T15 | P0-3 T14 |
| `MenuButton` | go/ts | P0-4 T3 | P0-4 T8 |
| `MenuItem` | go/ts | P0-4 T3 | P0-4 T8 |
| `mergeBars` | go/ts | P0-5 T3 | P0-5 T5 |
| `MergeOverrides` | go/ts | P0-2 T4, P0-2 hdr | P0-2 T10, P0-2 T5 |
| `Messages` | go/ts | P0-2 T8 | P0-2 T18 |
| `Migrate` | go/ts | P0-1 T14 | P0-1 T18 |
| `Migrator.Apply` | go/ts | P0-1 T7 | P0-2 T11 |
| `Migrator.Latest` | go/ts | P0-1 T7 | P0-2 T11 |
| `MinPassphraseRunes` | go/ts | P0-1 T13 | P0-3 T27 |
| `Missing` | go/ts | P0-2 T11, P0-3 T14, P0-3 T15 | P0-2 T13, P0-3 T14 |
| `missing` | go/ts | P0-3 T15, P0-3 hdr, P0-5 T14 | P0-5 hdr |
| `mock.NewFake` | go/ts | P0-2 T8 | P0-2 T14 |
| `mock.NewServer` | go/ts | P0-2 T8 | P0-2 T15, P0-2 T18, P0-2 T9 |
| `MockBackend` | go/ts | P0-4 T2 | P0-5 T18, P0-5 T2, P0-5 hdr |
| `MockState` | go/ts | P0-4 T2 | P0-5 T2, P0-5 hdr |
| `ModeChoose` | go/ts | P0-1 T14 | P0-3 T27 |
| `ModeFile` | go/ts | P0-1 T14, P0-1 T18 | P0-3 T27 |
| `ModeKeyring` | go/ts | P0-1 T14 | P0-3 T27 |
| `Ms` | go/ts | P0-4 T2 | P0-5 T2 |
| `mustCalendar` | go/ts | P0-3 T16 | P0-3 T21 |
| `mustExec` | go/ts | P0-1 T11, P0-1 T8 | P0-1 T11 |
| `name` | go/ts | P0-2 T15, P0-3 T11, P0-3 T15, P0-3 T16, P0-3 T17, P0-3 T26, P0-3 T33, P0-3 hdr | P0-5 hdr |
| `New` | go/ts | P0-1 T1, P0-1 T15, P0-1 T2, P0-1 T3, P0-1 T4, P0-2 T14, P0-2 T17, P0-2 T19, P0-2 T2, P0-2 T4, P0-2 T6, P0-2 T9, P0-2 hdr, P0-3 T14, P0-3 T22, P0-3 T23, P0-3 T6, P0-3 hdr, P0-5 T16 | P0-3 T6, P0-3 T9 |
| `NewAuth` | go/ts | P0-3 T6 | P0-3 T25 |
| `NewKeyring` | go/ts | P0-1 T12, P0-1 T14, P0-3 T27 | P0-1 T14, P0-1 T18 |
| `NewLayout` | go/ts | P0-1 T6 | P0-1 T8, P0-1 T9 |
| `newNonce` | go/ts | P0-3 T7 | P0-3 T25 |
| `NewNone` | go/ts | P0-1 T12, P0-1 T14 | P0-3 T23 |
| `NewRecorder` | go/ts | P0-3 T24 | P0-3 T24 |
| `NewRedactor` | go/ts | P0-1 T4 | P0-1 T5 |
| `NewSecretSwitch` | go/ts | P0-3 T27 | P0-3 T22 |
| `nextRetry` | go/ts | P0-2 T17, P0-3 hdr | P0-4 T7 |
| `NextTradingDay` | go/ts | P0-2 T4 | P0-2 T16 |
| `nextWatchlistName` | go/ts | P0-5 T13 | P0-5 T16, P0-5 T17 |
| `NiftyIndexToken` | go/ts | P0-3 T25, P0-3 T9 | P0-3 T25 |
| `no_data_plan` | go/ts | P0-3 T16, P0-3 hdr | P0-5 hdr |
| `none` | go/ts | P0-3 T16, P0-3 T27, P0-3 T29, P0-3 T9, P0-3 hdr | P0-5 hdr |
| `not_found` | go/ts | P0-3 T16, P0-3 hdr | P0-5 hdr |
| `NotFoundView` | go/ts | P0-4 T5 | P0-4 T14, P0-4 T18 |
| `NSE` | go/ts | P0-2 T3, P0-3 T16 | P0-2 T15, P0-5 hdr |
| `o` | go/ts | P0-2 T10, P0-3 T16, P0-3 hdr | P0-3 T6, P0-5 hdr |
| `ObserveHTTPDate` | go/ts | P0-2 T2, P0-2 hdr | P0-3 T23 |
| `Observer` | go/ts | P0-2 T17 | P0-2 T21 |
| `offline` | go/ts | P0-3 T1, P0-3 T14, P0-3 T16, P0-3 T27, P0-3 hdr | P0-5 hdr |
| `Offset` | go/ts | P0-2 T1, P0-2 T2, P0-2 hdr, P0-3 hdr | P0-2 T2, P0-2 T21 |
| `OHLC` | go/ts | P0-2 T8, P0-2 T9, P0-3 T29 | P0-3 T29 |
| `oi` | go/ts | P0-2 T10, P0-2 T17, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `on` | go/ts | P0-4 T2 | P0-5 hdr |
| `onBackfillProgress` | go/ts | P0-5 T2, P0-5 T5, P0-5 hdr | P0-5 T10 |
| `onCandle` | go/ts | P0-5 T2, P0-5 T5, P0-5 hdr | P0-5 T10 |
| `onClose` | go/ts | P0-4 T14, P0-4 T15, P0-4 T3, P0-4 T5, P0-5 T17, P0-5 T9 | P0-5 hdr |
| `OnConnect` | go/ts | P0-2 T17 | P0-3 T9 |
| `OnError` | go/ts | P0-2 T17 | P0-3 T9 |
| `Open` | go/ts | P0-1 T7, P0-2 T10, P0-2 T11, P0-2 T17, P0-2 T8, P0-2 hdr, P0-3 T12, P0-3 T16, P0-3 T5, P0-5 T8 | P0-1 T8, P0-2 T13 |
| `openApp` | go/ts | P0-4 T19 | P0-5 T18, P0-5 hdr |
| `OpenDirectoryDialog` | go/ts | P0-3 T22 | P0-3 T22 |
| `OpenEncFile` | go/ts | P0-1 T13, P0-1 T14 | P0-1 T18, P0-3 T27 |
| `OpenFileDialog` | go/ts | P0-3 T22 | P0-3 T22 |
| `OpenLive` | go/ts | P0-3 T16, P0-3 T22, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `openLive` | go/ts | P0-5 T2, P0-5 T5, P0-5 hdr | P0-5 T10 |
| `openTest` | go/ts | P0-1 T7, P0-2 T11 | P0-2 T12, P0-2 T13 |
| `Options` | go/ts | P0-1 T4, P0-1 T7, P0-2 T11, P0-2 T14, P0-2 T16, P0-2 T17, P0-2 T19, P0-2 T2, P0-2 T20, P0-2 T21, P0-2 T6, P0-2 T9, P0-2 hdr, P0-3 T22, P0-3 T6 | P0-1 T8, P0-2 T18 |
| `options` | go/ts | P0-5 T3 | P0-3 T22 |
| `Options.System` | go/ts | P0-2 T1, P0-2 T2, P0-3 T22 | P0-2 T2, P0-2 T21 |
| `Order` | go/ts | P0-2 T7, P0-3 T21, P0-3 hdr, P0-4 T2 | P0-3 T12 |
| `order_journal` | go/ts | P0-1 T11, P0-1 T8, P0-3 hdr | P0-4 T16 |
| `OrderRow` | go/ts | P0-3 T12, P0-3 hdr, P0-4 T11, P0-4 T2 | P0-4 T10, P0-4 T11 |
| `orders` | go/ts | P0-2 T8, P0-3 T12, P0-3 T21, P0-3 hdr | P0-3 T12 |
| `Orders` | go/ts | P0-1 T2, P0-2 T7, P0-2 T8, P0-2 T9, P0-3 T12, P0-3 T14, P0-3 T19, P0-3 T24 | P0-3 T19 |
| `orders.ParseScope` | go/ts | P0-3 T12 | P0-3 T20 |
| `orders.Scope` | go/ts | P0-3 T12 | P0-3 T14 |
| `orders.Sync` | go/ts | P0-3 T19 | P0-3 T24 |
| `orders.TopicBook` | go/ts | P0-3 T19, P0-3 hdr | P0-3 T21 |
| `orders_cache` | go/ts | P0-3 T18 | P0-3 T18 |
| `OrderSection` | go/ts | P0-4 T2 | P0-4 T11 |
| `orderStatusLabel` | go/ts | P0-4 T10 | P0-4 T11 |
| `org.freedesktop.Notifications.GetServerInformation` | go/ts | P0-3 T26 | P0-3 T26 |
| `Overrides` | go/ts | P0-2 T4 | P0-3 T31 |
| `P0_VIEWS` | go/ts | P0-4 T19 | P0-5 T18, P0-5 hdr |
| `Packet` | go/ts | P0-2 T8 | P0-2 T18 |
| `parseBackendError` | go/ts | P0-4 T2 | P0-4 T10, P0-5 T2, P0-5 hdr |
| `ParseBinding` | go/ts | P0-3 T28, P0-3 hdr | P0-3 hdr |
| `ParseFile` | go/ts | P0-2 T3, P0-3 T31 | P0-2 T10, P0-2 T4 |
| `ParseRedirect` | go/ts | P0-3 T3 | P0-3 T25 |
| `parseTimeframe` | go/ts | P0-5 T1, P0-5 hdr | P0-5 T11 |
| `path` | go/ts | P0-3 T30, P0-3 T32, P0-3 hdr | P0-4 T15 |
| `PauseAll` | go/ts | P0-2 T6, P0-3 T24, P0-3 T6 | P0-2 T9 |
| `personalSeed` | go/ts | P0-4 T2 | P0-5 T2, P0-5 hdr |
| `Phase` | go/ts | P0-2 T3, P0-3 T21, P0-4 T1 | P0-4 T14 |
| `PhaseContinuous` | go/ts | P0-2 T3 | P0-2 T19 |
| `PhaseKind` | go/ts | P0-2 T3, P0-4 T2 | P0-4 T17 |
| `PhaseSpec` | go/ts | P0-2 T3 | P0-2 T4 |
| `Portfolio` | go/ts | P0-3 T22 | P0-4 T2 |
| `PortfolioSync` | go/ts | P0-3 T14 | P0-3 T20 |
| `Position` | go/ts | P0-3 T12, P0-3 T13, P0-3 T15 | P0-3 T12, P0-3 T19 |
| `positionNetChangePct` | go/ts | P0-4 T10 | P0-4 T11 |
| `positionPnl` | go/ts | P0-4 T10 | P0-4 T11 |
| `PositionRow` | go/ts | P0-3 T12, P0-3 hdr, P0-4 T2 | P0-4 T10, P0-4 T11 |
| `Positions` | go/ts | P0-2 T7, P0-2 T8, P0-2 T9, P0-3 T14, P0-3 T19, P0-3 T20, P0-3 T22, P0-3 T24, P0-3 hdr | P0-3 T19 |
| `positions_cache` | go/ts | P0-3 T18 | P0-3 T18 |
| `PositionsSnapshot` | go/ts | P0-3 T12, P0-3 T20, P0-3 hdr, P0-4 T2 | P0-4 T10 |
| `precisionForTick` | go/ts | P0-5 T3 | P0-5 T5, P0-5 T8 |
| `PrevTradingDay` | go/ts | P0-2 T4 | P0-3 T16 |
| `PriceBandSpec` | go/ts | P0-5 T2 | P0-5 T3 |
| `PriorityFrom` | go/ts | P0-2 T6 | P0-2 T9 |
| `ProbeKeyring` | go/ts | P0-1 T12, P0-1 T15 | P0-1 T14 |
| `ProbeKeyringWithin` | go/ts | P0-1 T12 | P0-1 T18 |
| `ProbeTimeout` | go/ts | P0-1 T12, P0-1 T18 | P0-1 T18 |
| `Profile` | go/ts | P0-2 T8, P0-2 T9, P0-3 T11, P0-3 T14, P0-3 T22, P0-3 T24, P0-3 T6, P0-3 hdr, P0-4 T2 | P0-4 T13 |
| `profile` | go/ts | P0-4 T2 | P0-5 hdr |
| `progress` | go/ts | P0-3 T30 | P0-3 T32 |
| `Progress` | go/ts | P0-2 T12, P0-2 T14, P0-2 hdr, P0-3 hdr | P0-5 hdr |
| `PushTicks` | go/ts | P0-2 T8 | P0-2 T18 |
| `pushToast` | go/ts | P0-4 T10, P0-4 T13, P0-4 T14, P0-4 T15, P0-4 T16, P0-4 T17, P0-4 T4, P0-4 T7, P0-4 T8, P0-4 T9, P0-5 T14, P0-5 T17 | P0-4 T10, P0-4 T13, P0-4 T7, P0-4 T8, P0-4 T9, P0-5 T14, P0-5 T15, P0-5 T17, P0-5 hdr |
| `quickCheck` | go/ts | P0-1 T7 | P0-1 T9 |
| `Quit` | go/ts | P0-3 T22 | P0-3 T22 |
| `QUOTE_CONSUMERS` | go/ts | P0-5 T2 | P0-5 T6 |
| `quoteChange` | go/ts | P0-5 T6, P0-5 hdr | P0-5 T12, P0-5 T7 |
| `QuoteConsumer` | go/ts | P0-5 T2 | P0-5 T6 |
| `QuoteLTP` | go/ts | P0-2 T7 | P0-3 T19 |
| `QuoteReader` | go/ts | P0-3 T29 | P0-3 hdr |
| `quotes` | go/ts | P0-5 hdr | P0-5 hdr |
| `Quotes.Watch` | go/ts | P0-3 T16, P0-3 T21, P0-3 T22, P0-3 hdr, P0-5 hdr | P0-5 T2, P0-5 hdr |
| `R` | go/ts | P0-1 T7 | P0-3 T13, P0-3 T18, P0-3 T28, P0-3 T5 |
| `RangeInfo` | go/ts | P0-5 T2 | P0-5 T3 |
| `ratelimit.ClassHistorical` | go/ts | P0-2 T6 | P0-2 T8 |
| `ratelimit.Interactive` | go/ts | P0-2 T6 | P0-3 T16 |
| `ratelimit.Limiter` | go/ts | P0-2 T6 | P0-2 T8 |
| `ratelimit.New` | go/ts | P0-2 T6, P0-2 hdr | P0-2 T14, P0-2 T15, P0-2 T20, P0-2 T9, P0-3 T23, P0-3 T6 |
| `ratelimit.Priority` | go/ts | P0-2 T6 | P0-2 T14, P0-2 T7, P0-2 T8 |
| `ratelimit.TopicUsage` | go/ts | P0-2 T6, P0-3 hdr | P0-3 T21 |
| `ratelimit.Usage` | go/ts | P0-2 T6, P0-3 hdr | P0-3 T21 |
| `ratelimit.WithPriority` | go/ts | P0-2 T6 | P0-2 T16 |
| `Raw` | go/ts | P0-2 T17, P0-2 hdr | P0-2 T20 |
| `Read` | go/ts | P0-2 T11, P0-3 T14, P0-3 T16, P0-3 T19 | P0-2 T13 |
| `readAppVersion` | go/ts | P0-1 T7 | P0-1 T11, P0-1 T9 |
| `ReadClient` | go/ts | P0-2 T7, P0-2 hdr | P0-2 T9 |
| `ReadClient.Historical` | go/ts | P0-2 T8, P0-2 T9, P0-3 T16, P0-3 T24, P0-3 T9 | P0-3 T9 |
| `readySession` | go/ts | P0-4 T2 | P0-5 T10, P0-5 hdr |
| `Real.ObserveHTTPDate` | go/ts | P0-2 T2 | P0-2 T9 |
| `Real.ObserveTick` | go/ts | P0-2 T2 | P0-2 T17 |
| `Rebind` | go/ts | P0-3 T22, P0-3 T28, P0-3 hdr | P0-4 T15 |
| `Reconnect` | go/ts | P0-2 T17, P0-2 hdr, P0-3 T33, P0-3 T6 | P0-3 hdr |
| `Recorder` | go/ts | P0-2 T6, P0-3 T24 | P0-3 T24 |
| `Redactor` | go/ts | P0-1 T11, P0-1 T4, P0-1 T5 | P0-1 T11, P0-1 T5 |
| `RedactString` | go/ts | P0-1 T4 | P0-1 T5 |
| `refresh` | go/ts | P0-3 T19, P0-4 T2, P0-5 T2, P0-5 T5 | P0-4 T11, P0-5 T8, P0-5 hdr |
| `Refresh` | go/ts | P0-2 T16, P0-2 hdr, P0-3 T14, P0-3 T19, P0-3 T22, P0-3 T30, P0-3 hdr, P0-4 T10 | P0-5 hdr |
| `refreshRange` | go/ts | P0-5 T2, P0-5 T5, P0-5 hdr | P0-5 T10 |
| `RefreshRequest` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `refused` | go/ts | P0-3 T14 | P0-3 T28 |
| `registerReset` | go/ts | P0-4 T2 | P0-4 T10, P0-4 T5, P0-5 hdr |
| `registry.go` | go/ts | P0-3 T2 | P0-3 hdr |
| `Release` | go/ts | P0-1 T6, P0-2 T17, P0-2 hdr, P0-3 T14, P0-3 T19, P0-3 T24, P0-3 T6 | P0-2 T20, P0-3 T14, P0-3 hdr |
| `reload` | go/ts | P0-4 T8 | P0-4 T11, P0-4 T12 |
| `reloadSettings` | go/ts | P0-4 T2 | P0-4 T16 |
| `reloadShortcuts` | go/ts | P0-4 T6 | P0-4 T15 |
| `rememberTicks` | go/ts | P0-3 T21 | P0-3 T29 |
| `Remove` | go/ts | P0-3 T22, P0-3 hdr, P0-4 T2 | P0-5 hdr |
| `RemovedItem` | go/ts | P0-3 T15, P0-3 hdr, P0-5 T2 | P0-5 T13, P0-5 hdr |
| `Rename` | go/ts | P0-3 T13, P0-3 T22, P0-3 hdr | P0-5 hdr |
| `renderApp` | go/ts | P0-4 T10, P0-4 T11, P0-4 T12, P0-4 T13, P0-4 T14, P0-4 T15, P0-4 T16, P0-4 T17, P0-4 T18, P0-4 T2, P0-4 T7, P0-4 T8 | P0-4 T5, P0-5 hdr |
| `Reorder` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `ReorderLists` | go/ts | P0-3 T13, P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Request` | go/ts | P0-3 T24 | P0-3 T24 |
| `resetAllStores` | go/ts | P0-4 T17, P0-4 T2, P0-4 T8 | P0-5 hdr |
| `ResetShortcuts` | go/ts | P0-3 T22, P0-3 T28, P0-3 hdr | P0-4 T15 |
| `resolveIdentifier` | go/ts | P0-5 T7, P0-5 hdr | P0-5 T12, P0-5 T9 |
| `Restore` | go/ts | P0-1 T9, P0-3 T14, P0-3 T22, P0-3 T27, P0-3 T30, P0-3 T6, P0-3 hdr | P0-3 T22, P0-3 T27 |
| `Resume` | go/ts | P0-2 T6, P0-2 hdr, P0-3 T24, P0-3 T6 | P0-2 T14 |
| `resumeSettingsWrites` | go/ts | P0-4 T16, P0-4 T2 | P0-4 T16 |
| `riskAckRequired` | go/ts | P0-3 T1, P0-3 T14, P0-3 T27, P0-3 hdr | P0-4 T9 |
| `riskDisclosure` | go/ts | P0-4 T9 | P0-4 T18 |
| `Root` | go/ts | P0-1 T6, P0-3 T6 | P0-3 T30 |
| `rootChildren` | go/ts | P0-4 T1, P0-4 T9, P0-5 T11, P0-5 hdr | P0-5 T11, P0-5 hdr |
| `route` | go/ts | P0-3 T21, P0-3 hdr | P0-3 T24 |
| `Run` | go/ts | P0-1 T15, P0-2 T16, P0-2 T19, P0-2 T6, P0-2 hdr, P0-3 T14, P0-3 T19, P0-3 T22, P0-3 T23, P0-3 T26, P0-3 T8 | P0-2 T20, P0-3 T22, P0-3 T23 |
| `RunBackups` | go/ts | P0-3 T30, P0-3 hdr | P0-3 T22 |
| `RunQuotes` | go/ts | P0-3 T29, P0-3 hdr | P0-3 T22 |
| `RunSession` | go/ts | P0-3 T14, P0-3 hdr | P0-3 T22 |
| `runtime` | go/ts | P0-3 T22 | P0-3 T22, P0-4 T2 |
| `rupees` | go/ts | P0-4 T10 | P0-4 T11 |
| `safeLastRoute` | go/ts | P0-4 T6, P0-4 T9 | P0-4 T9 |
| `scanners` | go/ts | P0-1 T8, P0-3 hdr | P0-4 T16 |
| `SchemaTooNewError` | go/ts | P0-1 T7 | P0-1 T9 |
| `Scope` | go/ts | P0-3 T12, P0-3 T28 | P0-3 T19 |
| `ScrubBytes` | go/ts | P0-1 T4 | P0-1 T11, P0-1 T5 |
| `Search` | go/ts | P0-2 T15, P0-2 hdr, P0-3 T14, P0-3 T22, P0-3 hdr, P0-5 T2, P0-5 T9 | P0-5 hdr |
| `search` | go/ts | P0-5 T2 | P0-5 T8, P0-5 hdr |
| `searchInstruments` | go/ts | P0-5 T15, P0-5 T2, P0-5 T9, P0-5 hdr | P0-5 T15, P0-5 T17, P0-5 T9 |
| `secrets.Choice` | go/ts | P0-1 T14 | P0-3 T27 |
| `secrets.CreateEncFile` | go/ts | P0-1 T13 | P0-3 T27 |
| `secrets.Detect` | go/ts | P0-1 T14, P0-4 T9 | P0-1 T15 |
| `secrets.Detection` | go/ts | P0-1 T14, P0-1 T15 | P0-1 T15, P0-3 T22, P0-3 T23, P0-3 T27 |
| `secrets.Key` | go/ts | P0-1 T12 | P0-3 T27 |
| `secrets.Migrate` | go/ts | P0-1 T14 | P0-3 T27 |
| `secrets.Mode` | go/ts | P0-1 T14 | P0-3 T27 |
| `secrets.NewKeyring` | go/ts | P0-1 T12 | P0-3 T23, P0-3 T27 |
| `secrets.NewNone` | go/ts | P0-1 T12 | P0-3 T22, P0-3 T27 |
| `secrets.OpenEncFile` | go/ts | P0-1 T13 | P0-3 T27 |
| `secrets.ProbeKeyringWithin` | go/ts | P0-1 T12 | P0-1 T15, P0-3 T23 |
| `secrets.ProbeTimeout` | go/ts | P0-1 T12 | P0-1 T15, P0-3 T23 |
| `secrets.Store` | go/ts | P0-1 T12, P0-1 T14 | P0-3 T27, P0-3 T6, P0-3 hdr |
| `SecretStorageInfo` | go/ts | P0-3 T27, P0-3 hdr, P0-4 T2 | P0-4 T14 |
| `section` | go/ts | P0-3 T12, P0-3 hdr | P0-4 T11 |
| `seedByName` | go/ts | P0-4 T2 | P0-5 hdr |
| `seedDataDir` | go/ts | P0-1 T8 | P0-1 T9 |
| `segment` | go/ts | P0-2 T15, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `ServeWithContext` | go/ts | P0-3 T9 | P0-3 T9 |
| `Service` | go/ts | P0-2 T4, P0-3 T14, P0-3 T22, P0-3 T31 | P0-2 T5, P0-3 T29 |
| `service.CandlesRequest` | go/ts | P0-3 T16, P0-3 hdr | P0-5 T2 |
| `service.Deps` | go/ts | P0-3 T14 | P0-3 T23 |
| `service.EventSources` | go/ts | P0-3 T21 | P0-3 T22 |
| `Service.IsTradingDay` | go/ts | P0-2 T4 | P0-2 T16, P0-3 T16 |
| `Service.MergeOverrides` | go/ts | P0-2 T4 | P0-3 T31 |
| `service.Navigate` | go/ts | P0-3 T21, P0-3 hdr | P0-3 T23 |
| `service.NewSecretSwitch` | go/ts | P0-3 T27 | P0-3 T23 |
| `Service.PhaseAt` | go/ts | P0-2 T4 | P0-2 T19, P0-3 T21 |
| `service.RefreshRequest` | go/ts | P0-3 T16, P0-3 hdr | P0-5 T2 |
| `service.SecretSwitch` | go/ts | P0-3 T27 | P0-3 hdr |
| `service.Service` | go/ts | P0-3 T14 | P0-3 T24 |
| `Service.Session` | go/ts | P0-2 T4 | P0-2 T10, P0-2 T11 |
| `Service.SetOverrides` | go/ts | P0-2 T4 | P0-3 T23 |
| `service.TopicWatchlists` | go/ts | P0-3 T15, P0-3 hdr | P0-3 T21 |
| `Service.Version` | go/ts | P0-2 T4 | P0-3 T30 |
| `servicetest.New` | go/ts | P0-3 T22 | P0-3 T33 |
| `session.dataPlan` | go/ts | P0-4 T7 | P0-5 hdr |
| `Session.SetRedirectPort` | go/ts | P0-3 T10, P0-3 T22, P0-3 hdr, P0-4 T14 | P0-4 T14 |
| `session.state` | go/ts | P0-3 T10, P0-3 T21, P0-3 hdr, P0-4 T14, P0-4 T7 | P0-5 hdr |
| `SessionAPI` | go/ts | P0-3 T14 | P0-3 T14 |
| `SessionDeps.Secrets` | go/ts | P0-1 T15, P0-3 T23, P0-3 T27, P0-3 T6 | P0-3 T27 |
| `SessionErrorRelay` | go/ts | P0-3 T6 | P0-3 T8 |
| `SessionStatus` | go/ts | P0-3 T14, P0-3 T27, P0-3 hdr, P0-4 T2 | P0-4 T7, P0-4 T9, P0-5 hdr |
| `SetAccessToken` | go/ts | P0-2 T9, P0-2 hdr, P0-3 T24, P0-3 T6 | P0-3 T6 |
| `SetActive` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `SetAutoReconnect` | go/ts | P0-3 T9 | P0-3 T9 |
| `setBackend` | go/ts | P0-5 T10, P0-5 T11, P0-5 T13, P0-5 T14, P0-5 T15, P0-5 T16, P0-5 T17, P0-5 T2, P0-5 hdr | P0-5 T10, P0-5 T13, P0-5 T14, P0-5 T15, P0-5 T16, P0-5 T17, P0-5 T6 |
| `SetCandles` | go/ts | P0-2 T8 | P0-2 T9 |
| `SetDate` | go/ts | P0-2 T8 | P0-2 T9 |
| `setFeed` | go/ts | P0-4 T2 | P0-5 hdr |
| `SetInstrumentsCSV` | go/ts | P0-2 T8 | P0-2 T15 |
| `SetReconnectMaxRetries` | go/ts | P0-2 T18, P0-3 T9 | P0-3 T9 |
| `SettingNumberField` | go/ts | P0-4 T14 | P0-4 T16 |
| `settings` | go/ts | P0-1 T8, P0-3 T17 | P0-3 hdr |
| `Settings` | go/ts | P0-1 T10, P0-1 T15, P0-3 T14, P0-3 T17, P0-3 T22, P0-3 T6, P0-3 hdr | P0-4 T2 |
| `settings.AppearanceTheme` | go/ts | P0-3 T17, P0-3 hdr | P0-3 hdr |
| `settings.Backend` | go/ts | P0-1 T10 | P0-3 T2 |
| `settings.Changed` | go/ts | P0-1 T10, P0-3 hdr | P0-3 T21 |
| `settings.ErrUnknownKey` | go/ts | P0-3 T2 | P0-3 T17, P0-3 hdr |
| `settings.Get` | go/ts | P0-1 T10 | P0-1 T15, P0-3 T15, P0-3 T17, P0-3 T2, P0-3 T30, P0-3 T6 |
| `settings.GetJSON` | go/ts | P0-3 T2 | P0-3 T17, P0-3 hdr |
| `Settings.GetUiState` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `settings.Key` | go/ts | P0-1 T10, P0-3 T2, P0-3 T28 | P0-3 T17, P0-3 T2 |
| `settings.Load` | go/ts | P0-1 T10, P0-3 T2 | P0-1 T15, P0-3 T14, P0-3 T17, P0-3 T2, P0-3 T22, P0-3 T24, P0-3 T6 |
| `settings.LogLevel` | go/ts | P0-1 T10, P0-3 hdr | P0-1 T15 |
| `settings.Names` | go/ts | P0-3 T2 | P0-3 T17, P0-3 hdr |
| `settings.OneOf` | go/ts | P0-1 T10 | P0-3 T17 |
| `settings.Register` | go/ts | P0-3 T2 | P0-3 T15, P0-3 T17, P0-3 hdr |
| `settings.Set` | go/ts | P0-1 T10 | P0-3 T15, P0-3 T2, P0-3 T24, P0-3 T6 |
| `Settings.Set` | go/ts | P0-3 T22, P0-3 hdr, P0-4 T14 | P0-4 T14 |
| `settings.SetJSON` | go/ts | P0-3 T2 | P0-3 T17, P0-3 hdr |
| `settings.ShortcutAction` | go/ts | P0-3 T28, P0-3 hdr | P0-3 hdr |
| `settings.Topic` | go/ts | P0-1 T10, P0-3 hdr | P0-1 T15, P0-3 T21 |
| `settings.ValidationError` | go/ts | P0-1 T10 | P0-3 T17, P0-3 T2 |
| `SettingSection` | go/ts | P0-4 T14 | P0-4 T15, P0-4 T16 |
| `SetUiState` | go/ts | P0-3 T22, P0-3 T28, P0-3 hdr | P0-3 T22, P0-5 hdr |
| `setUiState` | go/ts | P0-4 T2 | P0-5 T10, P0-5 hdr |
| `SHA256SUMS` | go/ts | P0-1 T16 | P0-5 T19, P0-5 hdr |
| `shellChildren` | go/ts | P0-4 T1, P0-4 T9 | P0-5 T11, P0-5 hdr |
| `ShellLayout` | go/ts | P0-4 T1, P0-4 T2, P0-4 T6, P0-4 T8 | P0-4 T2 |
| `SHIPPED_DEFAULT_TIMEFRAME` | go/ts | P0-5 T1 | P0-5 T11 |
| `ShortcutAction` | go/ts | P0-3 T28, P0-3 hdr, P0-4 T2 | P0-4 T15, P0-4 T6 |
| `shortcuts` | go/ts | P0-4 T14, P0-4 T2 | P0-3 T28, P0-5 T11, P0-5 hdr |
| `ShortcutScope` | go/ts | P0-4 T2 | P0-4 T6 |
| `showOnDashboard` | go/ts | P0-5 T10 | P0-5 T12, P0-5 T16 |
| `side` | go/ts | P0-3 T12, P0-3 hdr | P0-4 T11 |
| `Sidebar` | go/ts | P0-4 T1, P0-4 T3 | P0-4 T2 |
| `Signed` | go/ts | P0-4 T10 | P0-4 T11 |
| `simulateBrowserLogin` | go/ts | P0-4 T2 | P0-4 T19 |
| `SingleInstanceLock` | go/ts | P0-1 T15, P0-1 T6, P0-3 T26 | P0-3 T22 |
| `Skeleton` | go/ts | P0-4 T3 | P0-4 T12, P0-4 T13, P0-4 T5, P0-5 T10, P0-5 hdr |
| `slots.tsx` | go/ts | P0-4 T12 | P0-5 T10, P0-5 T16, P0-5 hdr |
| `Snapshot` | go/ts | P0-1 T10, P0-2 T6, P0-3 T29 | P0-2 T14 |
| `snapshot` | go/ts | P0-1 T3, P0-3 T14, P0-3 T29, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `source` | go/ts | P0-1 T11, P0-2 T1, P0-3 T12, P0-3 T31, P0-3 hdr, P0-4 T11 | P0-4 T11 |
| `SourceTicks` | go/ts | P0-2 T1 | P0-2 T21 |
| `spec` | go/ts | P0-5 T5 | P0-5 T4 |
| `Split` | go/ts | P0-2 T10 | P0-2 T12 |
| `sqlite.Open` | go/ts | P0-2 T11, P0-2 hdr | P0-2 T14, P0-2 T19, P0-3 T23 |
| `src` | go/ts | P0-2 T10 | P0-5 T10, P0-5 T11, P0-5 T16, P0-5 T3, P0-5 hdr |
| `Stale` | go/ts | P0-2 T4, P0-2 hdr, P0-3 T12, P0-3 T31 | P0-3 T31 |
| `Start` | go/ts | P0-1 T15, P0-2 T10, P0-2 T14, P0-2 T17, P0-2 T3, P0-2 hdr, P0-3 T21, P0-3 T23, P0-3 T24, P0-3 T26, P0-3 T31, P0-3 T6 | P0-3 T23, P0-3 hdr |
| `startEvents` | go/ts | P0-3 T21 | P0-3 T29 |
| `startLoopback` | go/ts | P0-3 T4 | P0-3 T25 |
| `state` | go/ts | P0-1 T5, P0-2 T14, P0-2 T17, P0-2 T20, P0-3 T1, P0-3 T14, P0-3 T21, P0-3 T25, P0-3 T27, P0-3 T3, P0-3 T7, P0-3 hdr | P0-4 T19, P0-4 T7, P0-4 T9, P0-5 hdr |
| `Stats` | go/ts | P0-1 T3, P0-2 T10, P0-2 T11, P0-3 T22, P0-3 T30, P0-3 hdr | P0-2 T13 |
| `statusFrom` | go/ts | P0-3 T14, P0-3 T27 | P0-3 T27 |
| `Stop` | go/ts | P0-2 T1, P0-2 T17, P0-2 T18, P0-2 T2, P0-2 hdr, P0-3 T23, P0-3 T24, P0-3 T6 | P0-2 T20, P0-3 T9, P0-3 hdr |
| `Store` | go/ts | P0-1 T12, P0-1 T13, P0-1 T14, P0-2 T10, P0-2 T11, P0-2 T13, P0-2 T14, P0-2 T19, P0-2 hdr | P0-1 T13, P0-1 T14, P0-2 T12 |
| `store` | go/ts | P0-2 T14, P0-2 T5 | P0-2 hdr, P0-3 hdr, P0-5 hdr |
| `store.AcquireLock` | go/ts | P0-1 T15, P0-1 T6, P0-3 T26 | P0-1 T15, P0-3 T23, P0-3 T26 |
| `store.BackupOptions` | go/ts | P0-1 T8 | P0-3 T30 |
| `store.CalendarOverrideRow` | go/ts | P0-2 T5, P0-2 hdr | P0-3 T31 |
| `store.DB.W` | go/ts | P0-1 T7, P0-3 T22 | P0-3 T5 |
| `store.EmbeddedMigrations` | go/ts | P0-1 T7 | P0-3 T30, P0-3 T5 |
| `store.EnvDataDir` | go/ts | P0-1 T6 | P0-3 T23 |
| `store.ErrBadArchive` | go/ts | P0-1 T9 | P0-3 T30 |
| `store.ErrDeploymentsActive` | go/ts | P0-1 T9 | P0-3 T30 |
| `store.ErrLocked` | go/ts | P0-1 T15, P0-1 T6, P0-3 T23, P0-3 T26 | P0-1 T15, P0-3 T23 |
| `store.ErrNotConfirmed` | go/ts | P0-1 T9 | P0-3 T30 |
| `store.InspectBackup` | go/ts | P0-1 T9, P0-3 T30 | P0-3 T30 |
| `store.InstrumentRow` | go/ts | P0-2 T15 | P0-2 T16 |
| `store.Layout` | go/ts | P0-1 T15, P0-1 T6 | P0-3 T30, P0-3 T32 |
| `store.Manifest` | go/ts | P0-1 T8, P0-1 T9 | P0-3 T30 |
| `store.NewLayout` | go/ts | P0-1 T6 | P0-1 T15, P0-3 T22, P0-3 T30 |
| `store.NewMigrator` | go/ts | P0-1 T7 | P0-2 T11, P0-2 hdr |
| `store.NewShortcutsRepo` | go/ts | P0-3 T28 | P0-3 T23 |
| `store.NewWindowStateRepo` | go/ts | P0-3 T28 | P0-3 T23 |
| `store.Open` | go/ts | P0-1 T7 | P0-1 T10, P0-1 T15, P0-2 T15, P0-2 T16, P0-2 T5, P0-3 T30, P0-3 T5 |
| `store.Options` | go/ts | P0-1 T7 | P0-1 T10, P0-2 T15, P0-2 T16 |
| `Store.Read` | go/ts | P0-2 T11 | P0-3 T14 |
| `store.ResolveDataDir` | go/ts | P0-1 T6 | P0-1 T15, P0-3 T23 |
| `store.Restore` | go/ts | P0-1 T9, P0-3 T30 | P0-3 T30 |
| `store.RestoreConfirmPhrase` | go/ts | P0-1 T9 | P0-3 T30 |
| `store.RestoreRequest` | go/ts | P0-1 T9 | P0-3 T30 |
| `store.RestoreResult` | go/ts | P0-1 T9 | P0-3 T30 |
| `store.SchemaTooNewError` | go/ts | P0-1 T7 | P0-1 T15 |
| `store.SessionsRepo` | go/ts | P0-3 T5 | P0-3 T6 |
| `store.ShortcutsRepo` | go/ts | P0-3 T28 | P0-3 hdr |
| `store.SupportedSchema` | go/ts | P0-1 T7 | P0-3 T30, P0-3 T5 |
| `Store.Upsert` | go/ts | P0-2 T11 | P0-2 T19 |
| `Store.UpsertCovered` | go/ts | P0-2 T11 | P0-2 T13 |
| `store.WatchlistsRepo` | go/ts | P0-3 T13 | P0-3 T14 |
| `store.WindowStateRepo` | go/ts | P0-3 T28 | P0-3 T29, P0-3 T30, P0-3 hdr |
| `storetest.Open` | go/ts | P0-3 T5 | P0-3 T13, P0-3 T14, P0-3 T18, P0-3 T22, P0-3 T24, P0-3 T28, P0-3 hdr |
| `strategies` | go/ts | P0-1 T8, P0-3 hdr | P0-4 T16 |
| `StudySpec` | go/ts | P0-5 T2 | P0-5 T3 |
| `Subscribe` | go/ts | P0-1 T3, P0-2 T17, P0-2 T18, P0-2 hdr, P0-3 T14, P0-3 T19, P0-3 T24, P0-3 T6 | P0-3 hdr |
| `subscribe` | go/ts | P0-5 T6 | P0-5 T6, P0-5 hdr |
| `Success` | go/ts | P0-2 T6 | P0-2 T9 |
| `summarizeHoldings` | go/ts | P0-4 T10 | P0-4 T12 |
| `SupportedSchema` | go/ts | P0-1 T7, P0-3 T30 | P0-1 T8, P0-1 T9 |
| `suspendSettingsWrites` | go/ts | P0-4 T16, P0-4 T2 | P0-4 T16 |
| `ta` | go/ts | P0-1 T16 | P0-5 T19, P0-5 hdr |
| `tabTo` | go/ts | P0-4 T19 | P0-5 hdr |
| `tag` | go/ts | P0-1 T11, P0-2 T15, P0-3 T12, P0-3 T16, P0-3 hdr, P0-5 T1 | P0-5 hdr |
| `tags` | go/ts | P0-3 T16 | P0-5 hdr |
| `test` | go/ts | P0-1 T16, P0-4 T11, P0-4 T19, P0-5 T18, P0-5 T3 | P0-5 hdr |
| `TextField` | go/ts | P0-4 T3 | P0-4 T13, P0-4 T9 |
| `ThemeName` | go/ts | P0-4 T1 | P0-5 T3, P0-5 hdr |
| `ThemeProvider` | go/ts | P0-4 T1 | P0-4 T2, P0-5 T10, P0-5 hdr |
| `THIRD_PARTY_NOTICES.md` | go/ts | P0-3 T33 | P0-3 T33, P0-3 hdr |
| `tick` | go/ts | P0-2 T19, P0-5 T6 | P0-5 T10, P0-5 T12, P0-5 T18, P0-5 T7, P0-5 T8 |
| `tick.batch` | go/ts | P0-2 T17, P0-3 T29, P0-3 hdr, P0-4 T10 | P0-5 T2, P0-5 hdr |
| `TickDTO` | go/ts | P0-3 T16, P0-3 hdr, P0-4 T2 | P0-3 T21, P0-4 T10, P0-5 T2, P0-5 T6, P0-5 hdr |
| `TickDTOFrom` | go/ts | P0-3 T16 | P0-3 T21 |
| `ticker` | go/ts | P0-3 T21, P0-3 T6, P0-4 T1 | P0-3 T9, P0-5 hdr |
| `ticker.Conn` | go/ts | P0-2 T17 | P0-3 T6 |
| `ticker.ConsumerChart` | go/ts | P0-2 T17, P0-3 T29 | P0-3 T16, P0-3 T29 |
| `ticker.ConsumerWidget` | go/ts | P0-2 T17 | P0-3 T19 |
| `ticker.Dialer` | go/ts | P0-2 T17 | P0-3 T6 |
| `ticker.Events` | go/ts | P0-2 T17 | P0-3 T6 |
| `ticker.FeedState` | go/ts | P0-2 T17, P0-2 hdr, P0-3 hdr | P0-3 T21, P0-3 T22 |
| `ticker.KiteDialer` | go/ts | P0-2 T18 | P0-3 T6 |
| `ticker.ModeQuote` | go/ts | P0-2 T17 | P0-3 T16, P0-3 T19, P0-3 T29 |
| `ticker.New` | go/ts | P0-2 T17, P0-2 hdr | P0-2 T20, P0-2 T21, P0-3 T6 |
| `ticker.Options` | go/ts | P0-2 T17, P0-2 T20, P0-2 hdr | P0-3 T23, P0-3 T6 |
| `ticker.Tick` | go/ts | P0-2 T17, P0-2 hdr | P0-2 T19, P0-3 T16, P0-3 T19, P0-3 T21 |
| `ticker.TickObserver` | go/ts | P0-2 T17 | P0-3 T23 |
| `ticker.TopicFeed` | go/ts | P0-2 T17, P0-3 hdr | P0-3 T21 |
| `ticker.TopicTicks` | go/ts | P0-2 T17, P0-3 hdr | P0-3 T19, P0-3 T21 |
| `TickerURL` | go/ts | P0-2 T8 | P0-2 T18 |
| `tickFixture` | go/ts | P0-4 T2 | P0-4 T11, P0-4 T12 |
| `TickObserver` | go/ts | P0-2 T17 | P0-2 T17 |
| `tickRupees` | go/ts | P0-5 T1, P0-5 hdr | P0-5 T10, P0-5 T5, P0-5 T8 |
| `Ticks` | go/ts | P0-3 T6 | P0-3 hdr |
| `tickSizePaise` | go/ts | P0-2 T15, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `time.Time` | go/ts | P0-1 T11, P0-1 T15, P0-1 T2, P0-1 T5, P0-1 T7, P0-1 T8, P0-1 T9, P0-3 hdr, P0-4 T2 | P0-3 T22 |
| `timeframeById` | go/ts | P0-5 T1 | P0-5 T5 |
| `TimeframeId` | go/ts | P0-5 T1, P0-5 hdr | P0-5 T8 |
| `TIMEFRAMES` | go/ts | P0-4 T2, P0-5 T1, P0-5 hdr | P0-5 T8 |
| `timeMs` | go/ts | P0-3 T12, P0-3 hdr, P0-4 T2 | P0-4 T16, P0-4 T7 |
| `Timer` | go/ts | P0-2 T1 | P0-2 T2 |
| `TimeRange` | go/ts | P0-5 T2 | P0-5 T3 |
| `TOAST_DURATION_MS` | go/ts | P0-4 T4 | P0-5 T14, P0-5 hdr |
| `Toggle` | go/ts | P0-4 T3 | P0-4 T16 |
| `tokensOf` | go/ts | P0-5 T13 | P0-5 T14, P0-5 T15, P0-5 T17 |
| `toMs` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `Tooltip` | go/ts | P0-2 T4, P0-3 T21, P0-4 T3 | P0-4 T7, P0-4 T8 |
| `TopBar` | go/ts | P0-4 T1 | P0-5 T12 |
| `TopicHoldings` | go/ts | P0-3 T19, P0-3 hdr | P0-3 T21 |
| `TopicJobProgress` | go/ts | P0-3 T21, P0-3 hdr | P0-3 T30 |
| `TopicPolledTicks` | go/ts | P0-3 T21, P0-3 T29 | P0-3 T29 |
| `TopicPositions` | go/ts | P0-3 T19, P0-3 hdr | P0-3 T21 |
| `toRow` | go/ts | P0-2 T15 | P0-2 T16 |
| `Track` | go/ts | P0-2 T19, P0-2 hdr, P0-3 T16 | P0-2 T20 |
| `tradeloom` | go/ts | P0-3 T12 | P0-3 hdr |
| `tradingDays` | go/ts | P0-3 T16, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `tradingsymbol` | go/ts | P0-2 T15, P0-3 T16, P0-3 hdr | P0-5 hdr |
| `tsMs` | go/ts | P0-3 T16, P0-3 T29, P0-3 hdr | P0-5 hdr |
| `UILastRoute` | go/ts | P0-3 T17, P0-3 hdr | P0-3 hdr |
| `Unsubscribe` | go/ts | P0-1 T3, P0-2 T17, P0-2 T18, P0-3 T14, P0-3 T19, P0-3 T24, P0-3 T6 | P0-3 T14, P0-3 hdr |
| `UpdateOutcome` | go/ts | P0-5 T2 | P0-5 T4 |
| `Usage` | go/ts | P0-2 T17, P0-2 T6, P0-2 hdr, P0-3 T6, P0-3 hdr | P0-2 T20, P0-3 hdr |
| `useAppStore` | go/ts | P0-4 T2 | P0-4 T14, P0-4 T8 |
| `useBackend` | go/ts | P0-4 T2 | P0-5 T6, P0-5 hdr |
| `useInstruments` | go/ts | P0-5 T7, P0-5 hdr | P0-5 T15, P0-5 T17, P0-5 T9 |
| `useLtp` | go/ts | P0-4 T2 | P0-5 hdr |
| `useNow` | go/ts | P0-4 T7 | P0-4 T13 |
| `usePolled` | go/ts | P0-5 T6, P0-5 hdr | P0-5 T14 |
| `usePortfolioResource` | go/ts | P0-4 T10 | P0-4 T11, P0-4 T12 |
| `useProfile` | go/ts | P0-4 T8 | P0-4 T13 |
| `useQuote` | go/ts | P0-5 T6, P0-5 hdr | P0-5 T12, P0-5 T7 |
| `userId` | go/ts | P0-3 T1, P0-3 T11, P0-3 T14, P0-3 T27, P0-3 hdr | P0-4 T8 |
| `useSessionStore` | go/ts | P0-4 T2 | P0-4 T14, P0-4 T7, P0-4 T8, P0-4 T9, P0-5 T10, P0-5 hdr |
| `useSetting` | go/ts | P0-4 T14, P0-4 T2 | P0-4 T14, P0-4 T15, P0-4 T16, P0-4 T6, P0-4 T8, P0-4 T9, P0-5 T10, P0-5 hdr |
| `useSettingsStore` | go/ts | P0-4 T2 | P0-4 T17, P0-4 T6, P0-4 T9, P0-5 T10, P0-5 hdr |
| `useShortcut` | go/ts | P0-4 T6, P0-5 T11, P0-5 T14 | P0-5 T11, P0-5 T14, P0-5 hdr |
| `useShortcutStore` | go/ts | P0-4 T6 | P0-4 T15, P0-4 T18 |
| `useStatusStore` | go/ts | P0-4 T2 | P0-4 T11, P0-4 T7, P0-5 T6, P0-5 hdr |
| `useSubscription` | go/ts | P0-5 T12, P0-5 T14, P0-5 T17, P0-5 T6, P0-5 T8, P0-5 hdr | P0-5 T12, P0-5 T14, P0-5 T17, P0-5 T8 |
| `useTheme` | go/ts | P0-4 T1, P0-5 T3 | P0-4 T14, P0-4 T8, P0-5 T10, P0-5 hdr |
| `useTicksStore` | go/ts | P0-4 T2, P0-5 hdr | P0-4 T10, P0-4 T11, P0-4 T12, P0-5 T12, P0-5 T6, P0-5 T7, P0-5 T8, P0-5 hdr |
| `useToastStore` | go/ts | P0-4 T4 | P0-5 T14, P0-5 T15, P0-5 T17, P0-5 hdr |
| `useWatchlists` | go/ts | P0-5 T13, P0-5 hdr | P0-5 T14, P0-5 T15, P0-5 T16, P0-5 T17 |
| `v` | go/ts | P0-2 T10, P0-3 T16, P0-3 hdr, P0-4 T1 | P0-5 hdr |
| `Version` | go/ts | P0-1 T1, P0-1 T15, P0-2 T3, P0-2 T4, P0-2 hdr, P0-3 T22, P0-3 T26, P0-3 T31, P0-3 T33 | P0-3 T31 |
| `visibleNavEntries` | go/ts | P0-4 T1 | P0-4 T6 |
| `volume` | go/ts | P0-2 T17, P0-3 T16, P0-3 hdr | P0-5 T4, P0-5 hdr |
| `WaitStopped` | go/ts | P0-2 T14, P0-2 hdr | P0-3 T23 |
| `Watch` | go/ts | P0-3 T16, P0-3 T21, P0-3 T22, P0-3 hdr, P0-5 T2, P0-5 T6, P0-5 hdr | P0-5 hdr |
| `watchlist` | go/ts | P0-3 T29, P0-5 T14 | P0-5 hdr |
| `Watchlist` | go/ts | P0-3 T15, P0-3 hdr, P0-5 T2 | P0-5 T13, P0-5 T14, P0-5 hdr |
| `watchlist.active` | go/ts | P0-3 hdr | P0-5 hdr |
| `watchlist.moveDown` | go/ts | P0-3 hdr | P0-5 hdr |
| `watchlist.moveUp` | go/ts | P0-3 hdr | P0-5 hdr |
| `watchlist_items` | go/ts | P0-1 T8, P0-3 T13 | P0-3 T13 |
| `WatchlistItem` | go/ts | P0-3 T15, P0-3 hdr, P0-5 T14, P0-5 T2 | P0-5 T13, P0-5 T14, P0-5 hdr |
| `watchlistItemOf` | go/ts | P0-5 T2 | P0-5 T13, P0-5 T14, P0-5 T15, P0-5 T16, P0-5 T17 |
| `watchlists` | go/ts | P0-1 T8, P0-3 T5, P0-3 hdr, P0-5 hdr | P0-3 T13, P0-4 T16, P0-5 hdr |
| `Watchlists` | go/ts | P0-3 T14, P0-3 T15, P0-3 T22, P0-3 hdr | P0-5 T2 |
| `Watchlists.Add` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Create` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.List` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Remove` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Rename` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `Watchlists.Reorder` | go/ts | P0-3 T22, P0-3 hdr | P0-5 hdr |
| `watchlistsApi` | go/ts | P0-5 T2, P0-5 hdr | P0-5 T13 |
| `WatchlistsState` | go/ts | P0-3 T15, P0-3 hdr, P0-5 T2 | P0-5 hdr |
| `watchQuotes` | go/ts | P0-5 T2, P0-5 hdr | P0-5 T6 |
| `WatchResult` | go/ts | P0-3 T29, P0-3 hdr, P0-5 T2 | P0-5 T6, P0-5 hdr |
| `widget` | go/ts | P0-3 T29 | P0-5 T12, P0-5 T17, P0-5 hdr |
| `Window` | go/ts | P0-2 T10 | P0-2 T12 |
| `window.__tradeloomMock` | go/ts | P0-4 T2 | P0-4 T19, P0-5 hdr |
| `window_state` | go/ts | P0-3 T28, P0-3 T29, P0-3 T30, P0-3 hdr | P0-3 T28 |
| `WindowShow` | go/ts | P0-3 T22 | P0-3 T22 |
| `WindowStatus` | go/ts | P0-2 T10 | P0-2 T12 |
| `WindowUnminimise` | go/ts | P0-3 T22 | P0-3 T22 |
| `WithPriority` | go/ts | P0-2 T6 | P0-2 T9 |
| `wrap` | go/ts | P0-2 T7 | P0-2 T9, P0-5 T2 |
| `writeSetting` | go/ts | P0-4 T2 | P0-4 T16, P0-4 T17, P0-4 T6, P0-5 hdr |
| `appearance.follow_system` | key | P0-3 T17, P0-3 hdr | P0-4 T14 |
| `appearance.sidebar_collapsed` | key | P0-3 T17, P0-3 hdr | P0-4 T14 |
| `appearance.theme` | key | P0-3 T17, P0-3 hdr | P0-4 T14 |
| `backup.include_candles` | key | P0-1 T10, P0-3 hdr | P0-4 T16 |
| `backup.schedule.enabled` | key | P0-1 T10, P0-3 hdr | P0-4 T16 |
| `backup.schedule.folder` | key | P0-1 T10, P0-3 hdr | P0-4 T16 |
| `backup.schedule.keep` | key | P0-1 T10, P0-3 hdr | P0-4 T16 |
| `builder.redo` | key | P0-3 T28 | — |
| `builder.undo` | key | P0-3 T28 | — |
| `calendar.stale_toast_day` | key | P0-3 T17, P0-3 hdr | — |
| `chart.dashboard` | key | P0-3 T28 | P0-5 hdr |
| `chart.default_timeframe` | key | P0-3 T17, P0-3 hdr | P0-4 T14, P0-5 hdr |
| `chart.fullView` | key | P0-3 T28 | — |
| `chart.trendLine` | key | P0-3 T28 | — |
| `help.open` | key | P0-3 T28, P0-3 hdr | — |
| `kill.switch` | key | P0-3 T28, P0-3 hdr | — |
| `kite.api_key` | key | P0-3 T2, P0-3 hdr | — |
| `kite.api_secret` | key | P0-1 T10 | P0-3 hdr |
| `kite.redirect_port` | key | P0-3 T2, P0-3 hdr | P0-4 T14 |
| `log.level` | key | P0-1 T10, P0-3 hdr | P0-4 T15 |
| `order.new` | key | P0-3 T28, P0-3 hdr | — |
| `search.open` | key | P0-3 T28, P0-3 hdr | — |
| `service.SettingValue` | key | P0-3 hdr | — |
| `settings.ShortcutAction` | key | P0-3 hdr | — |
| `store.migrated` | key | P0-1 T4 | — |
| `toasts.focus` | key | P0-3 T28, P0-3 hdr | — |
| `ui.last_route` | key | P0-3 T17, P0-3 hdr | — |
| `watchlist.active` | key | P0-3 T15, P0-3 hdr | P0-5 hdr |
| `watchlist.moveDown` | key | P0-3 T28 | — |
| `watchlist.moveUp` | key | P0-3 T28 | — |
| `window.geometry` | key | P0-3 T28 | — |
