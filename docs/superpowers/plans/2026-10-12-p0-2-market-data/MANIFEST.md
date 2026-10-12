# P0-2 Time & Market Data Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Tradeloom's time and market-data layer — app clock, market calendar, read-only Kite client with the shared rate limiter and a mock Kite, SQLite candle store with coverage/checksums/resampling/purge, resumable backfill, instrument master with search and daily refresh, ticker subscription manager and tick-to-candle builder — and run spikes S0-2, S0-6 and S0-7, so that P0-3 can expose session, chart and watchlist data and P0-5 can draw it.

**Architecture:** Two pure leaf packages own all time: `internal/clock` (system time + measured exchange offset, timers) and `internal/calendar` (trading days, phases, anchors). `internal/kite` is the only importer of gokiteconnect; every HTTP call goes through `internal/kite/ratelimit` (sliding-window classes, priority queues, per-class 429 back-off, global pause). Candles live in `candles/candles.db` behind the `candles.Store` interface (`internal/candles/sqlite`, integer paise, one `WITHOUT ROWID` table per Kite interval); `internal/candles/backfill` fills it in Kite-sized persisted windows and `internal/candles/builder` builds live candles from `internal/kite/ticker` ticks. `internal/instruments` keeps the scoped instrument dump in `app.db` (through `internal/store`) and in an in-memory searchable `Master`. Producers publish typed events on `internal/bus`.

**Tech Stack:** Go 1.25 (see the modernc note below), `github.com/zerodha/gokiteconnect/v4` v4.4.3 (and its `github.com/gorilla/websocket` v1.4.2), `modernc.org/sqlite` (driver name `sqlite`), `github.com/google/uuid` v1.6.0, stdlib `log/slog`, `time/tzdata`, `net/http/httptest`.

**Spec:** `docs/requirements/REQUIREMENTS.md` v2.2 — FR-DATA-001, FR-DATA-002, FR-DATA-003, FR-DATA-004, FR-WATCH-008, FR-WATCH-013, FR-WATCH-014 (backend), C-05…C-09, NFR-07/08/09/12/15; `docs/architecture/ARCHITECTURE.md` §2 (layout, rules), §3.4 (bus topics), §3.6 (clock/timers), §4.3 (candle store), §5 (Kite integration), §6.5 (live path uses builder candles), §11 (testing), §12 rows 1d/2b/2c/3b/3c/4a, §12.1 S0-2/S0-6/S0-7; `docs/research/areas/07-platform-kite-wails.md` §A2–A6; `docs/superpowers/plans/2026-10-12-p0-index.md` (shared contract).

## Global Constraints

