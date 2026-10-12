> **P0-3 Session & Service Layer Implementation Plan — Task 23.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 23: Single instance (D-Bus focus on P0-1's lock), std-stream capture, platform build and `cmd/tradeloom` wiring

**Files:**
- Create: `internal/app/singleinstance.go`, `internal/app/stdstreams.go`, `cmd/tradeloom/wire_p03.go`, `cmd/tradeloom/exit_test.go`
- Modify: `internal/app/options.go` (remove the Task 22 stub), `cmd/tradeloom/main.go` (P0-1 Task 15: `run` is kept and extended), `.go-arch-lint.yml` (P0-1 Task 16: full replacement listing every P0-2 and P0-3 component)
- Test: `internal/app/singleinstance_test.go`, `internal/app/stdstreams_test.go`, `cmd/tradeloom/exit_test.go`; P0-1's `cmd/tradeloom/main_test.go` (`TestRun_SecondInstanceExits`, `TestRun_BadFlag`) keeps passing unchanged

**Interfaces:**
- Consumes:
  - P0-1 `app.Start`/`app.Boot`/`app.ExitMessage`, `store.ErrLocked` (the flock inside `app.Start` is the authoritative single-instance lock: `store.AcquireLock`, last-exit marker, `Boot.CrashedLastTime`), `store.ResolveDataDir`, `store.EnvDataDir`, `clock.SystemNow`, `frontend.Assets`, `logging.Go`, `(*store.DB).CalendarOverrides`, `Boot.Layout.{Root, CandlesDB, SecretsFile}`, `Boot.Secrets` (`secrets.Detection`), `Boot.CrashReports`, `secrets.NewKeyring`/`NewNone`, `secrets.ProbeKeyringWithin`, `secrets.ProbeTimeout`, `store.NewShortcutsRepo`, `store.NewWindowStateRepo` (Task 28); `service.NewSecretSwitch` (Task 27); `service.Deps` fields of Tasks 16, 27–33; `licences.Notices` (Task 33); `service.Navigate` (Task 21).
  - P0-2 constructors exactly as in "Consumes": `clock.New(clock.Options{Bus, Log}) *clock.Real` (`ObserveHTTPDate`, implements `ticker.TickObserver`), `ratelimit.New(clk, ratelimit.Options{Bus, Log})` + `Run`, `kite.New(kite.Options{…})`, `calendar.NewDefault()`, `calendar.FromRecord`, `(*calendar.Service).SetOverrides`, `sqlite.Open(ctx, path, sqlite.Options{Clock, Calendar})` (package `internal/candles/sqlite`, imported as `candlesql`), `instruments.NewMaster(clk)`, `instruments.NewRefresher(instruments.Options{…})` + `Load`/`Run`, `backfill.New(backfill.Options{…})` + `Start`/`WaitStopped`, `builder.New(builder.Options{…})` + `Run(ctx, ticks)`, `ticker.Options`; Task 6 `kite.NewAllowListTransport`, `kite.AllowedHosts`, `kite.SessionErrorRelay`, `feed.New`; Task 9 `kite.NiftyIndexToken`; Task 22 `app.New(app.Deps{…})`, `app.Options(a, assets, uniqueID, smoke)`, `(*App).RestartRequested`.
  - `godbus/dbus/v5` v5.2.3 (verified): `ConnectSessionBus(...ConnOption) (*Conn, error)`, `(*Conn).Object(dest string, path ObjectPath) BusObject`, `BusObject.Call(method string, flags Flags, args ...any) *Call`, `(*Call).Store(...any) error`, `(*Conn).Export`, `(*Conn).RequestName`, `NameFlagDoNotQueue`.
  - Wails v2.16.0's Linux single-instance protocol (`internal/frontend/desktop/linux/single_instance.go`): bus name `org.<id>.SingleInstance`, object path `/org/<id>/SingleInstance`, method `<bus name>.SendMessage(string)` carrying JSON `options.SecondInstanceData{Args, WorkingDirectory}`, where `id = "wails_app_" + uniqueID with "-" and "." → "_"`.
- Produces:
  ```go
  // package app
  func UniqueID(dataDir string) string // "tradeloom-" + sha1(abs dataDir)[:12]
  func ForwardToRunningInstance(uniqueID string, args []string, workDir string) error
  func (a *App) SecondInstance(data options.SecondInstanceData) // focus; a Kite redirect URL → CompleteLogin; "--view <route>" → service.Navigate
  func CaptureStdStreams(log *slog.Logger) (restore func())
  // package main (cmd/tradeloom)
  const exitRestart = 3
  func run(args []string, getenv func(string) string, stderr io.Writer) int // P0-1's run, extended: ErrLocked → forward; exit 3 = restart requested
  type p03Platform struct{…}                                               // every P0-2 component, built once per process; Secrets *service.SecretSwitch
  func buildPlatform(ctx context.Context, boot *app.Boot) (*p03Platform, error)
  func secretSwitch(d secrets.Detection) *service.SecretSwitch              // keyring → usable; file → locked until Unlock; choose → NewNone until setup
  func (p *p03Platform) Start(ctx context.Context) error; func (p *p03Platform) Stop(ctx context.Context) // App.Deps.StartPlatform / StopPlatform (Stop is idempotent)
  func (p *p03Platform) CloseForRestore() error                            // service.Deps.CloseForRestore: Stop + close app.db before store.Restore
  func wireP03(p *p03Platform) *app.App
  func finalExit(restart bool) int
  ```

**Why the second instance does not just call `wails.Run`:** in v2.16.0, `Frontend.Run` launches `OnStartup` on a goroutine *before* `SetupSingleInstance` (`internal/frontend/desktop/linux/frontend.go:295-303`). A second process calling `wails.Run` would start our startup sequence and only then `os.Exit(1)`. It also falls through to opening a second window when no session bus exists. So `run` keeps P0-1's order: `app.Start` takes the flock first (authoritative, ARCH §3.7, before logging and `app.db`). When it returns `store.ErrLocked`, `run` speaks Wails' D-Bus protocol directly (`ForwardToRunningInstance`), so the first instance's `OnSecondInstanceLaunch` focuses the window and receives the arguments (a Kite redirect URL is routed to the paste login). The second process then exits 0. Without a session bus (or no listener) it prints `Tradeloom is already running (data dir …): <reason>` and exits 1 — exactly what P0-1's `TestRun_SecondInstanceExits` asserts.

**Why capture std streams:** gokiteconnect's HTTP helper logs transport failures with `log.New(os.Stdout, …)` as `Request failed: <err>` (`http.go` `DoRaw`). For `DELETE /session/token` that error text contains the URL with `access_token` in the query. `CaptureStdStreams` routes stdout/stderr lines through the redacting slog logger and strips URL query strings first. It must run before the first `kiteconnect.New`, because the logger captures `os.Stdout` at construction.

- [ ] **Step 1: Write the failing tests**

```go
// internal/app/singleinstance_test.go
package app

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v2/pkg/options"

	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

// The lock itself (FR-ACCT-004 AC3, authoritative half) is P0-1's store.AcquireLock inside app.Start:
// TestStart_FirstRunCreatesEverything (boot_test.go) and TestRun_SecondInstanceExits (cmd/tradeloom).

func TestUniqueIDAndDBusNames(t *testing.T) {
	a, b := UniqueID("/home/u/.local/share/tradeloom"), UniqueID("/tmp/other")
	if !strings.HasPrefix(a, "tradeloom-") || len(a) != len("tradeloom-")+12 || a == b || a != UniqueID("/home/u/.local/share/tradeloom") {
		t.Fatalf("%s %s", a, b)
	}
	name, path := dbusNames("tradeloom-0123456789ab")
	if name != "org.wails_app_tradeloom_0123456789ab.SingleInstance" || path != "/org/wails_app_tradeloom_0123456789ab/SingleInstance" {
		t.Fatalf("%s %s", name, path)
	}
}

type recv struct{ ch chan string }

func (r recv) SendMessage(m string) *dbus.Error { r.ch <- m; return nil }

// Runs where a session bus exists (CI: `dbus-run-session -- go test …`, Task 26).
func TestForwardToRunningInstance_DeliversArgs(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus")
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Skip("session bus unreachable:", err)
	}
	defer conn.Close()
	uid := UniqueID(t.TempDir())
	name, path := dbusNames(uid)
	r := recv{ch: make(chan string, 1)}
	if err := conn.Export(r, path, name); err != nil {
		t.Fatal(err)
	}
	if reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue); err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("request name %v %v", reply, err)
	}
	args := []string{"http://127.0.0.1:7420/kite/callback?request_token=rtok12345678&status=success"}
	if err := ForwardToRunningInstance(uid, args, "/work"); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-r.ch:
		var d options.SecondInstanceData
		if err := json.Unmarshal([]byte(m), &d); err != nil || d.Args[0] != args[0] || d.WorkingDirectory != "/work" {
			t.Fatalf("%q %v", m, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("message not delivered")
	}
}

func TestForwardToRunningInstance_NoBusFails(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/tradeloom-test-bus")
	if err := ForwardToRunningInstance("tradeloom-0123456789ab", nil, "/"); err == nil {
		t.Fatal("expected an error without a session bus")
	}
}

// Review Focus 5: a redirect URL passed to a second launch completes the login in the running instance.
func TestSecondInstance_RoutesRedirectToLogin(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	u := "http://127.0.0.1:7420/kite/callback?request_token=rtok12345678&status=success"
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--verbose", u}})
	eventually(t, "login routed", func() bool { c := fs.Completed(); return len(c) == 1 && c[0] == u })
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--verbose"}})
	if fr.focusCount() != 2 || len(fs.Completed()) != 1 {
		t.Fatalf("focus %d completed %v", fr.focusCount(), fs.Completed())
	}
}

// A second launch with "--view <route>" (desktop shortcuts, P0-4 router) navigates the running window.
func TestSecondInstance_NavigatesToView(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--view", "/settings/data"}})
	eventually(t, "navigate event", func() bool { return fr.sawEvent(service.EvAppNavigate) })
	fr.mu.Lock()
	nav := fr.payloads[service.EvAppNavigate].(service.Navigate)
	fr.mu.Unlock()
	if nav.Route != "/settings/data" || fr.focusCount() != 1 {
		t.Fatalf("route %q focus %d", nav.Route, fr.focusCount())
	}
	a.SecondInstance(options.SecondInstanceData{Args: []string{"--view"}}) // no route: focus only
	if fr.focusCount() != 2 {
		t.Fatalf("focus %d", fr.focusCount())
	}
}
```

```go
// internal/app/stdstreams_test.go
package app

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
)

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}
func (l *lockedBuf) String() string { l.mu.Lock(); defer l.mu.Unlock(); return l.b.String() }

func TestCaptureStdStreams_ScrubsQueryStrings(t *testing.T) {
	buf := &lockedBuf{}
	restore := CaptureStdStreams(slog.New(slog.NewJSONHandler(buf, nil)))
	fmt.Fprintln(os.Stdout, `base.HTTP: Request failed: Delete "https://api.kite.trade/session/token?api_key=k&access_token=SECRETTOKEN123": dial tcp: lookup api.kite.trade: no such host`)
	restore()
	out := buf.String()
	if strings.Contains(out, "SECRETTOKEN123") || !strings.Contains(out, "session/token?[redacted]") {
		t.Fatalf("log: %s", out)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/app/ -run 'TestUniqueID|TestForwardToRunningInstance|TestSecondInstance|TestCaptureStdStreams' -v`
Expected: FAIL to compile with `undefined: UniqueID`, `undefined: dbusNames`, `undefined: CaptureStdStreams`.

- [ ] **Step 3: Write the implementation**

Delete the temporary `SecondInstance` stub (and its comment) from the end of `internal/app/options.go`, then create:

```go
// internal/app/singleinstance.go
package app

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v2/pkg/options"

	"tradeloom/internal/logging"
)

// The process lock is P0-1's store.AcquireLock (taken inside app.Start, with the last-exit marker that
// feeds Boot.CrashedLastTime). This file only adds what sits on top of it: the Wails single-instance id
// and the D-Bus hand-off of a second launch's arguments.

// UniqueID scopes Wails' SingleInstanceLock to one data directory (ARCH §3.7).
func UniqueID(dataDir string) string {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		abs = dataDir
	}
	sum := sha1.Sum([]byte(abs))
	return "tradeloom-" + hex.EncodeToString(sum[:])[:12]
}

// dbusNames mirrors wails v2.16.0 internal/frontend/desktop/linux/single_instance.go.
func dbusNames(uniqueID string) (string, dbus.ObjectPath) {
	id := "wails_app_" + strings.ReplaceAll(strings.ReplaceAll(uniqueID, "-", "_"), ".", "_")
	return "org." + id + ".SingleInstance", dbus.ObjectPath("/org/" + id + "/SingleInstance")
}

// ForwardToRunningInstance delivers this launch's arguments to the running instance's OnSecondInstanceLaunch.
func ForwardToRunningInstance(uniqueID string, args []string, workDir string) error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("session bus: %w", err)
	}
	defer conn.Close()
	name, path := dbusNames(uniqueID)
	payload, err := json.Marshal(options.SecondInstanceData{Args: args, WorkingDirectory: workDir})
	if err != nil {
		return err
	}
	return conn.Object(name, path).Call(name+".SendMessage", 0, string(payload)).Store()
}

// SecondInstance is options.SingleInstanceLock.OnSecondInstanceLaunch: focus; route a Kite redirect to the paste
// path; "--view <route>" asks the UI router to show that route (app.navigate).
func (a *App) SecondInstance(data options.SecondInstanceData) {
	a.runtime().Focus()
	for i, arg := range data.Args {
		if strings.Contains(arg, "request_token=") {
			ctx := a.context()
			a.d.Log.Info("second launch carried a Kite redirect; completing the login")
			logging.Go("app.secondinstance.login", func() {
				if _, err := a.d.Service.CompleteLogin(ctx, arg); err != nil {
					a.d.Log.Warn("login from a second launch failed", "err", err)
				}
			})
			return
		}
		if arg == "--view" && i+1 < len(data.Args) {
			a.d.Log.Info("second launch asked for a view", "route", data.Args[i+1])
			a.d.Service.Navigate(data.Args[i+1])
			return
		}
	}
	a.d.Log.Info("second launch focused the existing window", "args", len(data.Args))
}
```

```go
// internal/app/stdstreams.go
package app

import (
	"bufio"
	"log/slog"
	"os"
	"regexp"

	"tradeloom/internal/logging"
)

var urlQuery = regexp.MustCompile(`\?[^\s"']*`)

// CaptureStdStreams sends library output on stdout/stderr through the (redacting) logger, with URL query
// strings removed. Call it before the first kiteconnect client is created.
func CaptureStdStreams(log *slog.Logger) (restore func()) {
	r, w, err := os.Pipe()
	if err != nil {
		log.Warn("capturing stdout/stderr failed", "err", err)
		return func() {}
	}
	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	done := make(chan struct{})
	logging.Go("app.stdstreams", func() {
		defer close(done)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			log.Warn("library output", "line", urlQuery.ReplaceAllString(sc.Text(), "?[redacted]"))
		}
	})
	return func() {
		os.Stdout, os.Stderr = origOut, origErr
		_ = w.Close()
		<-done
		_ = r.Close()
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/app/ -v`
Expected: PASS (incl. `TestSecondInstance_RoutesRedirectToLogin`, `TestSecondInstance_NavigatesToView`, `TestCaptureStdStreams_ScrubsQueryStrings`). `TestForwardToRunningInstance_DeliversArgs` reports SKIP outside a D-Bus session; under `dbus-run-session -- go test …` it reports PASS.

- [ ] **Step 5: Write the platform build, the wiring, the extended `main.go` and the arch-lint file**

`cmd/tradeloom/wire_p03.go` builds P0-2 on top of P0-1's `Boot` in ARCHITECTURE §3.2 order and stops it in §3.3 order:

```go
// cmd/tradeloom/wire_p03.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"tradeloom/internal/app"
	"tradeloom/internal/calendar"
	"tradeloom/internal/candles/backfill"
	"tradeloom/internal/candles/builder"
	candlesql "tradeloom/internal/candles/sqlite"
	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/feed"
	"tradeloom/internal/kite/ratelimit"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/licences"
	"tradeloom/internal/logging"
	"tradeloom/internal/orders"
	"tradeloom/internal/secrets"
	"tradeloom/internal/service"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

// p03Platform is every P0-2 component, built once per process by buildPlatform on top of P0-1's Boot.
type p03Platform struct {
	Boot      *app.Boot
	Secrets   *service.SecretSwitch
	Clock     *clock.Real
	Transport http.RoundTripper
	Relay     *kite.SessionErrorRelay
	Kite      *kite.Client
	Limiter   *ratelimit.Limiter
	Calendar  *calendar.Service
	Candles   *candlesql.Store
	Master    *instruments.Master
	Refresher *instruments.Refresher
	Backfill  *backfill.Backfill
	Feed      *feed.Feed
	Builder   *builder.Builder
	Log       *slog.Logger

	cancel   context.CancelFunc // platform context (limiter, backfill, refresher, builder); set by Start
	stopOnce sync.Once
}

// buildPlatform wires P0-2 in ARCHITECTURE §3.2 order: clock → limiter → allow-list transport → Kite client
// (session errors through the relay) → calendar + persisted overrides → candle store → instrument master
// (loaded from app.db) → backfill → feed → candle builder → secrets backend. Nothing runs yet: Start does.
func buildPlatform(ctx context.Context, boot *app.Boot) (*p03Platform, error) {
	log := boot.Log.Logger
	p := &p03Platform{Boot: boot, Log: log}
	p.Clock = clock.New(clock.Options{Bus: boot.Bus, Log: log.With("component", "clock")})
	p.Limiter = ratelimit.New(p.Clock, ratelimit.Options{Bus: boot.Bus, Log: log.With("component", "ratelimit")})
	p.Transport = kite.NewAllowListTransport(http.DefaultTransport, kite.AllowedHosts)
	p.Relay = &kite.SessionErrorRelay{}
	p.Kite = kite.New(kite.Options{
		APIKey:         settings.Get(boot.Settings, kite.SettingAPIKey),
		Limiter:        p.Limiter,
		Clock:          p.Clock,
		DateSink:       p.Clock.ObserveHTTPDate,
		OnSessionError: p.Relay.Handle,
		Transport:      p.Transport,
		Log:            log.With("component", "kite"),
	})
	cal, err := calendar.NewDefault()
	if err != nil {
		return nil, fmt.Errorf("calendar: %w", err)
	}
	rows, err := boot.DB.CalendarOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("calendar overrides: %w", err)
	}
	overrides := make([]calendar.Entry, 0, len(rows))
	for _, r := range rows {
		e, err := calendar.FromRecord(calendar.Record{Exchange: r.Exchange, Date: r.Date, Type: r.Type, Phases: r.Phases, Source: r.Source})
		if err != nil {
			log.Warn("calendar.override_skipped", "component", "calendar", "exchange", r.Exchange, "date", r.Date, "err", err)
			continue
		}
		overrides = append(overrides, e)
	}
	cal.SetOverrides(overrides)
	p.Calendar = cal
	cstore, err := candlesql.Open(ctx, boot.Layout.CandlesDB, candlesql.Options{Clock: p.Clock, Calendar: cal})
	if err != nil {
		return nil, fmt.Errorf("candle store: %w", err)
	}
	p.Candles = cstore
	p.Master = instruments.NewMaster(p.Clock)
	p.Refresher = instruments.NewRefresher(instruments.Options{
		Client: p.Kite, Master: p.Master, DB: boot.DB, Calendar: cal, Clock: p.Clock, Bus: boot.Bus,
		DataDir: boot.Layout.Root, Log: log.With("component", "instruments"),
	})
	if err := p.Refresher.Load(ctx); err != nil {
		log.Warn("instruments.load_failed", "component", "instruments", "err", err) // the daily refresh retries (FR-WATCH-008 AC4)
	}
	p.Backfill = backfill.New(backfill.Options{Store: cstore, Jobs: cstore, Fetcher: p.Kite, Clock: p.Clock, Bus: boot.Bus, Log: log.With("component", "backfill")})
	p.Feed = feed.New(feed.Options{
		Ticker: ticker.Options{Clock: p.Clock, Bus: boot.Bus, Log: log.With("component", "ticker"), ClockRef: kite.NiftyIndexToken, Observer: p.Clock},
		Log:    log.With("component", "feed"),
	})
	p.Builder = builder.New(builder.Options{Clock: p.Clock, Calendar: cal, Store: cstore, Bus: boot.Bus, ExchangeOf: p.Master.ExchangeOf, Log: log.With("component", "builder")})
	p.Secrets = secretSwitch(boot.Secrets)
	return p, nil
}

// secretSwitch maps P0-1's detection to Task 27's runtime backend (FR-ACCT-013): the keyring is usable at
// once; an existing secrets.enc starts locked until Session.Unlock opens it; "choose" starts as NewNone
// (refuses writes with secrets.ErrStorageRefused) until the user picks the encrypted file or no storage.
func secretSwitch(d secrets.Detection) *service.SecretSwitch {
	switch d.Mode {
	case secrets.ModeKeyring:
		return service.NewSecretSwitch(secrets.NewKeyring(), false)
	case secrets.ModeFile:
		return service.NewSecretSwitch(secrets.NewNone(), true)
	default:
		return service.NewSecretSwitch(secrets.NewNone(), false)
	}
}

// Start runs the process-wide P0-2 goroutines (App.Deps.StartPlatform, called from App.Startup).
func (p *p03Platform) Start(ctx context.Context) error {
	pctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	logging.Go("ratelimit", func() { p.Limiter.Run(pctx) })
	if err := p.Backfill.Start(pctx); err != nil {
		cancel()
		return fmt.Errorf("backfill: %w", err)
	}
	logging.Go("instruments.refresher", func() { p.Refresher.Run(pctx) })
	logging.Go("candles.builder", func() { p.Builder.Run(pctx, p.Feed.Ticks()) })
	return nil
}

// Stop is ARCHITECTURE §3.3: stop the feed, cancel the platform context, let the backfill drain, close the
// candle store. boot.Shutdown (bus, app.db, exit marker, lock, log) runs afterwards in run(). Stop is
// idempotent: a restore closes the platform early (CloseForRestore) and App.Shutdown calls it again.
func (p *p03Platform) Stop(context.Context) {
	p.stopOnce.Do(func() {
		p.Feed.Stop()
		if p.cancel != nil {
			p.cancel()
		}
		p.Backfill.WaitStopped()
		if err := p.Candles.Close(); err != nil {
			p.Log.Warn("candles.close_failed", "component", "candles", "err", err)
		}
	})
}

// CloseForRestore stops the platform and closes app.db so store.Restore can replace the data directory
// (service.Deps.CloseForRestore, Task 30). The process lock (Boot.Lock) stays held; boot.Shutdown's later
// DB.Close returns nil on an already closed *sql.DB.
func (p *p03Platform) CloseForRestore() error {
	p.Stop(context.Background())
	return p.Boot.DB.Close()
}

// wireP03 builds this plan's components on the platform and returns the Wails App.
func wireP03(p *p03Platform) *app.App {
	boot := p.Boot
	sess := kite.NewSession(kite.SessionDeps{
		Settings: boot.Settings, Secrets: p.Secrets, Repo: store.NewSessionsRepo(boot.DB), Clock: p.Clock, Bus: boot.Bus,
		Auth:     kite.NewAuth(&http.Client{Timeout: 10 * time.Second, Transport: p.Transport}, ""),
		Profiles: p.Kite, Tokens: p.Kite, Limiter: p.Limiter, Feed: p.Feed,
		Probe:    kite.NewCapabilityProbe(p.Kite, p.Clock, nil, p.Log.With("component", "kite.probe")),
		Log:      p.Log.With("component", "kite.session"),
	})
	p.Relay.Set(sess.HandleTokenError)
	portfolio := orders.NewSync(orders.SyncDeps{Read: p.Kite, Cache: store.NewOrdersCacheRepo(boot.DB), Bus: boot.Bus, Clock: p.Clock,
		Live: p.Feed, Log: p.Log.With("component", "orders.sync")})
	svc := service.New(service.Deps{
		// Task 14
		Session: sess, Orders: portfolio, Settings: boot.Settings, Watchlists: store.NewWatchlistsRepo(boot.DB),
		Candles: p.Candles, Backfill: p.Backfill, Instruments: p.Master, Live: p.Feed,
		Clock: p.Clock, Bus: boot.Bus, Log: p.Log.With("component", "service"),
		// Task 16
		Calendar: p.Calendar, Historical: p.Kite, CandleWriter: p.Candles, Tracker: p.Builder,
		// Task 27
		Secrets: p.Secrets, SecretsFile: boot.Layout.SecretsFile, Detection: boot.Secrets,
		KeyringProbe: func() error { return secrets.ProbeKeyringWithin(secrets.ProbeTimeout) }, NewKeyring: secrets.NewKeyring,
		// Task 28
		Shortcuts: store.NewShortcutsRepo(boot.DB), UiState: store.NewWindowStateRepo(boot.DB),
		// Task 29
		Quotes: p.Kite,
		// Task 30
		DB: boot.DB, Layout: boot.Layout, AppVersion: app.Version, CandleAdmin: p.Candles, Refresher: p.Refresher,
		ImportedAt: p.Master.ImportedAt, CloseForRestore: p.CloseForRestore,
		// Task 32
		CrashReports: boot.CrashReports,
		// Task 33
		FeedCtl: p.Feed, Licences: licences.Notices,
	})
	return app.New(app.Deps{
		Boot: boot, Service: svc, Log: p.Log.With("component", "app"), StartPlatform: p.Start,
		StopPlatform: func(ctx context.Context) {
			sess.Close()
			p.Stop(ctx)
		},
	})
}
```

`cmd/tradeloom/main.go` — P0-1 Task 15's file, with `run` extended (full replacement; P0-1's `main_test.go` keeps passing):

```go
// Command tradeloom is the desktop application (the only package besides internal/app that imports Wails).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/wailsapp/wails/v2"

	"tradeloom/frontend"
	"tradeloom/internal/app"
	"tradeloom/internal/clock"
	"tradeloom/internal/store"
)

// exitRestart asks main to re-exec the binary (a finished restore, FR-DATA-005).
const exitRestart = 3

func main() {
	code := run(os.Args[1:], os.Getenv, os.Stderr)
	if code == exitRestart {
		exe, err := os.Executable()
		if err == nil {
			err = syscall.Exec(exe, os.Args, os.Environ()) // returns only on failure
		}
		fmt.Fprintln(os.Stderr, "tradeloom: restart failed:", err)
		code = 1
	}
	os.Exit(code)
}

// run returns the process exit code: 0 ok (also: this launch was handed to the running instance),
// 1 startup refused (already running with no way to reach the first instance, newer schema, damaged
// database …), 2 bad flags, 3 restart requested.
func run(args []string, getenv func(string) string, stderr io.Writer) int {
	fl := flag.NewFlagSet("tradeloom", flag.ContinueOnError)
	fl.SetOutput(stderr)
	smoke := fl.Bool("smoke", false, "open the window, run startup and migrations, then quit with status 0 (install tests)")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	home, _ := os.UserHomeDir()
	dataDir, err := store.ResolveDataDir(getenv, home)
	if err != nil {
		fmt.Fprintln(stderr, "tradeloom:", err)
		return 1
	}
	boot, err := app.Start(context.Background(), app.BootOptions{
		DataDir: dataDir, AppVersion: app.Version, PID: os.Getpid(), Now: clock.SystemNow,
	})
	if errors.Is(err, store.ErrLocked) {
		// ARCH §3.7: the flock inside app.Start is authoritative and was taken before any log or app.db
		// access. Hand the arguments (deep link, focus) to the running instance over Wails' D-Bus protocol.
		wd, _ := os.Getwd()
		if ferr := app.ForwardToRunningInstance(app.UniqueID(dataDir), args, wd); ferr != nil {
			fmt.Fprintf(stderr, "%s: %v\n", app.ExitMessage(dataDir, err), ferr)
			return 1
		}
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, app.ExitMessage(dataDir, err))
		return 1
	}
	var once sync.Once
	shutdown := func() { once.Do(func() { boot.Shutdown() }) }
	defer shutdown()
	defer app.CaptureStdStreams(boot.Log.Logger)() // restored (LIFO) before boot.Shutdown closes the log

	platform, err := buildPlatform(context.Background(), boot)
	if err != nil {
		boot.Log.Error("platform.build_failed", "component", "app", "err", err)
		fmt.Fprintln(stderr, "Tradeloom could not start:", err)
		return 1
	}
	a := wireP03(platform)
	if err := wails.Run(app.Options(a, frontend.Assets(), app.UniqueID(dataDir), *smoke)); err != nil {
		boot.Log.Error("wails.exited", "component", "app", "err", err)
		fmt.Fprintln(stderr, "tradeloom:", err)
		return 1
	}
	return finalExit(a.RestartRequested())
}

