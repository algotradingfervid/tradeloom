# P0-3 Session & Service Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Daily Kite login (loopback + paste), token lifecycle to 06:00 IST, plan-capability detection with degraded mode, logout and profile, the transport-neutral `service` façade (session, chart data, watchlists, settings, portfolio read), the Wails `app` layer (bound API structs, event bridge, single instance) and read-only `orders.Sync`, all without a single order-write call.

**Architecture:** `internal/kite` gains a `Session` that owns credentials, the loopback listener, the token record (in `secrets.Store`), the 06:00 watcher and the capability probe. `internal/orders` gains a read-only `Sync` that keeps order book, positions and holdings in sync with Kite through polling and ticker order updates, plus pure builders for the rows and the FR-HOME-002 summary. `internal/service` wraps everything behind DTO-returning methods and turns bus topics into coalesced UI events. `internal/app` only binds those methods to Wails, emits the events and handles single-instance focus. `cmd/tradeloom` wires it all.

**Tech Stack:** Go 1.25; `github.com/zerodha/gokiteconnect/v4` v4.4.3 (session, read APIs, `ticker`); `github.com/wailsapp/wails/v2` v2.16.0; `github.com/godbus/dbus/v5` v5.2.3; `github.com/google/uuid` v1.6.0 (`NewV7`); `github.com/gorilla/websocket` (test only; the version gokiteconnect requires, v1.4.2); SQLite through `internal/store`; the single-instance flock is P0-1's (`github.com/gofrs/flock` v0.13.1 via `store.AcquireLock`).

**Spec:** `docs/architecture/ARCHITECTURE.md` (§2.2–2.5, §3.1–3.4, §3.7, §4.2, §5.1–5.3, §8.4, §9.1, §11, §12 rows 3a/4b/5d, §12.1 S0-1/S0-8); `docs/requirements/REQUIREMENTS.md` FR-ACCT-001/004/005/006/007, FR-ORD-001/002/003/014, FR-HOME-002, C-01…C-04, NFR-04/05/09; `docs/research/areas/07-platform-kite-wails.md` §A1/A2/A11; `docs/superpowers/plans/2026-10-12-p0-index.md`.

## Global Constraints