- Every build and test command passes `-tags webkit2_41,noorders` (G-0) and runs with `GOWORK=off`, as in CI.
- Module path `tradeloom`, Go 1.25, `modernc.org/sqlite` **v1.59.0** (all owned by P0-1's `go.mod`). ARCHITECTURE §4.1 names v1.60.1, but v1.60.1 and v1.60.0 declare `go 1.26.0` (checked on proxy.golang.org and in the downloaded `go.mod`; `go get` with Go 1.25.5 fails "requires go >= 1.26.0"); v1.59.0 declares `go 1.25.0`, which is what P0-1 pins. This plan uses only `database/sql`, the driver name `sqlite`, the DSN keys `_pragma` and `_txlock`, FTS5 and the `dbstat` table, all present in v1.59.0 (every task was verified on it) and v1.60.1.
- `github.com/zerodha/gokiteconnect/v4` is pinned at **v4.4.3**. Only packages under `internal/kite/...` import it, in tests too (ARCHITECTURE §2.4 rule 3). Other packages use the aliases in `internal/kite/types.go` (`kite.HistoricalData`, `kite.Instrument`, `kite.Time`, …).
- Only `internal/store` and `internal/candles/sqlite` import `database/sql` or `modernc.org/sqlite` (rule 4). `internal/clock` and `internal/calendar` import no I/O package (rule 1).
- No package other than `internal/clock` calls `time.Now`, `Since`, `Until`, `Sleep`, `After`, `AfterFunc`, `Tick`, `NewTimer` or `NewTicker` in non-test code (rule 8, P0-1 archtest); all scheduling goes through `clock.Clock`, raw wall time through `clock.SystemNow`. No bare `go` statement outside `internal/logging` and `internal/bus` in non-test code: goroutines start with `logging.Go(name, fn)`.
- Session-time string literals (`"09:15"`, `"15:30"`, `"15:20"`, `"08:30"`, `"16:00"`) appear only in `internal/calendar` and in test files (ARCHITECTURE §10.5). Regular-day phases: pre-open 09:00–09:15 (cash only), continuous 09:15–15:30, closing 15:30–15:40 (cash only), post-closing 15:40–16:00 (cash only); derivatives (NFO/BFO) have only the continuous phase and follow their cash exchange's holidays and special sessions.
- All times are IST (`clock.IST`, loaded with embedded `time/tzdata`); every candle boundary is `anchor + k × interval` where the anchor is the calendar's continuous-phase start (09:15 on a regular day, the special session's start otherwise).
- App clock thresholds (FR-DATA-004): tick window 60 s (median), HTTP `Date` samples 5 (median), whole-second truncation corrected by +0.5 s, warn above 1 s, skewed above 5 s, cleared below 2 s or on acknowledge, timers re-armed when the offset moves more than 100 ms. Builder close = boundary + 1 s grace (FR-DEP-006).
- Rate limits per class (A07 §A3, FR-DATA-003): quote 1/s, historical 3/s, orders 8/s app default (Kite allows 10/s), other 10/s; each class admits at most `rate` grants in any 1 s + 20 ms window. Priorities: Interactive > Deployment > LiveScanner > Backtest > Recorder > Library, FIFO within a priority. HTTP 429: back-off on that class only, 1 s × 2ⁿ capped at 30 s. Network errors: idempotent GETs retried ≤ 3× with 0.5–1.5 s jitter; never for non-idempotent calls. Server errors: one retry after 2 s. `TokenException`: pause every class, notify, no retry.
- Historical windows (A07 §A4): minute 60 d; 3/5/10-minute 100 d; 15/30-minute 200 d; 60-minute 400 d; day 2000 d; halved on an input/window error. Kite's `to` is inclusive, so a half-open window `[From, To)` is sent as `to = To − 1 s`. From/To are sent as IST wall time.
- Ticker (A07 §A6, C-07): at most 3 connections × 3000 tokens; refuse (never evict) beyond 9000; reconnect delay 2 s doubling to 60 s with ±20 % jitter, attempt counter reset only after a connection stayed up 30 s, Offline after 50 consecutive failures; tick fan-out coalesced every 100 ms.
- Candle prices are stored as integer paise; the engine sees `float64` rupees. Candles are stored raw (unadjusted, NFR-12).
- Instrument scope (NFR-15): keep only `INDICES` (NSE/BSE), NSE/BSE cash `EQ`, `NFO-FUT`/`BFO-FUT` `FUT`; drop options, MCX, currency and expired contracts at import.
- Commit messages end with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Consumes (from P0-1)

Checked against `docs/superpowers/plans/2026-10-12-p0-1-platform-core/MANIFEST.md` (its "Produces" blocks) on 2026-10-12. These are the only P0-1 names this plan uses; verification ran against stubs with exactly these signatures.

```go
// P0-1 Task 2 — internal/clock/system.go (kept unchanged; P0-2 adds the rest of package clock)
func SystemNow() time.Time

// P0-1 Task 3 — internal/bus
type Bus struct{ /* … */ }
func New(log *slog.Logger) *Bus                                   // nil log uses slog.Default(); tests pass nil
type Topic[T any] struct{ /* … */ }
func NewTopic[T any](name string) Topic[T]                        // one payload type per topic name (a second type panics)
type SubOpts[T any] struct{ Name string; Buffer int; Critical bool; Coalesce func(old, new T) T } // P0-2 sets Buffer
type Sub[T any] struct{ /* … */ }
func (s *Sub[T]) C() <-chan T
func Publish[T any](b *Bus, t Topic[T], v T)                      // never blocks on a non-critical subscriber
func Subscribe[T any](b *Bus, t Topic[T], o SubOpts[T]) *Sub[T]

// P0-1 Task 5 — internal/logging
func Go(name string, fn func())                                    // the only sanctioned way to start a goroutine

// P0-1 Task 7 — internal/store
type Options struct{ AppVersion string; BackupDir string; Now func() time.Time; Migrations fs.FS; Log *slog.Logger } // Now, BackupDir required
type DB struct{ W *sql.DB; R *sql.DB /* … */ }                   // W: single read-write connection; R: read-only pool (query_only)
func Open(ctx context.Context, path string, o Options) (*DB, error) // migrates app.db; v1 has instruments, instrument_imports,
                                                                   //   instrument_fts, calendar_overrides exactly as ARCHITECTURE §4.2
func (db *DB) Close() error
type Migrator struct{ /* … */ }
func NewMigrator(fsys fs.FS) (*Migrator, error)                    // *.sql at the root of fsys, NNNN_name.sql contiguous from 0001;
                                                                   //   rejects PRAGMA/ATTACH/DETACH/BEGIN/COMMIT/ROLLBACK/VACUUM lines
func (m *Migrator) Latest() int
func (m *Migrator) Apply(ctx context.Context, db *sql.DB, from int) error // one transaction incl. PRAGMA user_version

// go.mod (P0-1 Task 1/2): module tradeloom, go 1.25.0, modernc.org/sqlite v1.59.0, gokiteconnect/v4 v4.4.3.
// internal/archtest (P0-1 Task 2): rules 1–8 + G-0; rule 8 bans time.Now/Since/Until/Sleep/After/AfterFunc/Tick/NewTimer/NewTicker
//   outside internal/clock in non-test code.
// internal/settings — not used by P0-2.
```