// finalExit maps a clean window exit to the process exit code.
func finalExit(restart bool) int {
	if restart {
		return exitRestart
	}
	return 0
}
```

```go
// cmd/tradeloom/exit_test.go
package main

import "testing"

// A finished restore asks for a restart: run returns 3 and main re-execs the binary.
func TestRun_RestartExitCode(t *testing.T) {
	if got := finalExit(false); got != 0 {
		t.Fatalf("plain exit = %d", got)
	}
	if got := finalExit(true); got != exitRestart || exitRestart != 3 {
		t.Fatalf("restart exit = %d", got)
	}
}
```

`.go-arch-lint.yml` — full replacement of P0-1 Task 16's file. P0-2 added no components, so every P0-2 package is declared here together with this plan's; `deps` edges follow ARCHITECTURE §2.4 (app → service/api and the P0-1 platform; api → service; service → the engine; cmd → everything it wires). `depOnAnyVendor` stays `false`, so every third-party import is named:

```yaml
# go-arch-lint v1.19.0 — ARCHITECTURE §2.4 rules 2, 3, 5, 6, 7 and the component graph.
# go-arch-lint does not see standard-library imports, so rule 1/4 (database/sql, net/http), rule 8 (time)
# and the G-0 call check live in internal/archtest. P0-1 declared the platform; P0-3 adds P0-2's and its own components.
version: 3
workdir: .
allow:
  depOnAnyVendor: false
  deepScan: false   # interface injection (store.SettingsRepo → settings.Backend) is the intended design