- Every `go build`/`go test`/`go vet` command in this plan runs with `-tags webkit2_41,noorders`; CI uses `GOWORK=off`; Go 1.25.
- **G-0 / `noorders`:** no code in this plan calls `PlaceOrder`, `ModifyOrder`, `CancelOrder`, `ExitOrder`, `ConvertPosition`, `PlaceGTT`, `ModifyGTT`, `DeleteGTT`, `CreateAlert`, `ModifyAlert`, `DeleteAlerts`, `InitiateHoldingsAuth` or any `/mf` write. The only non-GET Kite requests this layer may send are `POST /session/token` (login) and `DELETE /session/token` (logout). Task 24 proves it against a recording fake.
- **Loopback redirect:** `http://127.0.0.1:7420/kite/callback` by default; the port is the setting `kite.redirect_port` (1024–65535, default 7420); the listener binds `127.0.0.1` only, lives 10 minutes per login attempt, and the paste fallback (`CompleteLogin`) is always available, including after the listener timed out or could not bind.
- **Token expiry:** the next 06:00 IST after the login instant, evaluated on the app clock (`clock.Clock`), checked at the exact instant and every 5 s (covers suspend/resume). Expired ⇒ ticker stopped, limiter paused, token deleted, no Kite calls until the next login.
- **Secrets:** the API secret and the access token exist only in `secrets.Store` (`secrets.KeyAPISecret`, `secrets.KeyAccessToken`) and are registered with `logging.RegisterSecret` before any log line can contain them. They never reach `app.db`, settings, events, binding results, error strings or logs. The API key (not secret) lives in the setting `kite.api_key` and is shown masked (`••••` + last 4).
- **Layering:** `internal/service` is the only entry point for `internal/app` (rule 7). `internal/app` imports `service`, `logging`, the P0-1 platform packages its `Boot` already holds (`bus`, `settings`, `store`, `secrets`) and Wails, plus `godbus` for forwarding a second launch; `internal/app/api` imports only `service`. Only `internal/kite/...` imports gokiteconnect (rule 3). Only `internal/store` and `internal/candles/sqlite` import `database/sql` (rule 4). Only `internal/app/...` and `cmd/tradeloom` import Wails (rule 2). The single-instance lock is P0-1's `store.AcquireLock` inside `app.Start`; this plan only adds the D-Bus focus/forwarding on top of it.
- No `time.Now`, `time.Sleep` or `time.After` outside `internal/clock` and `_test.go` files (rule 8). Every goroutine starts through `logging.Go(name, fn)`.
- Money is integer paise; percentages are integer basis points (`2.50 %` = `250`); every timestamp in a binding or event payload is Unix milliseconds `int64`. No `time.Time`, interface or channel ever appears in a payload.
- Wails event names equal bus topic names (§3.4). Each topic is emitted at most once per 100 ms, except the "immediate" topics `session.state`, `candle.closed`, `watchlists.changed`, `settings.changed`, `order.update`, `market.status`, `job.progress` and `app.navigate`.
- **Execution order of Tasks 22–33:** Tasks 27–32 and Task 33 Steps 1–4 add only `internal/service`, `internal/settings`, `internal/store` and `internal/licences` code; they are executed **after Task 21 and before Task 22**, because Task 22 binds their façade methods and Task 23 wires their dependencies. Task 33 Steps 5–8 (binding regeneration, the contract test, the plan's final tables) run after Task 23. The numbering keeps the Wails layer, the wiring and the spikes together; the `Interfaces → Consumes` line of each task names what it needs.
- A rejected binding promise carries a JSON **string** `{"code":"…","stage":"…","reason":"…"}` (§8.4).
- Kite login is always manual, in the system browser. The app never stores or automates passwords or TOTP.
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **The user finishes the Kite login after the loopback listener timed out** (the tab was left open for more than 10 minutes, or the port was taken). The browser shows "can't connect". Pasting the address-bar URL must still log in, and the session must not stay stuck in `logging_in`. Test: `TestLogin_AfterListenerTimeout_PasteStillWorks` in Task 7.
2. **The `request_token` is reused or has expired** (the user reloads the callback tab, presses Back, or waits several minutes). A reload after success shows the success page again without a second exchange. A rejected exchange (`TokenException`) gives the code `request_token_invalid` with retry guidance. It must not enter the global "session expired" state and must not pause the limiter. Tests: `TestLogin_CallbackReloadAfterSuccess_IsIdempotent` and `TestLogin_RequestTokenReusedOrExpired` in Task 7.
3. **The token expires at 06:00 while the ticker is running, including when the laptop was suspended across 06:00.** Within 5 s of 06:00 (or of resume), the state must be `expired`, the ticker stopped, the limiter paused, `orders.Sync` stopped and the token deleted. Tests: `TestExpiry_At0600_WithTickerRunning` and `TestExpiry_ClockJumpAfterSuspend` in Task 8, and `TestRun_StopsSyncWhenSessionExpires` in Task 14.
4. **A Personal-plan key (no data).** Kite's documented exception list has no `PermissionException`, and gokiteconnect spells the constant `PermissionError`. Every observed variant must map to `DataPlan=none`. The ticker is never started, the chart gets cached candles plus `needsDataPlan`, no backfill is submitted, and holdings, positions and orders still load. Tests: `TestIsNoDataPermission_Variants` and `TestProbe_PersonalPlan` in Task 9, `TestCandles_PersonalPlan_ServesCacheAndNeedsDataPlan` in Task 16, and `TestSync_PersonalPlan_PollsLTPAndStopsOnPermissionError` in Task 19.
5. **A second app instance is launched, possibly with a deep link** (`tradeloom 'http://127.0.0.1:7420/kite/callback?request_token=…'`). The second process must not open `app.db` or the log file. The running window must be focused, and a redirect URL in the arguments must be routed to the paste path. With no session bus, the second process prints the reason and exits 1. Tests: P0-1's `TestRun_SecondInstanceExits` (the lock is `store.AcquireLock` inside `app.Start`), `TestSecondInstance_RoutesRedirectToLogin`, `TestForwardToRunningInstance_DeliversArgs` and `TestRun_RestartExitCode` in Task 23.

---

## File Structure

Created by this plan (P0-1/P0-2 own every other file in these packages):

| Path | Responsibility |
|---|---|
| `internal/kite/session_types.go` | `State`, `DataPlan`, error codes, `SessionState`, `TokenRecord`, `LoginError`, `TopicSessionState`, `APIError`, `MaskUserID` |
| `internal/kite/expiry.go` | `NextExpiry` (06:00 IST rule) |
| `internal/settings/registry.go` | Package-level key registry: `Register`, `Names`, `GetJSON`, `SetJSON`, `ErrUnknownKey` (P0-1's five keys registered in `init()`) |
| `internal/settings/keys_ui.go` | UI settings keys `appearance.theme`, `appearance.follow_system`, `appearance.sidebar_collapsed`, `ui.last_route`, `chart.default_timeframe`, `calendar.stale_toast_day` (contract §E) |
| `internal/settings/shortcuts.go` | Shortcut registry: `ShortcutAction`, `DefaultShortcuts`, `ParseBinding`, `Conflicts` (contract §E, P0-4 A8) |
| `internal/store/shortcuts.go`, `internal/store/windowstate.go` | `ShortcutsRepo` over `shortcuts`, `WindowStateRepo` over `window_state` |
| `internal/licences/notices.go` | Embeds the generated `THIRD_PARTY_NOTICES.md` copy for `System.Info` |
| `internal/kite/credentials.go` | API key/secret validation, settings keys `kite.api_key` / `kite.redirect_port`, `RedirectURL`, `MaskKey`, `CredentialsInfo` |
| `internal/kite/redirect.go` | `ParseRedirect` for callback queries and pasted URLs/tokens |
| `internal/kite/loopback.go` | 127.0.0.1 callback HTTP listener and result pages |
| `internal/kite/auth.go` | `AuthAPI` over `*kiteconnect.Client` (login URL with `state`, `GenerateSession`, `InvalidateAccessToken`) |
| `internal/kite/client_key.go` | `(*Client).SetAPIKey` — rebuilds P0-2's two inner gokiteconnect clients with a new key |
| `internal/kite/session_relay.go` | `SessionErrorRelay`: late-bound `Options.OnSessionError` target (`Session` is built after `Client`) |
| `internal/kite/netguard.go` | `AllowedHosts`, `ErrHostNotAllowed`, `NewAllowListTransport` (NFR-04 network allow-list) |
| `internal/kite/feed/feed.go` | `feed.Feed`: one `ticker.Manager` per login; subscriptions remembered per (consumer, owner) and replayed on `Start`; raw tick stream for the builder |
| `internal/kite/portfolio_types.go` | `Position`, `Holding` aliases missing from P0-2's `types.go` |
| `internal/kite/session.go` | `Session`: dependencies, credentials save, restore at startup, risk-ack, post-login probe and feed start |
| `internal/kite/login.go` | `BeginLogin`, callback handling, `CompleteLogin`, `CancelLogin`, token exchange, exchange-error mapping |
| `internal/kite/watcher.go` | `Run` (06:00 and 5 s checks), `HandleTokenError`, expiry |
| `internal/kite/probe.go` | `CapabilityProbe`, `IsNoDataPermission`, `DialTicker` |
| `internal/kite/logout.go` | `Logout`, `RemoveCredentials`, `SetRedirectPort` |
| `internal/kite/profile.go` | `Profile` DTO, redaction (`LogValue`), avatar host filter |
| `internal/kite/live_login_test.go` | `-tags kitelive` S0-1 harness against the real Kite |
| `internal/kite/mock/recorder.go` | `mock.Recorder`: recording Kite HTTP fake used by the zero-writes test |
| `internal/kite/mock/zero_writes_test.go` | End-to-end test: login → sync → logout, asserting no order-write route |
| `internal/store/errors_p03.go` | `ErrNoRow`, `ErrDuplicate` |
| `internal/store/storetest/storetest.go` | `storetest.Open(t)`: migrated temporary `app.db` for tests outside `package store` |
| `internal/store/sessions.go` | `SessionsRepo` over the v1 `sessions` table |
| `internal/store/watchlists.go` | `WatchlistsRepo` over `watchlists` / `watchlist_items` |
| `internal/store/orderscache.go` | `OrdersCacheRepo` over `orders_cache` / `positions_cache` |
| `internal/orders/book.go`, `positions.go`, `holdings.go` | Pure row and summary builders (paise, basis points) |
| `internal/orders/sync.go` | Read-only `Sync` actor (poll, order updates, reconciliation, LTP overlay) |
| `internal/service/deps.go` | `Service`, `Deps`, the narrow interfaces it consumes, `ErrRefused` |
| `internal/service/session.go` | Session/credentials/profile methods, `SessionStatus`, `Run` supervisor |
| `internal/service/watchlists.go` | Watchlist rules (names, default list, active list, undo position) |
| `internal/service/chart.go` | Candles from the store (trading-day ranges), backfill submission, manual refresh, live chart subscriptions, instrument lookup |
| `internal/service/settings.go` | Generic settings get/set with a deny-list |
| `internal/service/portfolio.go` | Order book / positions / holdings snapshots and refresh |
| `internal/service/events.go` | Event catalogue, bus→UI coalescing sources, market-status clock, last-quote cache, navigation |
| `internal/service/secrets.go` | `SecretSwitch` (runtime secrets backend), secret-storage setup/unlock/migration (FR-ACCT-013) |
| `internal/service/shortcuts.go` | Shortcut list/rebind/reset over the registry and `ShortcutsRepo`; UI state get/set |
| `internal/service/quotes.go` | `Quotes.Watch`: per-consumer live sets, polled fallback, persisted last-quote cache |
| `internal/service/data.go` | Data stats, clear candles, instrument refresh, backup/restore jobs, daily scheduled backup (FR-DATA-005) |
| `internal/service/calendar.go` | Calendar info, import, upsert and remove of overrides (FR-DATA-001 UI backend) |
| `internal/service/diagnostics.go` | Logs folder, support bundle job, crash report access (FR-SHELL-018) |
| `internal/service/system.go` | `AppInfo` (version, build, licences), `StatusSnapshot`, `ReconnectFeed` |
| `internal/app/app.go` (replaces P0-1 Task 1's hello `App`; keeps `app.Version`) | `App` lifecycle (`Startup`/`Shutdown`/`BeforeClose`), runtime adapter, focus on login, restart request |
| `internal/app/errors.go` | `ErrorFormatter` (JSON string) |
| `internal/app/options.go` | `Options(a, assets, uniqueID, smoke)` → `*options.App` |
| `internal/app/singleinstance.go` | `UniqueID`, D-Bus forwarding to the running instance (Wails protocol), `SecondInstance`; the lock itself is P0-1's `store.AcquireLock` inside `app.Start` |
| `internal/app/events.go` | `//go:generate` hook for `frontend/src/api/events.gen.ts` |
| `internal/app/eventsgen/main.go` | TS generator for event names + payload interfaces |
| `internal/app/api/{session,portfolio,settings,chart,instruments,quotes,watchlists,data,calendar,diagnostics,system}.go` | Wails-bound structs (11, bound in this order) |
| `internal/app/contract_test.go` | Asserts the event catalogue and the bound structs equal the canonical contract (§C/§D) |
| `internal/notify/desktop/probe.go` | S0-8 `org.freedesktop.Notifications` probe |
| `cmd/tradeloom/wire_p03.go` | `buildPlatform` (P0-2 constructors in ARCH §3.2 order, §3.3 shutdown order) and `wireP03` (this plan's components) |
| `internal/app/stdstreams.go` | stdout/stderr capture through the redacting logger |
| `docs/spec/kite-login.md` | S0-1 output |
| `docs/architecture/spikes/S0-8-single-instance.md` | S0-8 output |
| Generated: `frontend/wailsjs/go/api/*.{js,d.ts}`, `frontend/wailsjs/go/models.ts`, `frontend/src/api/events.gen.ts` | Committed bindings (§8.1) |

Modified: `cmd/tradeloom/main.go` (P0-1 Task 15: `run` extended), `internal/app/app.go` + `app_test.go` (P0-1 Task 1: replaced), `.go-arch-lint.yml` (P0-1 Task 16), `Makefile` (P0-1: `bindings` target added, `notices` target also refreshes `internal/licences/THIRD_PARTY_NOTICES.md`), `internal/service/deps.go` (Task 14; later tasks add `Deps` fields and `Service` state as listed in their Files blocks).

## Consumes (from P0-1/P0-2)

Reconciled by the consistency review 2026-10-12 against the "Produces" blocks of `2026-10-12-p0-1-platform-core/MANIFEST.md` and `2026-10-12-p0-2-market-data/MANIFEST.md` (whose code was executed and passes). Every call site in this plan uses exactly these names. What P0-1/P0-2 do not provide and this plan adds is listed at the end of this section.

```go
// ---- P0-1 ----
package bus // Task 3
type Topic[T any] struct{…}; func NewTopic[T any](name string) Topic[T]; func (t Topic[T]) Name() string
type SubOpts[T any] struct { Name string; Buffer int; Critical bool; Coalesce func(old, new T) T }
type Bus struct{…}; func New(log *slog.Logger) *Bus; func (b *Bus) Close()          // tests: bus.New(nil)
func Publish[T any](b *Bus, t Topic[T], v T)                                        // never blocks a non-critical subscriber
func Subscribe[T any](b *Bus, t Topic[T], o SubOpts[T]) *Sub[T]; func Unsubscribe[T any](b *Bus, s *Sub[T])
type Sub[T any] struct{…}; func (s *Sub[T]) C() <-chan T; func (s *Sub[T]) Drops() uint64

package logging // Tasks 4–5
type Logger struct { *slog.Logger; Level *slog.LevelVar; … }; func (l *Logger) Close() error
func Go(name string, fn func()); func RegisterSecret(v string); func ParseLevel(s string) (slog.Level, error)
func UnseenCrashReports(dir string) ([]string, error); func MarkCrashSeen(path string) error // crash-*.json → crash-*.seen.json
type BundleFile struct { Name string; Data []byte }; type BundleOptions struct { LogDir, ExportDir string; Files []BundleFile; Now func() time.Time; Redactor *Redactor }
func CreateSupportBundle(o BundleOptions) (string, error) // <ExportDir>/tradeloom-support-YYYYMMDD-HHMMSS.zip (logs, crash reports, Files, MANIFEST.txt), scrubbed

package store // Tasks 6–10
const EnvDataDir = "TRADELOOM_DATA_DIR"; func ResolveDataDir(getenv func(string) string, home string) (string, error)
type Layout struct { Root, AppDB, CandlesDir, CandlesDB, LogsDir, ExportsDir, BackupsDir, LibraryDir, InstrumentsDir, SecretsFile, LockFile string }
var ErrLocked; type ExitState string; type Lock struct{…}; func AcquireLock(l Layout, pid int) (*Lock, error) // used by app.Start only
type Options struct { AppVersion string; BackupDir string; Now func() time.Time; Migrations fs.FS; Log *slog.Logger }
type DB struct { W *sql.DB; R *sql.DB; … } // W: single read-write connection; R: up to 4 query_only connections
func Open(ctx context.Context, path string, o Options) (*DB, error); func (db *DB) Path() string; func (db *DB) SchemaVersion() int; func (db *DB) Close() error
func EmbeddedMigrations() fs.FS; func SupportedSchema() int
type SettingsRepo struct{…}; func (db *DB) SettingsRepo() *SettingsRepo
type CalendarOverrideRow struct { Exchange string; Date int; Type string; Phases string; Source string } // P0-2 Task 5
func (db *DB) CalendarOverrides(ctx context.Context) ([]CalendarOverrideRow, error)
func (db *DB) UpsertCalendarOverrides(ctx context.Context, rs []CalendarOverrideRow) error; func (db *DB) DeleteCalendarOverride(ctx context.Context, exchange string, date int) error
func NewLayout(root string) Layout; func (l Layout) Ensure() error
type Manifest struct { Format int; CreatedAt string; AppVersion string; Schema int; CalendarVersion string; LibraryVersion string; IncludesCandles bool; Counts map[string]int }
type BackupOptions struct { Dir string; IncludeCandles bool; CandlesDB string; AppVersion string; CalendarVersion string; LibraryVersion string; Now func() time.Time }
func (db *DB) Backup(ctx context.Context, o BackupOptions) (string, error)                       // <Dir>/tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst
func (db *DB) ScheduledBackup(ctx context.Context, o BackupOptions, keep int) (string, []string, error) // backup + PruneBackups(dir, keep)
func InspectBackup(path string) (Manifest, error)                                                 // ErrBadArchive
const RestoreConfirmPhrase = "RESTORE"; var ErrBadArchive, ErrNotConfirmed, ErrDeploymentsActive error
type RestoreRequest struct { Archive, DataDir, Confirmation string; DeploymentsFlat bool; Now func() time.Time }; type RestoreResult struct { Manifest Manifest; ReplacedDir string }
func Restore(ctx context.Context, req RestoreRequest) (RestoreResult, error) // caller closes app.db first and keeps the lock
func (db *DB) SupportFiles(ctx context.Context) ([]logging.BundleFile, error) // settings.json, versions.json, order_journal.json, deployment_logs.json
// v1 tables: settings, sessions, shortcuts(action_id PK, binding, scope), window_state(k PK, v), instruments, instrument_imports,
// instrument_fts, watchlists, watchlist_items, order_journal, orders_cache, positions_cache, order_counters, notifications, calendar_overrides.

package settings // Task 10
type Key[T any] struct { Name string; Default T; Validate func(T) error }
type Changed struct { Key string `json:"key"`; Value json.RawMessage `json:"value"` }; var Topic = bus.NewTopic[Changed]("settings.changed")
var ErrSecretKey, ErrSecretValue error; type ValidationError struct { Key string; Err error }
type Settings struct{…}; func Load(ctx context.Context, b Backend, bs *bus.Bus) (*Settings, error)
func Get[T any](s *Settings, k Key[T]) T; func Set[T any](ctx context.Context, s *Settings, k Key[T], v T) error; func Reset[T any](ctx context.Context, s *Settings, k Key[T]) error
func OneOf(allowed ...string) func(string) error; func IntRange(lo, hi int) func(int) error; func AbsPathOrEmpty(v string) error
var LogLevel Key[string]; var BackupScheduleEnabled Key[bool]; var BackupScheduleFolder Key[string]; var BackupScheduleKeep Key[int]; var BackupIncludeCandles Key[bool]

package secrets // Tasks 12–14
type Key string; const KeyAPISecret Key = "kite.api_secret"; const KeyAccessToken Key = "kite.access_token"
type Store interface { Get(Key) (string, error); Set(Key, string) error; Delete(Key) error; Backend() string }
const BackendKeyring = "keyring"; const BackendFile = "file"; const BackendNone = "none"
var ErrNotFound, ErrUnavailable, ErrStorageRefused, ErrWrongPassphrase, ErrPassphraseTooShort, ErrCorruptFile, ErrFileExists error
const MinPassphraseRunes = 8; var DefaultKDF KDFParams // argon2id t=3, m=64 MiB, p=4: ~1 s per CreateEncFile/OpenEncFile
type Mode string; const ModeKeyring, ModeFile, ModeChoose Mode = "keyring", "file", "choose"
type Choice string; const ChoiceEncryptedFile, ChoiceNoStorage Choice = "encrypted_file", "no_storage"
type Detection struct { Mode Mode; KeyringAvailable bool; FileExists bool; ProbeError error; Choices []Choice }
func Detect(secretsFile string, probe func() error) Detection; const ProbeTimeout = 3 * time.Second; func ProbeKeyringWithin(d time.Duration) error
func NewKeyring() Store; func NewNone() Store // NewNone: Get → ErrNotFound, Set → ErrStorageRefused, Backend() "none"
func CreateEncFile(path string, passphrase []byte) (*EncFile, error); func OpenEncFile(path string, passphrase []byte) (*EncFile, error) // *EncFile implements Store
func (f *EncFile) Close(); func (f *EncFile) Destroy() error
func Migrate(from, to Store) error // copy + verify every present key, then delete from source (an *EncFile source is removed)

package app // Task 15 (kept; this plan's Task 22 replaces only Task 1's app.go/app_test.go)
var Version = "dev"
type BootOptions struct { DataDir string; AppVersion string; PID int; Now func() time.Time; Stderr io.Writer; ProbeKeyring func() error }
type Boot struct { Layout store.Layout; Lock *store.Lock; Log *logging.Logger; DB *store.DB; Bus *bus.Bus; Settings *settings.Settings
    Secrets secrets.Detection; PreviousExit store.ExitState; CrashReports []string; CrashedLastTime bool; … }
func Start(ctx context.Context, o BootOptions) (*Boot, error); func (b *Boot) Shutdown() error; func ExitMessage(dataDir string, err error) string
// cmd/tradeloom/main.go: func run(args []string, getenv func(string) string, stderr io.Writer) int — exit 0 ok / 1 refused / 2 bad flags; flag --smoke.
package frontend; func Assets() fs.FS
package archtest // rule 2 and rule 7 match internal/app by prefix: internal/app/api may import Wails-free code and only internal/service.

// ---- P0-2 ----
package clock // Tasks 1–2
var IST *time.Location
type Timer interface { C() <-chan time.Time; Stop() bool }
type Clock interface { Now() time.Time; System() time.Time; Offset() Offset; At(target time.Time) Timer; After(d time.Duration) Timer }
type Offset struct { Value time.Duration `json:"-"`; Millis int64 `json:"millis"`; Source Source `json:"source"`; Samples int `json:"samples"`; Warn bool `json:"warn"`; Skewed bool `json:"skewed"`; UpdatedAt time.Time `json:"updatedAt"` }
var TopicOffset = bus.NewTopic[Offset]("clock.offset")
type Options struct { Bus *bus.Bus; Log *slog.Logger; … }; type Real struct{…}; func New(o Options) *Real
func (r *Real) ObserveTick(exchangeTS, arrival time.Time); func (r *Real) ObserveHTTPDate(date, sent, received time.Time)
type Fake struct{…}; func NewFake(now time.Time) *Fake; func (f *Fake) Advance(d time.Duration); func (f *Fake) Set(t time.Time) // both fire due timers
// Pattern used everywhere in this plan: tm := clk.At(t); defer tm.Stop(); select { case <-tm.C(): … }; re-arm by calling At again.

package calendar // Tasks 3–5
type Service struct{…}; func NewDefault() (*Service, error); func (s *Service) SetOverrides(es []Entry)
func (s *Service) MergeOverrides(es []Entry) []Entry; func (s *Service) DeleteOverride(ex Exchange, d Date); func (s *Service) Overrides() []Entry
func (s *Service) Version() string; func (s *Service) CoversUntil() Date; func (s *Service) Stale(now time.Time) bool
func (s *Service) Day(ex Exchange, d Date) Day; func (s *Service) IsTradingDay(ex Exchange, d Date) bool; func (s *Service) PrevTradingDay(ex Exchange, d Date) (Date, bool)
func (s *Service) PhaseAt(ex Exchange, t time.Time) (Phase, Day) // Phase{Kind: PhaseClosed} with zero Start/End outside every phase
func (s *Service) LabelAt(ex Exchange, t time.Time) Label; type Label struct { Text string `json:"text"`; Tooltip string `json:"tooltip"` }
type Date struct { Year int; Month time.Month; Day int }; func DateOf(t time.Time) Date; func ParseDate(s string) (Date, error) // "2006-01-02"
func (d Date) String() string; func (d Date) Int() int; func (d Date) AddDays(n int) Date; func (d Date) Midnight() time.Time
type HM int; func ParseHM(s string) (HM, error); func (h HM) String() string // "HH:MM"
type Exchange string; const ( NSE Exchange = "NSE"; BSE Exchange = "BSE"; NFO Exchange = "NFO"; BFO Exchange = "BFO" ); var Exchanges = []Exchange{NSE, BSE, NFO, BFO}
type PhaseKind string; const ( PhasePreOpen PhaseKind = "pre-open"; PhaseContinuous PhaseKind = "continuous"; PhaseClosing PhaseKind = "closing"; PhasePostClosing PhaseKind = "post-closing"; PhaseClosed PhaseKind = "closed" )
type DayType string; const ( DayTrading DayType = "trading"; DayHoliday DayType = "holiday"; DaySpecial DayType = "special"; DayWeekend DayType = "weekend"; DayInferredClosed DayType = "inferred-non-trading" )
type PhaseSpec struct { Kind PhaseKind `json:"kind"`; Start HM `json:"start"`; End HM `json:"end"` }
type Phase struct { Kind PhaseKind `json:"kind"`; Start time.Time `json:"start"`; End time.Time `json:"end"` }
type Day struct { Exchange Exchange `json:"exchange"`; Date Date `json:"-"`; Type DayType `json:"type"`; Phases []Phase `json:"phases"` }; func (d Day) Phase(k PhaseKind) (Phase, bool)
type Entry struct { Date Date; Exchange Exchange; Type DayType; Phases []PhaseSpec; Source string }
type File struct { Version string; CoversFrom, CoversTo Date; Entries []Entry }; type RowError struct { Row int; Msg string } // Error() == "Row N: msg"
func ParseFile(r io.Reader, source string) (File, error) // header "date,exchange,type,phase,start,end"; all-or-nothing; *RowError per bad data row
type Record struct { Exchange string; Date int; Type string; Phases string; Source string }; func ToRecord(e Entry) Record; func FromRecord(r Record) (Entry, error)

package ratelimit // Task 6 (internal/kite/ratelimit)
type Priority uint8; const Interactive Priority = iota
type Options struct { Rates map[Class]int; Margin time.Duration; Bus *bus.Bus; Log *slog.Logger }
type Limiter struct{…}; func New(clk clock.Clock, o Options) *Limiter; func (l *Limiter) Run(ctx context.Context)
func (l *Limiter) PauseAll(reason string); func (l *Limiter) Resume(); func (l *Limiter) Snapshot() Usage
type Usage struct { At time.Time `json:"at"`; Paused bool `json:"paused"`; PauseReason string `json:"pauseReason"`; Classes []ClassUsage `json:"classes"` }
var TopicUsage = bus.NewTopic[Usage]("limits.usage")

package kite // Tasks 7, 9
type ( UserProfile = kiteconnect.UserProfile; HistoricalData = kiteconnect.HistoricalData; QuoteLTP = kiteconnect.QuoteLTP; QuoteOHLC = kiteconnect.QuoteOHLC
    Order = kiteconnect.Order; Orders = kiteconnect.Orders; Positions = kiteconnect.Positions; Holdings = kiteconnect.Holdings; Instruments = kiteconnect.Instruments )
type ReadClient interface { Profile(ctx) (UserProfile, error); Historical(ctx, token int, interval string, from, to time.Time, continuous, oi bool) ([]HistoricalData, error)
    LTP(ctx, ids ...string) (QuoteLTP, error); OHLC(ctx, ids ...string) (QuoteOHLC, error); Orders(ctx) (Orders, error); Positions(ctx) (Positions, error); Holdings(ctx) (Holdings, error); … }
type HistoricalRequest struct { Token int; Interval string; From, To time.Time; Continuous bool; OI bool }
type HistoricalPicker interface { HistoricalPick(ctx context.Context, prio ratelimit.Priority, pick func() (HistoricalRequest, bool)) (HistoricalRequest, []HistoricalData, bool, error) }
// kiteconnect.HistoricalData{Date models.Time; Open, High, Low, Close float64; Volume int; OI int}; kiteconnect.QuoteOHLC is map[string]struct{InstrumentToken int; LastPrice float64; OHLC struct{Open, High, Low, Close float64}}
var TopicOrderUpdate = bus.NewTopic[Order]("order.update")
type Kind uint8; const ( KindNone Kind = iota; KindSession; KindPermission; KindRateLimited; KindNetwork; KindInput; KindOrder; KindServer; KindCanceled; KindUnknown )
type Err struct { Kind Kind; Op string; Code int; Type string; Message string; … }; func (e *Err) Error() string; func (e *Err) Unwrap() error // Unwrap reaches the kiteconnect.Error
func Classify(err error) Kind
type Options struct { APIKey string; BaseURI string; Limiter *ratelimit.Limiter; Clock clock.Clock; DateSink func(date, sent, received time.Time)
    OnSessionError func(e *Err); Transport http.RoundTripper; Jitter func() time.Duration; Log *slog.Logger }
type Client struct{…}; func New(o Options) *Client; func (c *Client) SetAccessToken(token string); func (c *Client) APIKey() string
// Client implements ReadClient and HistoricalPicker; inner gokiteconnect clients: 10 s (api) and 30 s (hist) timeouts, both on one recording transport.

package mock // Task 8 (internal/kite/mock): Server, NewServer, Fake, NewFake, Packet, Failure, Route* constants — untouched by this plan's Recorder (Task 24).

package candles // Task 10
type Interval string; var Native []Interval; func ParseInterval(s string) (Interval, error)
type Source uint8; const ( SourceHistorical Source = 0; SourceTick Source = 1; SourceManual Source = 2 )
type Candle struct { Start time.Time `json:"t"`; Open float64 `json:"o"`; High float64 `json:"h"`; Low float64 `json:"l"`; Close float64 `json:"c"`
    Volume int64 `json:"v"`; OI int64 `json:"oi"`; HasOI bool `json:"hasOi"`; Source Source `json:"src"`; Complete bool `json:"complete"` }
type Range struct { From time.Time `json:"from"`; To time.Time `json:"to"` }
type Store interface { Read(ctx, token uint32, iv Interval, from, to time.Time) ([]Candle, error); Missing(ctx, token uint32, iv Interval, from, to time.Time) ([]Range, error)
    Upsert(ctx, token uint32, iv Interval, cs []Candle, src Source) error; Stats(ctx) (Stats, error); Clear(ctx, confirmRuns []string) error; … }
type Stats struct { Intervals []IntervalStats `json:"intervals"`; TotalBytes int64 `json:"totalBytes"`; BudgetBytes int64 `json:"budgetBytes"` }
type IntervalStats struct { Interval Interval `json:"interval"`; Rows int64 `json:"rows"`; Bytes int64 `json:"bytes"` }
type RunsReferencedError struct { Runs []string }; func (e *RunsReferencedError) Error() string // Clear/Purge refusal when backtest runs reference candles
type Job struct { ID string; Token uint32; Interval Interval; … }; type JobStore interface { …; OpenJobs(ctx context.Context) ([]Job, error) }
func (iv Interval) Duration() time.Duration; func (iv Interval) IsIntraday() bool
package sqlite // Task 11 (internal/candles/sqlite)
type Options struct { Clock clock.Clock; Sessions func(token uint32, d calendar.Date) (start, end time.Time, ok bool); Calendar *calendar.Service }
type Store struct{…}; func Open(ctx context.Context, path string, o Options) (*Store, error); func (s *Store) Close() error // implements candles.Store and candles.JobStore

package backfill // Task 14 (internal/candles/backfill)
type Job struct { Token uint32; Interval candles.Interval; From, To time.Time; Requester string; Priority ratelimit.Priority; OI bool }
type Progress struct { JobID string `json:"jobId"`; Token uint32 `json:"token"`; Interval string `json:"interval"`; Done int `json:"done"`; Failed int `json:"failed"`
    Total int `json:"total"`; ETA time.Duration `json:"etaNs"`; State string `json:"state"` }
var TopicProgress = bus.NewTopic[Progress]("backfill.progress"); var ErrFailed error
type Options struct { Store candles.Store; Jobs candles.JobStore; Fetcher kite.HistoricalPicker; Clock clock.Clock; Bus *bus.Bus; Log *slog.Logger; Workers int }
type Backfill struct{…}; func New(o Options) *Backfill; func (b *Backfill) Start(ctx context.Context) error; func (b *Backfill) WaitStopped()
func (b *Backfill) Submit(ctx context.Context, j Job) (string, error); func (b *Backfill) Wait(ctx context.Context, id string) (Progress, error)

package instruments // Tasks 15–16
type Tag string; const ( TagIndices Tag = "INDICES"; TagNSE Tag = "NSE"; TagBSE Tag = "BSE"; TagNFOFut Tag = "NFO-FUT"; TagBFOFut Tag = "BFO-FUT" )
type Instrument struct { Token uint32 `json:"token"`; ExchangeToken uint32 `json:"exchangeToken"`; Exchange string `json:"exchange"`; Segment string `json:"segment"`; Tag Tag `json:"tag"`
    Tradingsymbol string `json:"tradingsymbol"`; Name string `json:"name"`; Type string `json:"instrumentType"`; Expiry time.Time `json:"expiry"`; TickSizePaise int64 `json:"tickSizePaise"`
    LotSize int64 `json:"lotSize"`; DisplayName string `json:"displayName"`; Underlying string `json:"underlying"`; IsIndex bool `json:"isIndex"`; IsWeekly bool `json:"isWeekly"` }
func (i Instrument) ID() string; func (i Instrument) IsFuture() bool
type Master struct{…}; func NewMaster(clk clock.Clock) *Master; func (m *Master) ByToken(token uint32) (Instrument, bool); func (m *Master) ByID(id string) (Instrument, bool)
func (m *Master) ExchangeOf(token uint32) calendar.Exchange; func (m *Master) ImportedAt() time.Time
type Result struct { Instrument Instrument `json:"instrument"`; Rank int `json:"rank"`; AlsoOnNSE bool `json:"alsoOnNse"` }; func (m *Master) Search(q string, limit int) []Result
type Options struct { Client kite.ReadClient; Master *Master; DB *store.DB; Calendar *calendar.Service; Clock clock.Clock; Bus *bus.Bus; DataDir string; Log *slog.Logger }
type Refresher struct{…}; func NewRefresher(o Options) *Refresher; func (r *Refresher) Load(ctx context.Context) error; func (r *Refresher) Refresh(ctx context.Context) error; func (r *Refresher) Run(ctx context.Context)

package ticker // Tasks 17–18 (internal/kite/ticker)
type Mode uint8; const ( ModeLTP Mode = 1; ModeQuote Mode = 2; ModeFull Mode = 3 )
type Consumer uint8; const ( ConsumerClock Consumer = iota; ConsumerDeployment; ConsumerLiveScanner; ConsumerChart; ConsumerWatchlist; ConsumerWidget; ConsumerRecorder ); func (c Consumer) String() string
type Tick struct { Token uint32 `json:"token"`; Mode string `json:"mode"`; LTP float64 `json:"ltp"`; Change float64 `json:"change"`; Open float64 `json:"open"`; High float64 `json:"high"`; Low float64 `json:"low"`
    Close float64 `json:"close"`; Volume uint32 `json:"volume"`; LastQty uint32 `json:"lastQty"`; OI uint32 `json:"oi"`; ExchangeTS time.Time `json:"exchangeTs"`; LastTradeTime time.Time `json:"lastTradeTime"`
    Received time.Time `json:"received"`; IsIndex bool `json:"isIndex"` }
type ErrBudget struct { Used, Requested, Limit int; ByConsumer map[string]int }; func (e *ErrBudget) Error() string
type FeedState struct { State string `json:"state"`; Attempt int `json:"attempt"`; NextRetry time.Time `json:"nextRetry"`; LastTick time.Time `json:"lastTick"`; Connections int `json:"connections"`; Tokens int `json:"tokens"` }
type Usage struct { Used int `json:"used"`; Limit int `json:"limit"`; Connections int `json:"connections"`; ByConsumer map[string]int `json:"byConsumer"`; RawDrops uint64 `json:"rawDrops"` }
var TopicTicks = bus.NewTopic[[]Tick]("tick.batch"); var TopicFeed = bus.NewTopic[FeedState]("feed.state")
type Events struct { OnConnect func(); OnTick func(models.Tick); OnOrderUpdate func(kiteconnect.Order); OnError func(error) }
type Conn interface { Serve(ctx context.Context); Subscribe(tokens []uint32) error; Unsubscribe(tokens []uint32) error; SetMode(m Mode, tokens []uint32) error; Stop() }
type Dialer func(ev Events) Conn; type TickObserver interface { ObserveTick(exchangeTS, arrival time.Time) }
type Options struct { Dial Dialer; Clock clock.Clock; Bus *bus.Bus; Log *slog.Logger; PerConn int; MaxConns int; ClockRef uint32; Observer TickObserver; Coalesce time.Duration; … }
type Manager struct{…}; func New(o Options) *Manager; func (m *Manager) Start(ctx context.Context) error; func (m *Manager) Stop(); func (m *Manager) Reconnect()
func (m *Manager) Subscribe(c Consumer, owner string, tokens []uint32, mode Mode) error; func (m *Manager) Unsubscribe(c Consumer, owner string, tokens []uint32)
func (m *Manager) Release(c Consumer, owner string); func (m *Manager) Usage() Usage; func (m *Manager) Raw(buffer int) (<-chan Tick, func())
func KiteDialer(apiKey, accessToken string, root *url.URL) Dialer // one Manager per login: Stop is final

package builder // Task 19 (internal/candles/builder)
type Event struct { Token uint32 `json:"token"`; Interval candles.Interval `json:"interval"`; Candle candles.Candle `json:"candle"`; Partial bool `json:"partial"` }
var TopicClosed = bus.NewTopic[Event]("candle.closed"); var TopicForming = bus.NewTopic[Event]("candle.forming")
type Options struct { Clock clock.Clock; Calendar *calendar.Service; Store candles.Store; Bus *bus.Bus; ExchangeOf func(token uint32) calendar.Exchange; Grace time.Duration; Log *slog.Logger }
type Builder struct{…}; func New(o Options) *Builder; func (b *Builder) Track(token uint32, ivs ...candles.Interval); func (b *Builder) Untrack(token uint32, ivs ...candles.Interval)
func (b *Builder) Run(ctx context.Context, ticks <-chan ticker.Tick)
```

**Added by this plan because P0-1/P0-2 do not provide them** (each is a real-code step in the task named):
- `settings.Register`, `settings.Names`, `settings.GetJSON`, `settings.SetJSON`, `settings.ErrUnknownKey` — `internal/settings/registry.go` (Task 2; consumed by Task 17).
- `storetest.Open(t) *store.DB` — `internal/store/storetest` (Task 5).
- `(*kite.Client).SetAPIKey`, `kite.SessionErrorRelay`, `kite.AllowedHosts`, `kite.ErrHostNotAllowed`, `kite.NewAllowListTransport` — Task 6.
- package `internal/kite/feed` (`feed.New`, `Start`, `Stop`, `Reconnect`, `Subscribe`, `Unsubscribe`, `Release`, `Usage`, `Ticks`, `feed.ErrNotRunning`) — Task 6.
- `kite.Position`, `kite.Holding` aliases — `internal/kite/portfolio_types.go` (Task 12).
- UI settings keys (`settings.AppearanceTheme`, `AppearanceFollowSystem`, `AppearanceSidebarCollapsed`, `UILastRoute`, `ChartDefaultTimeframe`, `CalendarStaleToastDay`) — `internal/settings/keys_ui.go` (Task 17).
- Shortcut registry (`settings.ShortcutAction`, `DefaultShortcuts`, `ParseBinding`, `Conflicts`) — `internal/settings/shortcuts.go` (Task 28); `store.ShortcutsRepo`, `store.WindowStateRepo` — Task 28.
- `service.SecretSwitch` (a `secrets.Store` whose backend can be swapped at runtime) — Task 27.
- `licences.Notices` (embedded copy of `THIRD_PARTY_NOTICES.md`) — `internal/licences` (Task 33).

`internal/service` declares narrow interfaces (`CandleReader`, `BackfillSubmitter`, `InstrumentFinder`, `LiveSubscriber` in Task 14; `HistoricalFetcher`, `CandleTracker`, `CandleUpserter` in Task 16; `QuoteReader` in Task 29; `CandleAdmin`, `InstrumentRefresher` in Task 30; `FeedController` in Task 33) over these methods. The concrete P0-2 type names are referenced only in `cmd/tradeloom/wire_p03.go`.

## Produces (for P0-4 / P0-5)

This section is the **canonical binding, event and settings catalogue** (contract §C/§D/§E, consistency review 2026-10-12). P0-4 and P0-5 use every name below verbatim; a later change here is a contract change for both.

**Go façade** (`internal/service`; every method is also reachable from Telegram later; the task that adds it is in brackets):

```go
// session (Task 14)
func (s *Service) SessionStatus(ctx context.Context) SessionStatus
func (s *Service) CredentialsInfo(ctx context.Context) CredentialsInfo
func (s *Service) SaveCredentials(ctx context.Context, in CredentialsInput) (SessionStatus, error)
func (s *Service) RemoveCredentials(ctx context.Context) (SessionStatus, error)
func (s *Service) SetRedirectPort(ctx context.Context, port int) (CredentialsInfo, error)
func (s *Service) BeginLogin(ctx context.Context) (LoginStart, error)
func (s *Service) CompleteLogin(ctx context.Context, pasted string) (SessionStatus, error)
func (s *Service) CancelLogin(ctx context.Context) SessionStatus
func (s *Service) Logout(ctx context.Context) (LogoutResult, error)
func (s *Service) AckRiskDisclosure(ctx context.Context) (SessionStatus, error)
func (s *Service) Profile(ctx context.Context) (Profile, error)
// secret storage (Task 27, FR-ACCT-013)
func (s *Service) SecretStorage(ctx context.Context) SecretStorageInfo
func (s *Service) UseEncryptedFile(ctx context.Context, passphrase string) (SessionStatus, error) // passphrase_too_short, file_exists, already_configured
func (s *Service) RefuseSecretStorage(ctx context.Context) (SessionStatus, error)                 // already_configured
func (s *Service) Unlock(ctx context.Context, passphrase string) (SessionStatus, error)           // bad_passphrase, not_locked
func (s *Service) MigrateToKeyring(ctx context.Context) (SecretStorageInfo, error)                // keyring_unavailable, not_file_backend, migrate_failed
// feed (Task 33)
func (s *Service) ReconnectFeed(ctx context.Context) error // not_logged_in
// chart (Task 16)
func (s *Service) Candles(ctx context.Context, req CandlesRequest) (CandlesResult, error)        // invalid_interval, invalid_range
func (s *Service) RefreshCandles(ctx context.Context, req RefreshRequest) (CandlesResult, error) // invalid_interval, invalid_range, offline, no_data_plan, refresh_failed
func (s *Service) OpenLive(ctx context.Context, token uint32, interval string) (string, error)   // invalid_interval, no_data_plan, subscription_budget
func (s *Service) CloseLive(ctx context.Context, id string) error                                // not_found
// instruments (Task 16)
func (s *Service) SearchInstruments(ctx context.Context, q string, tags []string, limit int) ([]InstrumentDTO, error)
func (s *Service) InstrumentsByTokens(ctx context.Context, tokens []uint32) ([]InstrumentDTO, error)
func (s *Service) InstrumentByIdentifier(ctx context.Context, id string) (InstrumentDTO, error) // not_found
// quotes (Task 29)
func (s *Service) WatchQuotes(ctx context.Context, consumer string, tokens []uint32) (WatchResult, error) // invalid_consumer, subscription_failed
func (s *Service) RunQuotes(ctx context.Context) // 5 s poll loop for polled tokens; flushes the last-quote cache to window_state key quotes.last every 60 s and on exit
// watchlists (Task 15)
func (s *Service) Watchlists(ctx context.Context) (WatchlistsState, error)
func (s *Service) CreateWatchlist(ctx context.Context, name string) (WatchlistsState, error)
func (s *Service) RenameWatchlist(ctx context.Context, id, name string) (WatchlistsState, error)
func (s *Service) DeleteWatchlist(ctx context.Context, id string) (WatchlistsState, error)
func (s *Service) SetActiveWatchlist(ctx context.Context, id string) (WatchlistsState, error)
func (s *Service) AddToWatchlist(ctx context.Context, id string, token uint32, position int) (WatchlistsState, error)
func (s *Service) RemoveFromWatchlist(ctx context.Context, id string, token uint32) (RemovedItem, error)
func (s *Service) ReorderWatchlist(ctx context.Context, id string, tokens []uint32) (WatchlistsState, error)
func (s *Service) ReorderWatchlists(ctx context.Context, ids []string) (WatchlistsState, error)
// settings (Task 17), shortcuts and UI state (Task 28)
func (s *Service) Settings(ctx context.Context) ([]SettingValue, error)
func (s *Service) SetSetting(ctx context.Context, name, valueJSON string) (SettingValue, error) // use_credentials_flow, unknown_setting, invalid_setting
func (s *Service) Shortcuts(ctx context.Context) ([]settings.ShortcutAction, error)
func (s *Service) Rebind(ctx context.Context, actionID, binding string) (settings.ShortcutAction, error) // unknown_action, invalid, conflict (reason = other action's label)
func (s *Service) ResetShortcuts(ctx context.Context) ([]settings.ShortcutAction, error)
func (s *Service) GetUiState(ctx context.Context, key string) (string, error) // "" when absent; invalid_key
func (s *Service) SetUiState(ctx context.Context, key, value string) error
// portfolio (Task 20)
func (s *Service) OrderBook(ctx context.Context) BookSnapshot
func (s *Service) Positions(ctx context.Context) PositionsSnapshot
func (s *Service) Holdings(ctx context.Context) HoldingsSnapshot
func (s *Service) RefreshPortfolio(ctx context.Context, scope string) error // invalid_scope, not_logged_in, refresh_failed
// data (Task 30, FR-DATA-002/005)
func (s *Service) DataStats(ctx context.Context) (DataStats, error)
func (s *Service) ClearCandles(ctx context.Context) error                                              // runs_referenced
func (s *Service) RefreshInstruments(ctx context.Context) error                                        // not_logged_in, refresh_failed
func (s *Service) BackupNow(ctx context.Context, folder string, includeCandles bool) (string, error)   // job id; invalid_folder, job_running
func (s *Service) InspectBackup(ctx context.Context, path string) (BackupInfo, error)                  // not_found
func (s *Service) RestoreBackup(ctx context.Context, path, confirmation string) (string, error)        // job id; not_confirmed, bad_archive, schema_too_new, deployments_active, job_running
func (s *Service) BackupOnQuit(ctx context.Context) // scheduled backup if due (App.BeforeClose)
func (s *Service) RunBackups(ctx context.Context)   // daily scheduled backup after 16:00 IST (App.start)
// calendar (Task 31, FR-DATA-001)
func (s *Service) CalendarInfo(ctx context.Context) (CalendarInfo, error)
func (s *Service) ImportCalendar(ctx context.Context, path string) (CalendarInfo, error)                   // invalid_file (reason "Row N: msg"), not_found
func (s *Service) UpsertCalendarEntry(ctx context.Context, in CalendarEntryInput) (CalendarInfo, error)   // invalid_entry
func (s *Service) RemoveCalendarEntry(ctx context.Context, exchange, date string) (CalendarInfo, error)   // invalid_entry, not_found
// diagnostics (Task 32, FR-SHELL-018)
func (s *Service) OpenLogsFolder(ctx context.Context) (string, error)     // the folder path; internal/app opens it
func (s *Service) CreateSupportBundle(ctx context.Context) (string, error) // bundle path; also reports as job kind support_bundle; bundle_failed
func (s *Service) LastCrash(ctx context.Context) (CrashInfo, error)
func (s *Service) OpenCrashReport(ctx context.Context) (string, error)    // the report path; no_crash
func (s *Service) DismissCrash(ctx context.Context) error                 // no_crash
// system (Task 33) and status helpers (Task 21)
func (s *Service) AppInfo(ctx context.Context) AppInfo
func (s *Service) Status(ctx context.Context) StatusSnapshot
func (s *Service) MarketStatus() MarketStatus
func (s *Service) FeedState() ticker.FeedState
func (s *Service) Navigate(route string) // publishes app.navigate
// lifecycle (Tasks 14, 21)
func (s *Service) Restore(ctx context.Context) SessionStatus // deferred (state "locked") while the encrypted file is locked
func (s *Service) RunSession(ctx context.Context); func (s *Service) Run(ctx context.Context); func (s *Service) EventSources() []EventSource
```

**Bound structs** (`internal/app/api`, bound in this order by `app.New`; `len(options.App.Bind) == 11`) and the **generated TypeScript bindings** (`frontend/wailsjs/go/api/<Struct>.d.ts`; each function returns a `Promise`, `(T, error)` → `Promise<T>`, `error` → `Promise<void>`; a rejection carries the JSON-string error `{"code","stage","reason"}`):

| File | Functions (TS as generated) | Returns |
|---|---|---|
| `api/Session` | `Status()`, `Credentials()`, `SaveCredentials(arg1: service.CredentialsInput)`, `RemoveCredentials()`, `SetRedirectPort(arg1: number)`, `Login()` (opens the system browser), `CompleteLogin(arg1: string)`, `CancelLogin()`, `Logout()`, `AckRiskDisclosure()`, `Profile()`, `SecretStorage()`, `UseEncryptedFile(arg1: string)`, `RefuseSecretStorage()`, `Unlock(arg1: string)`, `MigrateToKeyring()`, `ReconnectFeed()` | `service.SessionStatus`; `kite.CredentialsInfo`; `service.SessionStatus`; `service.SessionStatus`; `kite.CredentialsInfo`; `kite.LoginStart`; `service.SessionStatus`; `service.SessionStatus`; `kite.LogoutResult`; `service.SessionStatus`; `kite.Profile`; `service.SecretStorageInfo`; `service.SessionStatus` ×3; `service.SecretStorageInfo`; `void` |
| `api/Portfolio` | `OrderBook()`, `Positions()`, `Holdings()`, `Refresh(arg1: string)` | `orders.BookSnapshot`; `orders.PositionsSnapshot`; `orders.HoldingsSnapshot`; `void` |
| `api/Settings` | `All()`, `Set(arg1: string, arg2: string)`, `Shortcuts()`, `Rebind(arg1: string, arg2: string)`, `ResetShortcuts()`, `GetUiState(arg1: string)`, `SetUiState(arg1: string, arg2: string)` | `service.SettingValue[]`; `service.SettingValue`; `settings.ShortcutAction[]`; `settings.ShortcutAction`; `settings.ShortcutAction[]`; `string`; `void` |
| `api/Chart` | `Candles(arg1: service.CandlesRequest)`, `Refresh(arg1: service.RefreshRequest)`, `OpenLive(arg1: number, arg2: string)`, `CloseLive(arg1: string)` | `service.CandlesResult`; `service.CandlesResult`; `string` (subscription id); `void` |
| `api/Instruments` | `Search(arg1: string, arg2: Array<string>, arg3: number)`, `ByTokens(arg1: Array<number>)`, `ByIdentifier(arg1: string)` | `service.InstrumentDTO[]`; `service.InstrumentDTO[]`; `service.InstrumentDTO` |
| `api/Quotes` | `Watch(arg1: string, arg2: Array<number>)` | `service.WatchResult` |
| `api/Watchlists` | `List()`, `Create(arg1: string)`, `Rename(arg1: string, arg2: string)`, `Delete(arg1: string)`, `SetActive(arg1: string)`, `Add(arg1: string, arg2: number, arg3: number)`, `Remove(arg1: string, arg2: number)`, `Reorder(arg1: string, arg2: Array<number>)`, `ReorderLists(arg1: Array<string>)` | `service.WatchlistsState` for all but `Remove` → `service.RemovedItem` |
| `api/Data` | `Stats()`, `ClearCandles()`, `RefreshInstruments()`, `ChooseFolder()`, `ChooseBackupFile()`, `BackupNow(arg1: string, arg2: boolean)`, `InspectBackup(arg1: string)`, `Restore(arg1: string, arg2: string)` | `service.DataStats`; `void`; `void`; `string` (`""` when cancelled); `string`; `string` (job id); `service.BackupInfo`; `string` (job id) |
| `api/Calendar` | `Info()`, `ChooseImportFile()`, `Import(arg1: string)`, `Upsert(arg1: service.CalendarEntryInput)`, `Remove(arg1: string, arg2: string)` | `service.CalendarInfo`; `string`; `service.CalendarInfo` ×3 |
| `api/Diagnostics` | `OpenLogsFolder()`, `CreateSupportBundle()`, `LastCrash()`, `OpenCrashReport()`, `DismissCrash()` | `void`; `string` (zip path); `service.CrashInfo`; `void`; `void` |
| `api/System` | `Info()`, `Status()` | `service.AppInfo`; `service.StatusSnapshot` |

Refusal codes per binding (A1): `Session.SetRedirectPort` `invalid_port`; `Session.UseEncryptedFile` `passphrase_too_short`; `Session.Unlock` `bad_passphrase`; `Session.MigrateToKeyring` `keyring_unavailable`; `Settings.Set` `use_credentials_flow`/`unknown_setting`/`invalid_setting`; `Settings.Rebind` `conflict` (reason = the conflicting action's label) / `invalid`; `Chart.Refresh` `no_data_plan`/`offline`; `Chart.OpenLive` `subscription_budget`; `Instruments.ByIdentifier` `not_found`; `Quotes.Watch` `invalid_consumer`; `Watchlists.*` `invalid_name`/`duplicate_name`/`already_in_list`/`unknown_instrument`/`not_found`/`stale_order`; `Data.ClearCandles` `runs_referenced`; `Data.Restore` `not_confirmed`/`bad_archive`/`schema_too_new`/`deployments_active`; `Calendar.Import` `invalid_file` with reason `Row N: msg`; plain Go errors are `internal`.

**Models** (`frontend/wailsjs/go/models.ts`, Go package namespaces; JSON field names):

| Model | Fields (JSON) |
|---|---|
| `service.SessionStatus` | `state` (`no_credentials`\|`locked`\|`logged_out`\|`logging_in`\|`ready`\|`expired`), `sessionId`, `userId`, `loggedInAtMs`, `expiresAtMs`, `dataPlan` (`unknown`\|`connect`\|`none`), `offline`, `riskAckRequired`, `errorCode`, `errorMessage`, `credentials` (`kite.CredentialsInfo`), `secrets` (`service.SecretStorageInfo`) |
| `service.SecretStorageInfo` | `mode` (`keyring`\|`file`\|`choose`\|`none`), `backend` (`keyring`\|`file`\|`none`\|`locked`), `keyringAvailable`, `fileExists`, `choices` (`["encrypted_file","no_storage"]` in mode `choose`, else `[]`) |
| `service.CredentialsInput` | `apiKey`, `apiSecret` |
| `kite.CredentialsInfo` | `configured`, `apiKeyMasked`, `port`, `redirectUrl`, `secretBackend` |
| `kite.LoginStart` / `kite.LogoutResult` / `kite.Profile` | `loginUrl`, `redirectUrl`, `listenerOk`, `listenerError` / `remoteInvalidated`, `message` / `userId`, `name`, `shortName`, `email`, `broker`, `avatarUrl`, `initials`, `exchanges`, `products`, `orderTypes` |
| `service.CandlesRequest` | `token`, `interval` (Kite form), `fromMs`, `toMs` (0 = now), `tradingDays` (> 0 ⇒ Go resolves `fromMs` from the calendar) |
| `service.RefreshRequest` | `token`, `interval`, `fromMs`, `toMs` |
| `service.CandlesResult` | `candles` (`CandleDTO[]`), `complete`, `backfillJobId`, `needsDataPlan`, `offline` |
| `service.CandleDTO` / `service.CandleEventDTO` | `t` (ms, candle start), `o`, `h`, `l`, `c`, `v`, `oi`, `complete` / `token`, `interval`, `candle`, `partial` |
| `service.InstrumentDTO` | `token`, `exchange`, `segment`, `tag`, `tradingsymbol`, `name`, `displayName`, `instrumentType`, `expiryMs` (0 = none), `tickSizePaise`, `lotSize`, `isIndex`, `isFuture`, `underlying` |
| `service.TickDTO` | `token`, `ltp`, `close` (previous close), `change`, `changePct`, `volume`, `tsMs`, `polled` |
| `service.WatchResult` | `live` (`number[]`), `polled` (`number[]`), `snapshot` (`TickDTO[]`: the last quote per token from the in-memory cache, persisted to `window_state` key `quotes.last` so it survives a restart; restored entries carry `polled: false` and their original `tsMs`) |
| `service.WatchlistsState` / `service.Watchlist` / `service.WatchlistItem` / `service.RemovedItem` | `lists`, `activeId` / `id`, `name`, `items` / `token`, `exchange`, `symbol`, `displayName`, `name`, `isIndex`, `missing` / `listId`, `token`, `position` |
| `service.SettingValue` | `name`, `valueJson` |
| `settings.ShortcutAction` | `id`, `label`, `scope` (`global`\|`view`\|`chart`\|`builder`), `defaultBinding`, `binding` |
| `orders.BookSnapshot` / `OrderRow` / `SectionCounts` | `orders`, `counts`, `asOfMs`, `stale` / `orderId`, `timeMs`, `side`, `exchange`, `symbol`, `token`, `product`, `orderType`, `variety`, `qty`, `filledQty`, `pricePaise`, `triggerPaise`, `avgPricePaise`, `status`, `statusMessage`, `section`, `tag`, `source` / `open`, `executed`, `rejected`, `cancelled` |
| `orders.PositionsSnapshot` / `PositionRow` | `net`, `day`, `netTotalPnlPaise`, `dayTotalPnlPaise`, `asOfMs`, `stale` / `key`, `exchange`, `symbol`, `token`, `product`, `qty`, `multiplier` (Kite `multiplier`, 0 → 1), `avgPaise`, `ltpPaise`, `pnlPaise`, `netChangeBp`, `closed` |
| `orders.HoldingsSnapshot` / `HoldingRow` / `HoldingsSummary` | `rows`, `summary`, `asOfMs`, `stale` / `exchange`, `symbol`, `token`, `qty`, `avgPaise`, `ltpPaise`, `prevClosePaise`, `investedPaise`, `currentPaise`, `pnlPaise`, `netChangeBp`, `dayChangePaise`, `dayChangeBp` / `empty`, `currentValuePaise`, `investmentPaise`, `pnlPaise`, `pnlBp`, `dayChangePaise` |
| `service.DataStats` | `dataDir`, `appDbBytes`, `candleBytes`, `candleBudgetBytes`, `instrumentsImportedAtMs`, `backfillOpenJobs` |
| `service.BackupInfo` | `path`, `ok`, `reason`, `schema`, `supportedSchema`, `appVersion`, `createdAt` (manifest RFC3339 string), `counts` (`strategies`, `scanners`, `backtest_runs`, `deployments`, `watchlists`, `order_journal`), `includesCandles`, `sizeBytes` |
| `service.CalendarInfo` / `CalendarEntryDTO` / `PhaseDTO` / `CalendarEntryInput` | `version`, `coversUntil` (YYYY-MM-DD), `stale`, `entries` / `exchange`, `date` (YYYY-MM-DD), `type` (`holiday`\|`special`\|`trading`), `phases`, `source` (`user`\|`import`) / `kind`, `start`, `end` (HH:MM) / `exchange`, `date`, `type`, `phases` |
| `service.CrashInfo` | `path` (`""` = none), `atMs` |
| `service.AppInfo` / `service.Licence` | `version`, `build` (Go version + build tags), `schemaVersion`, `dataDir`, `licences` / `name`, `version`, `licence` |
| `service.StatusSnapshot` | `feed` (`ticker.FeedState`), `market` (`service.MarketStatus`), `clock` (`clock.Offset`) |
| `service.MarketStatus` | `exchange`, `phase` (`pre-open`\|`continuous`\|`closing`\|`post-closing`\|`closed`), `dayType` (`trading`\|`holiday`\|`special`\|`weekend`\|`inferred-non-trading`), `label`, `tooltip`, `atMs` |
| `service.JobProgress` | `jobId`, `kind` (`backup`\|`restore`\|`support_bundle`), `stage`, `done`, `total`, `finished`, `error`, `resultPath` |
| `service.Navigate` | `route` |
| `ticker.FeedState` / `clock.Offset` / `backfill.Progress` / `ratelimit.Usage` / `settings.Changed` / `kite.Order` | P0-2 / P0-1 shapes unchanged: `state`, `attempt`, `nextRetry` (RFC3339), `lastTick` (RFC3339), `connections`, `tokens` / `millis`, `source`, `samples`, `warn`, `skewed`, `updatedAt` (RFC3339) / `jobId`, `token`, `interval`, `done`, `failed`, `total`, `etaNs`, `state` / `at`, `paused`, `pauseReason`, `classes` / `key`, `value` / gokiteconnect snake_case (`order_id`, `status`, `tradingsymbol`, `transaction_type`, `filled_quantity`, …) |

Times are Unix ms `int64` fields named `…Ms`, except the P0-2 payloads that already carry `time.Time` (RFC3339 strings) and `BackupInfo.createdAt` (manifest string). Money is integer paise, ratios are basis points.

**Events** (`frontend/src/api/events.gen.ts` exports `EventNames` and `EventPayloads`; names equal the bus topics; `service.EventCatalogue()` lists them in this order):

| Event | Payload (TS) | Go topic | Emission |
|---|---|---|---|
| `session.state` | `service_SessionStatus` | `kite.TopicSessionState` | immediate |
| `feed.state` | `ticker_FeedState` | `ticker.TopicFeed` | latest / 100 ms |
| `tick.batch` | `service_TickDTO[]` | `ticker.TopicTicks` merged with the polled quotes of `Quotes.Watch` | merged by token / 100 ms |
| `candle.forming` | `service_CandleEventDTO[]` | `builder.TopicForming` | latest per (token, interval) / 100 ms |
| `candle.closed` | `service_CandleEventDTO` | `builder.TopicClosed` | immediate |
| `clock.offset` | `clock_Offset` | `clock.TopicOffset` | latest / 100 ms |
| `backfill.progress` | `backfill_Progress[]` | `backfill.TopicProgress` | latest per job / 100 ms |
| `limits.usage` | `ratelimit_Usage` | `ratelimit.TopicUsage` | latest / 100 ms |
| `orders.changed` | `orders_BookSnapshot` | `orders.TopicBook` | latest / 100 ms |
| `positions.changed` | `orders_PositionsSnapshot` | `orders.TopicPositions` | latest / 100 ms |
| `holdings.changed` | `orders_HoldingsSnapshot` | `orders.TopicHoldings` | latest / 100 ms |
| `order.update` | `kite_Order` (gokiteconnect snake_case JSON) | `kite.TopicOrderUpdate` | immediate |
| `watchlists.changed` | `service_WatchlistsState` | `service.TopicWatchlists` | immediate |
| `settings.changed` | `settings_Changed` (`key`, `value`) | `settings.Topic` | immediate |
| `market.status` | `service_MarketStatus` | `service.TopicMarketStatus` (NSE on the app clock; published at start and at every phase boundary, on change) | immediate |
| `job.progress` | `service_JobProgress` | `service.TopicJobProgress` (backup / restore / support bundle; a finished restore ends with `stage: "restart"`, then the app quits with exit code 3 and `main` re-execs) | immediate |
| `app.navigate` | `service_Navigate` | `service.TopicNavigate` (published by `App.SecondInstance` for `--view <route>`) | immediate |

Feed state names are P0-2's `stopped|connecting|live|reconnecting|offline`; "logged out" and "no data plan" are derived by the UI from `session.state`.

**Settings keys** (Go-declared; JSON values validated by `Settings.Set`):

| Key | Type / allowed | Default | Declared in |
|---|---|---|---|
| `log.level` | `"info"` \| `"debug"` | `"info"` | P0-1 `settings.LogLevel` |
| `backup.schedule.enabled` | bool | `false` | P0-1 `settings.BackupScheduleEnabled` |
| `backup.schedule.folder` | absolute path or `""` | `""` | P0-1 `settings.BackupScheduleFolder` |
| `backup.schedule.keep` | int 1–365 | `7` | P0-1 `settings.BackupScheduleKeep` |
| `backup.include_candles` | bool | `false` | P0-1 `settings.BackupIncludeCandles` |
| `kite.api_key` | hidden from `Settings.All`; `Session.SaveCredentials` only | `""` | Task 2 `kite.SettingAPIKey` |
| `kite.redirect_port` | hidden; `Session.SetRedirectPort` only (1024–65535) | `7420` | Task 2 `kite.SettingRedirectPort` |
| `watchlist.active` | watchlist id | `""` | Task 15 `service.SettingActiveWatchlist` |
| `appearance.theme` | `"light"` \| `"dark"` | `"light"` | Task 17 `settings.AppearanceTheme` |
| `appearance.follow_system` | bool | `false` | Task 17 `settings.AppearanceFollowSystem` |
| `appearance.sidebar_collapsed` | bool | `false` | Task 17 `settings.AppearanceSidebarCollapsed` |
| `ui.last_route` | route starting with `/` | `"/home"` | Task 17 `settings.UILastRoute` |
| `chart.default_timeframe` | `minute`, `3minute`, `5minute`, `10minute`, `15minute`, `30minute`, `60minute`, `day` | `"60minute"` | Task 17 `settings.ChartDefaultTimeframe` |
| `calendar.stale_toast_day` | `YYYY-MM-DD` or `""` | `""` | Task 17 `settings.CalendarStaleToastDay` |

UI state (`Settings.GetUiState/SetUiState`, table `window_state`, not settings): `chart.dashboard` (P0-5, JSON `{identifier, timeframe}`), `window.geometry` (Go `BeforeClose`, JSON `{"w","h","x","y","maximised"}`), `quotes.last` (Go, the last-quote cache), `backup.last_scheduled` (Go, `YYYY-MM-DD` of the last scheduled backup).

**Shortcut registry** (`settings.DefaultShortcuts()`, Task 28; ids and defaults exactly P0-4 A8):

| ID | Default | Scope | Label |
|---|---|---|---|
| `nav.1` … `nav.8` | `Alt+1` … `Alt+8` | `global` | `Go to Home`, `Go to Scalper`, `Go to Strategies`, `Go to Equity Builder`, `Go to Live Strategies`, `Go to Scanners`, `Go to New Scanner`, `Go to Scanner Alerts` |
| `toasts.focus` | `Alt+T` | `global` | `Focus toasts` |
| `search.open` | `Ctrl+K` | `global` | `Open search` |
| `help.open` | `F1` | `global` | `Open help` |
| `kill.switch` | `Ctrl+Shift+K` | `global` | `Kill switch` |
| `chart.fullView` | `F11` | `global` | `Full-view chart` |
| `chart.trendLine` | `Alt+T` | `chart` | `Draw trend line` |
| `order.new` | `Shift+T` | `chart` | `New order` |
| `builder.undo` / `builder.redo` | `Ctrl+Z` / `Ctrl+Shift+Z` | `builder` | `Undo` / `Redo` |
| `watchlist.moveUp` / `watchlist.moveDown` | `Alt+ArrowUp` / `Alt+ArrowDown` | `view` | `Move item up` / `Move item down` |

Binding grammar: `+`-joined modifiers `Ctrl`, `Alt`, `Shift`, `Meta` (canonical order) and one key (`settings.ParseBinding`). A rebind conflicts when another action has the same binding in the same scope or in `global` (`settings.Conflicts`); the shipped defaults keep `Alt+T` for both `toasts.focus` (global) and `chart.trendLine` (chart) by design — the shell resolves it in favour of the chart scope while focus is inside the chart (P0-4 Task 6).

---


<!-- generated task table: begin (plan_split.py) -->
## Task files

Each implementer reads this manifest plus its one task file. Sizes are approximate (characters / 4).
"Consumes tasks" lists earlier tasks named in the task's Consumes lines. Dispatch in number order
unless Global Constraints or the P0 index state a different execution order.

| # | Task | File | ~tokens | Consumes tasks |
|---|---|---|---|---|
| 1 | Session primitives and the 06:00 IST expiry rule | [`tasks/01-session-primitives-and-the-06-00.md`](tasks/01-session-primitives-and-the-06-00.md) | 1.9k | — |
| 2 | Settings key registry, credential validation, settings keys and redirect URL guidance | [`tasks/02-settings-key-registry-credential-validation-settings.md`](tasks/02-settings-key-registry-credential-validation-settings.md) | 3.4k | — |
| 3 | Redirect parsing for the callback and the paste fallback | [`tasks/03-redirect-parsing-for-the-callback-and.md`](tasks/03-redirect-parsing-for-the-callback-and.md) | 1.2k | 1 |
| 4 | Loopback callback listener | [`tasks/04-loopback-callback-listener.md`](tasks/04-loopback-callback-listener.md) | 1.5k | — |
| 5 | `SessionsRepo` over the `sessions` table | [`tasks/05-sessionsrepo-over-the-sessions-table.md`](tasks/05-sessionsrepo-over-the-sessions-table.md) | 2.0k | — |
| 6 | `Session` core — dependencies, credentials, startup restore, risk acknowledgement | [`tasks/06-session-core-dependencies-credentials-startup-restore.md`](tasks/06-session-core-dependencies-credentials-startup-restore.md) | 12.3k | — |
| 7 | Login flow — loopback with state, paste fallback, token exchange | [`tasks/07-login-flow-loopback-with-state-paste.md`](tasks/07-login-flow-loopback-with-state-paste.md) | 5.3k | — |
| 8 | Session lifecycle — 06:00 expiry watcher and `TokenException` handling | [`tasks/08-session-lifecycle-06-00-expiry-watcher.md`](tasks/08-session-lifecycle-06-00-expiry-watcher.md) | 1.4k | 6, 7 |
| 9 | Capability probe and degraded-mode detection | [`tasks/09-capability-probe-and-degraded-mode-detection.md`](tasks/09-capability-probe-and-degraded-mode-detection.md) | 2.8k | — |
| 10 | Logout, remove credentials, change redirect port | [`tasks/10-logout-remove-credentials-change-redirect-port.md`](tasks/10-logout-remove-credentials-change-redirect-port.md) | 2.0k | — |
| 11 | Profile with redaction | [`tasks/11-profile-with-redaction.md`](tasks/11-profile-with-redaction.md) | 1.5k | — |
| 12 | Order book, positions and holdings builders (pure) | [`tasks/12-order-book-positions-and-holdings-builders.md`](tasks/12-order-book-positions-and-holdings-builders.md) | 4.9k | — |
| 13 | `WatchlistsRepo` | [`tasks/13-watchlistsrepo.md`](tasks/13-watchlistsrepo.md) | 2.3k | 5 |
| 14 | `service` core — dependencies, `ErrRefused`, session façade, lifecycle supervisor | [`tasks/14-service-core-dependencies-errrefused-session-facade.md`](tasks/14-service-core-dependencies-errrefused-session-facade.md) | 5.8k | 5, 12, 13 |
| 15 | Watchlist façade methods | [`tasks/15-watchlist-facade-methods.md`](tasks/15-watchlist-facade-methods.md) | 3.7k | 2, 13 |
| 16 | Chart data (trading-day ranges, manual refresh, live chart subscriptions) and instrument lookup | [`tasks/16-chart-data-and-instrument-lookup.md`](tasks/16-chart-data-and-instrument-lookup.md) | 9.4k | 6, 14 |
| 17 | Settings façade and the UI settings keys | [`tasks/17-settings-facade-and-the-ui-settings.md`](tasks/17-settings-facade-and-the-ui-settings.md) | 2.6k | 2 |
| 18 | `OrdersCacheRepo` | [`tasks/18-orderscacherepo.md`](tasks/18-orderscacherepo.md) | 1.6k | 5 |
| 19 | Read-only `orders.Sync` | [`tasks/19-read-only-orders-sync.md`](tasks/19-read-only-orders-sync.md) | 7.1k | 6, 9 |
| 20 | Portfolio façade methods | [`tasks/20-portfolio-facade-methods.md`](tasks/20-portfolio-facade-methods.md) | 0.8k | 12, 14 |
| 21 | Event catalogue, coalescing sources, market-status clock and the TypeScript event generator | [`tasks/21-event-catalogue-coalescing-sources-market-status.md`](tasks/21-event-catalogue-coalescing-sources-market-status.md) | 8.2k | 1, 15, 16, 19 |
| 22 | Wails app — bound API structs, lifecycle, error format, options, generated bindings | [`tasks/22-wails-app-bound-api-structs-lifecycle.md`](tasks/22-wails-app-bound-api-structs-lifecycle.md) | 12.5k | — |
| 23 | Single instance (D-Bus focus on P0-1's lock), std-stream capture, platform build and `cmd/tradeloom` wiring | [`tasks/23-single-instance-std-stream-capture-platform.md`](tasks/23-single-instance-std-stream-capture-platform.md) | 8.4k | 6, 9, 22 |
| 24 | Mock-Kite integration — proof of zero order-write calls (G-0) | [`tasks/24-mock-kite-integration-proof-of-zero.md`](tasks/24-mock-kite-integration-proof-of-zero.md) | 3.3k | 5, 14, 19 |
| 25 | Spike S0-1 — real Kite login procedure and `docs/spec/kite-login.md` | [`tasks/25-spike-s0-1-real-kite-login.md`](tasks/25-spike-s0-1-real-kite-login.md) | 2.0k | — |
| 26 | Spike S0-8 — single instance end to end and the D-Bus notifications probe | [`tasks/26-spike-s0-8-single-instance-end.md`](tasks/26-spike-s0-8-single-instance-end.md) | 1.8k | 23 |
| 27 | Secret storage setup, unlock and migration (FR-ACCT-013) | [`tasks/27-secret-storage-setup-unlock-and-migration.md`](tasks/27-secret-storage-setup-unlock-and-migration.md) | 5.5k | 6, 14 |
| 28 | Shortcut registry, UI state and their façade (FR-SHELL-019) | [`tasks/28-shortcut-registry-ui-state-and-their.md`](tasks/28-shortcut-registry-ui-state-and-their.md) | 5.6k | 5, 14 |
| 29 | `Quotes.Watch` — per-consumer live sets, polled fallback and the persisted last-quote cache | [`tasks/29-quotes-watch-per-consumer-live-sets.md`](tasks/29-quotes-watch-per-consumer-live-sets.md) | 4.9k | 6, 9, 14, 21, 28 |
| 30 | Data façade — stats, clear candles, instrument refresh, backup/restore jobs and the daily backup (FR-DATA-002/005) | [`tasks/30-data-facade-stats-clear-candles-instrument.md`](tasks/30-data-facade-stats-clear-candles-instrument.md) | 7.2k | 21, 28 |
| 31 | Calendar façade — info, CSV import, upsert and remove of overrides (FR-DATA-001 UI backend) | [`tasks/31-calendar-facade-info-csv-import-upsert.md`](tasks/31-calendar-facade-info-csv-import-upsert.md) | 3.6k | 16, 30 |
| 32 | Diagnostics façade — logs folder, support bundle, crash reports (FR-SHELL-018) | [`tasks/32-diagnostics-facade-logs-folder-support-bundle.md`](tasks/32-diagnostics-facade-logs-folder-support-bundle.md) | 2.9k | 30, 31 |
| 33 | System façade (app info with licences, status snapshot, feed reconnect), regenerated bindings and the contract test | [`tasks/33-system-facade-regenerated-bindings-and-the.md`](tasks/33-system-facade-regenerated-bindings-and-the.md) | 3.6k | 6, 21, 22 |

Coverage and self-review: [`APPENDIX.md`](APPENDIX.md).
<!-- generated task table: end -->