P0-2 adds two files to P0-1's `internal/store` package (`calendar_overrides.go`, `instruments.go`): rule 4 lets only `internal/store` touch `app.db` with SQL. They use only `DB.W` (writes, transactions) and `DB.R` (reads). The candle DB reuses `store.NewMigrator` + `Apply` with its own embedded migrations, as P0-1 Task 7 anticipates.

## Review Focus

The five inputs most likely to hurt a user that the FR text does not spell out, each pinned by a test in the owning task:

1. **Non-regular sessions** — a holiday, a half-day or an evening Muhurat special session: candles must anchor at the session's own start and close at its end, a Sunday session belongs to its own date's day candle and to the preceding Monday's week, derivatives follow the cash exchange. Tests: `TestSpecialSessionAnchorsCandlesAndSquareOff` (Task 4), `TestMuhuratCandlesBelongToTheirDate` (Task 10), `TestSessionAnchorsHourlyAndSpecialSession` (Task 19).
2. **Skewed or jumping system clock** — a laptop 3 s fast, then 7 s fast, then corrected: the estimate must converge to −3 s ± 0.5 s, skew must latch until < 2 s or acknowledged, and armed timers must move with the offset. Tests: `TestOffsetFromTicksWithSystemClock3sFast`, `TestSkewHysteresisAndAcknowledge`, `TestRealTimerFiresAndRearmsOnOffsetChange` (Task 2); live check `TestS07` (Task 21).
3. **Kite 429 or `TokenException` in the middle of a backfill** — the window must go back to the queue (not fail, not be marked done), every queue must pause until login, and quote/order traffic must not wait on a historical back-off. Tests: `TestRateLimitedHistoricalBacksOffThatClassOnly`, `TestTokenExceptionPausesAllQueuesAndNotifies` (Task 9), `TestTokenExceptionMidBackfillRequeuesWindow` (Task 14).
4. **Duplicate/overlapping backfill windows and the partial current candle** — two consumers asking for overlapping ranges must cause non-overlapping fetches; Kite's still-forming candle must be stored incomplete and stay "missing" so it is fetched again; a candle the builder only saw part of must never be stored; a late tick-built write must never overwrite a complete historical candle. Tests: `TestSameWindowFromTwoConsumersIsFetchedOnce`, `TestOverlappingRequestsProduceNonOverlappingWindows`, `TestFormingCandleStoredIncompleteAndNotCovered` (Task 14), `TestHistoricalReplacesTickBuiltButNotViceVersa` (Task 11), `TestPartialFirstCandleIsPublishedButNotStored` (Task 19).
5. **Ticker reconnect storm** — a flapping network must not hammer Kite: back-off doubles, short-lived connections do not reset it, all three connections jitter independently, and after the limit the feed goes Offline until the user retries; every reconnect resubscribes all tokens with their modes. Tests: `TestReconnectStormBacksOffAndGoesOffline`, `TestQuickReconnectsDoNotResetBackoff`, `TestReconnectResubscribesAllTokensWithModes` (Task 17), `TestManagerOverRealTickerAndMockServer` (Task 18).

## Verified library facts this plan depends on

Read in the module sources downloaded with `go mod download` (gokiteconnect v4.4.3; modernc.org/sqlite v1.59.0 and v1.60.1) on 2026-10-12:

- `kiteconnect.Client.GetHistoricalData(instrumentToken int, interval string, fromDate, toDate time.Time, continuous, OI bool) ([]HistoricalData, error)` formats `from`/`to` with `Format("2006-01-02 15:04:05")` in the value's own location (`market.go:265`) — the wrapper converts to IST first.
- `GetInstruments()` parses the body as CSV without checking the HTTP status (`market.go:290–307`); an expired token therefore surfaces as "Error parsing csv response". `internal/kite/transport.go` records the error envelope of a failed dump so `Classify` sees `TokenException`.
- API errors are `kiteconnect.Error{Code int; ErrorType, Message string; Data interface{}}` with `Code` = HTTP status (`http.go:159–167`); types `GeneralException, TokenException, PermissionError, UserException, TwoFAException, OrderException, InputException, DataException, NetworkException` (`errors.go:9–19`). Transport failures become `NetworkException` code 503; unreadable bodies become `DataException` code 504.
- No gokiteconnect call takes a `context.Context`; timeouts come from the `*http.Client` passed to `SetHTTPClient` (default 7 s, `connect.go:167`). The wrapper uses two clients (10 s, and 30 s for historical).
- `ticker.Ticker`: `New(apiKey, accessToken)`, `SetRootURL(url.URL)`, `SetAutoReconnect`, `SetReconnectMaxRetries`, `SetConnectTimeout`, `OnConnect/OnTick/OnOrderUpdate/OnError/OnClose/OnReconnect/OnNoReconnect`, `ServeWithContext(ctx)`, `Subscribe/Unsubscribe/SetMode/Resubscribe`, `Stop`; subscribe frames are `{"a":"subscribe","v":[…]}` and `{"a":"mode","v":["full",[…]]}`. Hazards: `Subscribe` writes to `t.Conn` (nil before the first connect, `ticker.go:510`); `Resubscribe` mutates the unsynchronised `subscribedTokens` map on the serve goroutine and prints the token list to stdout (`ticker.go:574`); with auto-reconnect off a failed dial dereferences a nil `Conn` (`ticker.go:320`); `SetReconnectMaxDelay` rejects values above 5 s (inverted check, `ticker.go:198`). With `SetReconnectMaxRetries(0)`, `ServeWithContext` returns after the first failure (`ticker.go:270`) — the Manager relies on this and Task 18 proves it against the real library.
- `models.Tick.Timestamp` (exchange timestamp) and `LastTradeTime` are whole Unix seconds and are set **only in full mode** (`ticker.go:700`, `ticker.go:739`); the clock reference token is therefore subscribed in full mode.
- modernc.org/sqlite v1.59.0 registers driver `"sqlite"` (`sqlite.go:50`), applies `_pragma=name(value)` DSN entries and `_txlock=immediate` (`sqlite.go:385`), and is built with `SQLITE_ENABLE_FTS5` and `SQLITE_ENABLE_DBSTAT_VTAB`. `PRAGMA page_size` only takes effect before the first table exists and before WAL, so `Open` sets it on a new file first.

---

## File Structure