exclude:
  - ta
  - third_party
  - frontend/node_modules
  - internal/archtest/testdata
excludeFiles:
  - "^.*_test\\.go$"

vendors:
  wails:      { in: [github.com/wailsapp/wails/v2, github.com/wailsapp/wails/v2/**] }
  kite:       { in: [github.com/zerodha/gokiteconnect/v4, github.com/zerodha/gokiteconnect/v4/**] }
  sqlite:     { in: [modernc.org/sqlite, modernc.org/sqlite/**] }
  keyring:    { in: [github.com/zalando/go-keyring] }
  xcrypto:    { in: [golang.org/x/crypto/**] }
  flock:      { in: [github.com/gofrs/flock] }
  lumberjack: { in: [gopkg.in/natefinch/lumberjack.v2] }
  zstd:       { in: [github.com/klauspost/compress/zstd] }
  xtools:     { in: [golang.org/x/tools/go/packages] }
  godbus:     { in: [github.com/godbus/dbus/v5] }
  uuid:       { in: [github.com/google/uuid] }
  websocket:  { in: [github.com/gorilla/websocket] }

components:
  cmd:          { in: cmd/tradeloom }
  frontend:     { in: frontend }
  app:          { in: internal/app }
  api:          { in: internal/app/api }
  service:      { in: internal/service }
  bus:          { in: internal/bus }
  clock:        { in: internal/clock }
  logging:      { in: internal/logging }
  settings:     { in: internal/settings }
  store:        { in: internal/store }
  storetest:    { in: internal/store/storetest }
  secrets:      { in: internal/secrets }
  keyringprobe: { in: internal/secrets/keyringprobe }   # S0-4 spike tool
  calendar:     { in: internal/calendar }
  kite:         { in: internal/kite }
  ratelimit:    { in: internal/kite/ratelimit }
  ticker:       { in: internal/kite/ticker }
  feed:         { in: internal/kite/feed }
  kitemock:     { in: internal/kite/mock }
  orderwrite:   { in: internal/kite/orderwrite }
  candles:      { in: internal/candles }
  candlesql:    { in: internal/candles/sqlite }
  backfill:     { in: internal/candles/backfill }
  builder:      { in: internal/candles/builder }
  instruments:  { in: internal/instruments }
  orders:       { in: internal/orders }
  notify:       { in: internal/notify/desktop }
  licences:     { in: internal/licences }
  archtest:     { in: internal/archtest }

commonComponents:
  - logging
  - clock

deps:
  cmd:
    mayDependOn: [frontend, app, api, service, bus, settings, store, secrets, calendar, candles, candlesql, backfill, builder, instruments, kite, ratelimit, ticker, feed, orders, licences]
    canUse: [wails]
  app:
    mayDependOn: [api, service, bus, settings, store, secrets]
    canUse: [wails, godbus]
  api:
    mayDependOn: [service, settings]
  service:
    mayDependOn: [bus, settings, store, secrets, calendar, kite, ratelimit, ticker, feed, candles, backfill, builder, instruments, orders]
  settings:
    mayDependOn: [bus]
  store:
    canUse: [sqlite, flock, zstd]
  storetest:
    mayDependOn: [store]
  secrets:
    canUse: [keyring, xcrypto]
  keyringprobe:
    mayDependOn: [secrets]
  kite:
    mayDependOn: [bus, settings, secrets, store, ratelimit]
    canUse: [kite, uuid]
  ratelimit:
    mayDependOn: [bus]
  ticker:
    mayDependOn: [bus, kite]
    canUse: [kite]
  feed:
    mayDependOn: [ticker]
  kitemock:
    mayDependOn: [kite, ratelimit]
    canUse: [kite, websocket]
  orderwrite:
    canUse: [kite]
  candles:
    mayDependOn: [calendar]
  candlesql:
    mayDependOn: [candles, calendar, store]
    canUse: [sqlite]
  backfill:
    mayDependOn: [bus, calendar, candles, kite, ratelimit]
    canUse: [uuid]
  builder:
    mayDependOn: [bus, calendar, candles, ticker]
  instruments:
    mayDependOn: [bus, calendar, kite, ratelimit, store]
  orders:
    mayDependOn: [bus, kite, ticker, store]
  notify:
    canUse: [godbus]
  logging:
    canUse: [lumberjack]
  archtest:
    canUse: [xtools]
```

- [ ] **Step 6: Build, test and lint**

Run: `go build -tags webkit2_41,noorders ./... && go vet -tags webkit2_41,noorders ./cmd/tradeloom/ ./internal/... && go test -tags webkit2_41,noorders ./cmd/tradeloom/ -run 'TestRun_' -v && go-arch-lint check`
Expected: build and vet exit 0; `TestRun_SecondInstanceExits` (P0-1), `TestRun_BadFlag` (P0-1) and `TestRun_RestartExitCode` PASS; `go-arch-lint` prints `OK - No warnings found`. If go-arch-lint names an edge this file does not list (a P0-2 import not visible in its plan), add that single edge and re-run — never widen with `depOnAnyVendor`.

- [ ] **Step 7: Commit**

```bash
git add internal/app/singleinstance.go internal/app/singleinstance_test.go internal/app/stdstreams.go internal/app/stdstreams_test.go internal/app/options.go \
  cmd/tradeloom/wire_p03.go cmd/tradeloom/main.go cmd/tradeloom/exit_test.go .go-arch-lint.yml go.mod go.sum
git commit -m "feat(app,cmd): D-Bus second-launch hand-off, std-stream redaction, P0-2 platform build and wiring

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