```
assets/calendar/
  calendar.go                    embeds the shipped calendar CSV (package calendardata)
  nse-bse-2026.csv               2026 NSE/BSE trading holidays (FR-DATA-001 data file, versioned)
internal/clock/
  clock.go                       Clock/Timer interfaces, Offset, Source, IST, Sleep
  ist.go                         IST helpers, NextBoundary, BucketStart
  fake.go                        Fake clock for every package's tests
  estimator.go                   tick / HTTP-Date offset estimator (median, +0.5 s correction)
  real.go                        Real clock: offset, skew state, re-arming timers, clock.offset events
internal/calendar/
  date.go                        Date, HM (time of day)
  model.go                       Exchange, PhaseKind, DayType, PhaseSpec, Entry, Phase, Day
  regular.go                     the only session-time constants in the code base
  csv.go                         calendar CSV parser (all-or-nothing, "Row N: …" errors)
  service.go                     Service: days, phases, anchors, sessions, staleness, labels, version
  record.go                      Entry <-> calendar_overrides row codec
  infer.go                       inferred non-trading days (NIFTY 50 daily-candle rule)
internal/store/                  (P0-1 package; P0-2 adds two repositories)
  calendar_overrides.go          calendar_overrides CRUD
  instruments.go                 instruments + instrument_imports + FTS rebuild
internal/kite/
  types.go                       gokiteconnect type aliases, ReadClient, HistoricalPicker, order.update topic
  errors.go                      Kind taxonomy, Err, Classify, IsOffline
  transport.go                   RoundTripper: HTTP Date -> clock, failed-dump error capture
  client.go                      Client: ReadClient over limiter + retry policy
internal/kite/ratelimit/
  types.go                       Class, Priority, rates, priority context
  sched.go                       pure per-class admission (sliding window, heap, back-off)
  limiter.go                     Limiter: scheduler goroutines, pause/resume, limits.usage events
internal/kite/mock/
  packets.go                     Kite binary tick packet/frame encoder
  server.go                      httptest Kite: envelopes, scripted failures, Date, WebSocket ticker
  fake.go                        in-memory ReadClient + HistoricalPicker
internal/candles/
  interval.go                    Interval (Kite names + week/month/N-minute), tables, Base
  candle.go                      Candle, Source, Range, Coverage, paise conversion
  ranges.go                      Merge, Subtract, Split
  store.go                       Store interface (OQ-41 swap point), Stats, PurgeRule, RunsReferencedError
  jobs.go                        backfill Job/Window/JobProgress and the JobStore interface
  resample.go                    session-anchored intraday, daily, weekly, monthly resampling
internal/candles/sqlite/
  migrations/0001_init.sql       candles.db schema v1
  open.go                        Open (page size, pragmas, migration)
  store.go                       Read, Upsert(Covered), coverage, Missing, checksums
  admin.go                       Stats, budget, run refs, Purge, Clear
  resampled.go                   ReadResampled
  jobs.go                        JobStore implementation
internal/candles/backfill/
  windows.go                     Kite window sizes, plan, halve
  backfill.go                    Backfill: Submit/Wait/Ensure/DailyCheck, workers, progress
internal/instruments/
  instrument.go                  Instrument, Tag, ID, UnitsForLots, row codec
  filter.go                      NFR-15 filter, display names, weekly marker
  aliases.go                     index -> F&O name aliases for search
  master.go                      in-memory Master: lookups, futures
  search.go                      FR-WATCH-008 ranking
  refresher.go                   daily refresh after 08:30, status events, dump file
  testdata/dump.csv              sanitised fixture in Kite's CSV format
internal/kite/ticker/
  types.go                       Mode, Consumer, Tick, ErrBudget, FeedState, Usage, topics
  conn.go                        Conn/Events/Dialer abstraction
  manager.go                     Manager: registry, budget, assignment, reconnect, fan-out, clock feed
  kitedial.go                    KiteDialer over gokiteconnect's ticker
internal/candles/builder/
  builder.go                     tick -> candle builder, candle.closed / candle.forming
docs/architecture/spikes/
  S0-2-candle-store.md           generated from the S0-2 harness output
  S0-6-ticker.md                 generated from the S0-6 harness output
  S0-7-clock.md                  generated from the S0-7 harness output
```

Tests sit next to the code (`*_test.go`). Live-account harnesses carry build tag `kitelive` (never set in CI); the full-scale store benchmark carries build tag `s02`.

---


<!-- generated task table: begin (plan_split.py) -->
## Task files

Each implementer reads this manifest plus its one task file. Sizes are approximate (characters / 4).
"Consumes tasks" lists earlier tasks named in the task's Consumes lines. Dispatch in number order
unless Global Constraints or the P0 index state a different execution order.

| # | Task | File | ~tokens | Consumes tasks |
|---|---|---|---|---|
| 1 | App clock interface, IST helpers, boundary math, fake clock | [`tasks/01-app-clock-interface-ist-helpers-boundary.md`](tasks/01-app-clock-interface-ist-helpers-boundary.md) | 3.2k | — |
| 2 | Offset estimator, Real clock, skew state, re-arming timers | [`tasks/02-offset-estimator-real-clock-skew-state.md`](tasks/02-offset-estimator-real-clock-skew-state.md) | 3.7k | 1 |
| 3 | Calendar model, CSV parser and shipped 2026 calendar | [`tasks/03-calendar-model-csv-parser-and-shipped.md`](tasks/03-calendar-model-csv-parser-and-shipped.md) | 5.4k | 1 |
| 4 | Calendar service: days, phases, anchors, special sessions, staleness, labels | [`tasks/04-calendar-service-days-phases-anchors-special.md`](tasks/04-calendar-service-days-phases-anchors-special.md) | 4.2k | 1, 3 |
| 5 | Calendar inference and persisted overrides | [`tasks/05-calendar-inference-and-persisted-overrides.md`](tasks/05-calendar-inference-and-persisted-overrides.md) | 2.1k | 4 |
| 6 | Shared rate limiter: classes, priorities, sliding window, 429 back-off, pause | [`tasks/06-shared-rate-limiter-classes-priorities-sliding.md`](tasks/06-shared-rate-limiter-classes-priorities-sliding.md) | 5.2k | 1 |
| 7 | Kite type aliases, ReadClient interface and error taxonomy | [`tasks/07-kite-type-aliases-readclient-interface-and.md`](tasks/07-kite-type-aliases-readclient-interface-and.md) | 3.7k | 6 |
| 8 | Mock Kite: HTTP envelopes, scripted failures, binary ticks, in-memory fake | [`tasks/08-mock-kite-http-envelopes-scripted-failures.md`](tasks/08-mock-kite-http-envelopes-scripted-failures.md) | 6.6k | 1, 6, 7 |
| 9 | Kite read client: limiter, retry policy, Date header and dump-error capture | [`tasks/09-kite-read-client-limiter-retry-policy.md`](tasks/09-kite-read-client-limiter-retry-policy.md) | 5.7k | 1, 6, 7, 8 |
| 10 | Candle model, intervals, ranges and session-anchored resampling | [`tasks/10-candle-model-intervals-ranges-and-session.md`](tasks/10-candle-model-intervals-ranges-and-session.md) | 7.0k | 1, 3 |
| 11 | SQLite candle store: schema, upsert, coverage, checksums, resample, purge | [`tasks/11-sqlite-candle-store-schema-upsert-coverage.md`](tasks/11-sqlite-candle-store-schema-upsert-coverage.md) | 8.9k | 1, 4, 10 |
| 12 | SQLite backfill job store | [`tasks/12-sqlite-backfill-job-store.md`](tasks/12-sqlite-backfill-job-store.md) | 3.0k | 10, 11 |
| 13 | Spike S0-2: candle store benchmark and CI-scale AC4 test | [`tasks/13-spike-s0-2-candle-store-benchmark.md`](tasks/13-spike-s0-2-candle-store-benchmark.md) | 2.3k | 11 |
| 14 | Backfill service: windows, dedupe, resume, partial candles, progress | [`tasks/14-backfill-service-windows-dedupe-resume-partial.md`](tasks/14-backfill-service-windows-dedupe-resume-partial.md) | 8.0k | 1, 3, 5, 6, 7, 8 |
| 15 | Instrument master: scope filter, display names, tags, lookups and search | [`tasks/15-instrument-master-scope-filter-display-names.md`](tasks/15-instrument-master-scope-filter-display-names.md) | 8.8k | 1, 3, 5, 6, 7, 8 |
| 16 | Instrument refresher: daily download after 08:30, failure fallback, dump copy | [`tasks/16-instrument-refresher-daily-download-after-08.md`](tasks/16-instrument-refresher-daily-download-after-08.md) | 3.6k | 1, 4, 6, 7, 15 |
| 17 | Ticker manager: budget, consumers, modes, reconnect, fan-out, clock feed | [`tasks/17-ticker-manager-budget-consumers-modes-reconnect.md`](tasks/17-ticker-manager-budget-consumers-modes-reconnect.md) | 8.5k | 1, 7 |
| 18 | KiteDialer over gokiteconnect ticker, proven against the mock server | [`tasks/18-kitedialer-over-gokiteconnect-ticker-proven-against.md`](tasks/18-kitedialer-over-gokiteconnect-ticker-proven-against.md) | 1.5k | 8, 17 |
| 19 | Candle builder: ticks to session-anchored candles, partial vs closed | [`tasks/19-candle-builder-ticks-to-session-anchored.md`](tasks/19-candle-builder-ticks-to-session-anchored.md) | 4.2k | 1, 4, 10, 17 |
| 20 | Spike S0-6: ticker manager against a live account | [`tasks/20-spike-s0-6-ticker-manager-against.md`](tasks/20-spike-s0-6-ticker-manager-against.md) | 2.9k | — |
| 21 | Spike S0-7: app clock against a live account | [`tasks/21-spike-s0-7-app-clock-against.md`](tasks/21-spike-s0-7-app-clock-against.md) | 1.4k | 2, 17, 20 |
| 22 | Phase verification: full suite, rules, perf, coverage | [`tasks/22-phase-verification-full-suite-rules-perf.md`](tasks/22-phase-verification-full-suite-rules-perf.md) | 0.6k | — |

Coverage and self-review: [`APPENDIX.md`](APPENDIX.md).
<!-- generated task table: end -->