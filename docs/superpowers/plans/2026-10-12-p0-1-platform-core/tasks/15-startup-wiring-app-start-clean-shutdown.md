> **P0-1 Platform Core Implementation Plan — Task 15.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 15: Startup wiring — `app.Start`, clean shutdown, `--smoke`, second-instance exit

**Files:**
- Create: `internal/app/boot.go`, `internal/app/boot_test.go`, `cmd/tradeloom/main_test.go`
- Modify: `cmd/tradeloom/main.go` (replace the Task 1 hello version)

**Interfaces:**
- Consumes: Task 1 `app.New`, `App.OnStartup`, `frontend.Assets`; Task 2 `clock.SystemNow`; Task 3 `bus.New`, `bus.Subscribe`; Task 4 `logging.New`, `Logger.Level`, `logging.ParseLevel`; Task 5 `logging.InstallCrashReporter`, `logging.UnseenCrashReports`, `logging.Go`; Task 6 `store.NewLayout`, `Layout.Ensure`, `store.AcquireLock`, `Lock.PreviousExit`, `Lock.MarkClean`, `Lock.Release`, `store.ResolveDataDir`, `store.ErrLocked`; Task 7 `store.Open`, `DB.Close`, `store.SchemaTooNewError`; Task 10 `settings.Load`, `settings.Topic`, `settings.LogLevel`, `settings.Get`; Task 12 `secrets.ProbeKeyringWithin`, `secrets.ProbeTimeout`; Task 14 `secrets.Detect`, `secrets.Detection`.
- Produces (`package app`):
  - `type BootOptions struct { DataDir string; AppVersion string; PID int; Now func() time.Time; Stderr io.Writer; ProbeKeyring func() error }`
  - `type Boot struct { Layout store.Layout; Lock *store.Lock; Log *logging.Logger; DB *store.DB; Bus *bus.Bus; Settings *settings.Settings; Secrets secrets.Detection; PreviousExit store.ExitState; CrashReports []string; CrashedLastTime bool; … }`
  - `func Start(ctx context.Context, o BootOptions) (*Boot, error)`; `func (b *Boot) Shutdown() error`; `func ExitMessage(dataDir string, err error) string`
  - `cmd/tradeloom`: `tradeloom --smoke` opens the window, runs startup + migrations, quits on DOM ready, exit 0 (used by S0-3); exit 1 with `Tradeloom is already running (data dir <dir>)` when the lock is held; exit 1 with the FR-DATA-005 AC2 message for a newer schema; exit 2 on bad flags.
  - Window options (`options.App`): `Width: 1440, Height: 900, MinWidth: 1280, MinHeight: 720` — the 1280 px floor is the contract P0-4/P0-5 lay out against (FR-SHELL-016, ROADMAP §P0 "1280 px layout").
  - P0-3 keeps `app.Start` before `wails.Run` (as tested here) and replaces `internal/app/app.go` / `app_test.go` (`New()`, `Hello`) with `app.New(app.Deps{Boot, Service, …})`, bound API structs under `internal/app/api`, the event bridge and `SingleInstanceLock` (Wails focus on top of `store.AcquireLock`, which stays authoritative: `store.ErrLocked` → P0-3 forwards the launch over D-Bus and exits). `app.Version` is unchanged.

- [ ] **Step 1: Write the failing tests**

`internal/app/boot_test.go`:

```go
package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

func bootOpts(dir string) BootOptions {
	return BootOptions{DataDir: dir, AppVersion: "v0.1.0", PID: 4242,
		Now:          func() time.Time { return time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC) },
		ProbeKeyring: func() error { return secrets.ErrUnavailable }}
}

func TestStart_FirstRunCreatesEverything(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tradeloom")
	b, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	if b.PreviousExit != store.ExitFirstRun || b.CrashedLastTime {
		t.Fatalf("first run: %s crashed=%v", b.PreviousExit, b.CrashedLastTime)
	}
	if b.DB.SchemaVersion() != store.SupportedSchema() {
		t.Fatalf("schema %d", b.DB.SchemaVersion())
	}
	if b.Secrets.Mode != secrets.ModeChoose || len(b.Secrets.Choices) != 2 {
		t.Fatalf("secrets %+v", b.Secrets)
	}
	if _, err := Start(context.Background(), bootOpts(dir)); !errors.Is(err, store.ErrLocked) {
		t.Fatalf("second instance: want ErrLocked, got %v", err)
	}
	if err := b.Shutdown(); err != nil {
		t.Fatal(err)
	}
	b2, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Shutdown()
	if b2.PreviousExit != store.ExitClean || b2.CrashedLastTime {
		t.Fatalf("after clean exit: %s crashed=%v", b2.PreviousExit, b2.CrashedLastTime)
	}
}

// FR-SHELL-018 AC2 (engine side): a run that ends without Shutdown is reported on the next start.
func TestStart_ReportsCrashLastTime(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tradeloom")
	b, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	b.close() // process died: no MarkClean
	b2, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Shutdown()
	if !b2.CrashedLastTime || b2.PreviousExit != store.ExitUnclean {
		t.Fatalf("want crash reported, got %s", b2.PreviousExit)
	}
}

// FR-SHELL-018: DEBUG is switchable at runtime without restart.
func TestStart_LogLevelFollowsSettings(t *testing.T) {
	b, err := Start(context.Background(), bootOpts(filepath.Join(t.TempDir(), "tl")))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Shutdown()
	if b.Log.Level.Level() != slog.LevelInfo {
		t.Fatal("default level must be INFO")
	}
	if err := settings.Set(context.Background(), b.Settings, settings.LogLevel, "debug"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for b.Log.Level.Level() != slog.LevelDebug {
		if time.Now().After(deadline) {
			t.Fatal("level not switched to DEBUG")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestExitMessage(t *testing.T) {
	if m := ExitMessage("/d", store.ErrLocked); m != "Tradeloom is already running (data dir /d)" {
		t.Fatal(m)
	}
	err := &store.SchemaTooNewError{Found: 3, Supported: 2, WrittenBy: "v1.4.0"}
	if m := ExitMessage("/d", err); !strings.HasPrefix(m, "Database schema 3 (written by v1.4.0)") {
		t.Fatal(m)
	}
}
```

`cmd/tradeloom/main_test.go`:

```go
package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/app"
	"tradeloom/internal/secrets"
	"tradeloom/internal/store"
)

// A second launch on the same data directory exits 1 with the ARCHITECTURE §3.7 message
// before any window is created.
func TestRun_SecondInstanceExits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tradeloom")
	first, err := app.Start(context.Background(), app.BootOptions{DataDir: dir, AppVersion: "test", PID: 1,
		Now: func() time.Time { return time.Unix(0, 0) }, ProbeKeyring: func() error { return secrets.ErrUnavailable }})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Shutdown()
	var stderr bytes.Buffer
	env := func(k string) string {
		if k == store.EnvDataDir {
			return dir
		}
		return ""
	}
	if code := run(nil, env, &stderr); code != 1 {
		t.Fatalf("exit code %d", code)
	}
	if want := "Tradeloom is already running (data dir " + dir + ")"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr %q", stderr.String())
	}
}

func TestRun_BadFlag(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"--nope"}, func(string) string { return "" }, &stderr); code != 2 {
		t.Fatalf("exit code %d", code)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -tags noorders ./internal/app/ ./cmd/tradeloom/`
Expected: FAIL — build errors `undefined: BootOptions`, `undefined: Start`, `undefined: ExitMessage` (app) and `undefined: app.Start`, `undefined: run` (cmd).

- [ ] **Step 3: Write the implementation**

`internal/app/boot.go`:

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

// BootOptions configures Start.
type BootOptions struct {
	DataDir      string           // resolved by store.ResolveDataDir
	AppVersion   string           // Version
	PID          int              // os.Getpid()
	Now          func() time.Time // clock.SystemNow
	Stderr       io.Writer        // optional log mirror (development); nil disables
	ProbeKeyring func() error     // nil means secrets.ProbeKeyringWithin(secrets.ProbeTimeout)
}

// Boot holds everything started before the window opens (ARCHITECTURE §3.2 up to wails.Run).
type Boot struct {
	Layout          store.Layout
	Lock            *store.Lock
	Log             *logging.Logger
	DB              *store.DB
	Bus             *bus.Bus
	Settings        *settings.Settings
	Secrets         secrets.Detection
	PreviousExit    store.ExitState
	CrashReports    []string // unseen crash-*.json, newest first
	CrashedLastTime bool     // FR-SHELL-018 AC2: show "The app crashed last time — view report"
	levelSub        *bus.Sub[settings.Changed]
}

// Start runs the pre-window startup sequence: layout → lock → logging + crash reporter →
// app.db open/migrate → bus → settings (log level applied live) → secrets backend detection.
// Errors: store.ErrLocked (another instance), *store.SchemaTooNewError, store.ErrCorrupt.
func Start(ctx context.Context, o BootOptions) (*Boot, error) {
	if o.ProbeKeyring == nil {
		o.ProbeKeyring = func() error { return secrets.ProbeKeyringWithin(secrets.ProbeTimeout) }
	}
	b := &Boot{Layout: store.NewLayout(o.DataDir)}
	if err := b.Layout.Ensure(); err != nil {
		return nil, err
	}
	lock, err := store.AcquireLock(b.Layout, o.PID)
	if err != nil {
		return nil, err
	}
	b.Lock, b.PreviousExit = lock, lock.PreviousExit()
	fail := func(err error) (*Boot, error) {
		b.close()
		return nil, err
	}
	if b.Log, err = logging.New(logging.Options{Dir: b.Layout.LogsDir, Stderr: o.Stderr}); err != nil {
		return fail(err)
	}
	logging.InstallCrashReporter(logging.CrashOptions{
		Dir: b.Layout.LogsDir, AppVersion: o.AppVersion, Now: o.Now,
		SchemaVersion: func() int {
			if b.DB == nil {
				return 0
			}
			return b.DB.SchemaVersion()
		},
	})
	if b.CrashReports, err = logging.UnseenCrashReports(b.Layout.LogsDir); err != nil {
		return fail(err)
	}
	b.CrashedLastTime = b.PreviousExit == store.ExitUnclean || len(b.CrashReports) > 0
	log := b.Log.With("component", "app")
	log.Info("app.starting", "version", o.AppVersion, "data_dir", o.DataDir, "previous_exit", string(b.PreviousExit))

	if b.DB, err = store.Open(ctx, b.Layout.AppDB, store.Options{AppVersion: o.AppVersion,
		BackupDir: b.Layout.BackupsDir, Now: o.Now, Log: b.Log.Logger}); err != nil {
		log.Error("store.open_failed", "err", err)
		return fail(err)
	}
	b.Bus = bus.New(b.Log.Logger)
	if b.Settings, err = settings.Load(ctx, b.DB.SettingsRepo(), b.Bus); err != nil {
		return fail(err)
	}
	b.applyLevel()
	b.levelSub = bus.Subscribe(b.Bus, settings.Topic, bus.SubOpts[settings.Changed]{Name: "app.loglevel", Buffer: 16})
	logging.Go("app.loglevel", func() {
		for ev := range b.levelSub.C() {
			if ev.Key == settings.LogLevel.Name {
				b.applyLevel()
			}
		}
	})
	b.Secrets = secrets.Detect(b.Layout.SecretsFile, o.ProbeKeyring)
	log.Info("secrets.detected", "mode", string(b.Secrets.Mode), "keyring", b.Secrets.KeyringAvailable,
		"probe_error", errString(b.Secrets.ProbeError))
	return b, nil
}

func (b *Boot) applyLevel() {
	lvl, err := logging.ParseLevel(settings.Get(b.Settings, settings.LogLevel))
	if err != nil {
		lvl = slog.LevelInfo
	}
	b.Log.Level.Set(lvl)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Shutdown is the clean-exit path (ARCHITECTURE §3.3 steps 5–7): bus close, app.db checkpoint +
// close, exit marker, lock release, log close.
func (b *Boot) Shutdown() error {
	b.Log.With("component", "app").Info("app.shutdown")
	var errs []error
	if b.Bus != nil {
		b.Bus.Close()
	}
	if b.DB != nil {
		errs = append(errs, b.DB.Close())
		b.DB = nil
	}
	errs = append(errs, b.Lock.MarkClean())
	b.close()
	return errors.Join(errs...)
}

// close releases whatever Start acquired, without marking a clean exit.
func (b *Boot) close() {
	if b.Bus != nil {
		b.Bus.Close()
	}
	if b.DB != nil {
		b.DB.Close()
	}
	if b.Lock != nil {
		b.Lock.Release()
	}
	if b.Log != nil {
		b.Log.Close()
	}
}

// ExitMessage maps a Start error to the single line printed on stderr before exiting.
func ExitMessage(dataDir string, err error) string {
	var tooNew *store.SchemaTooNewError
	switch {
	case errors.Is(err, store.ErrLocked):
		return fmt.Sprintf("Tradeloom is already running (data dir %s)", dataDir)
	case errors.As(err, &tooNew):
		return tooNew.Error()
	default:
		return "Tradeloom could not start: " + err.Error()
	}
}
```

`cmd/tradeloom/main.go` (full replacement):

```go
// Command tradeloom is the desktop application (the only package besides internal/app that imports Wails).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tradeloom/frontend"
	"tradeloom/internal/app"
	"tradeloom/internal/clock"
	"tradeloom/internal/store"
)

func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stderr)) }

// run returns the process exit code: 0 ok, 1 startup refused (already running, newer schema,
// damaged database …), 2 bad flags.
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
	if err != nil {
		fmt.Fprintln(stderr, app.ExitMessage(dataDir, err))
		return 1
	}
	var once sync.Once
	shutdown := func() { once.Do(func() { boot.Shutdown() }) }
	defer shutdown()

	a := app.New()
	opts := &options.App{
		Title:       "Tradeloom",
		Width:       1440,
		Height:      900,
		MinWidth:    1280, // FR-SHELL layouts are designed for 1280 px; P0-4 relies on this floor
		MinHeight:   720,
		AssetServer: &assetserver.Options{Assets: frontend.Assets()},
		OnStartup:   a.OnStartup,
		OnShutdown:  func(context.Context) { shutdown() },
		Bind:        []interface{}{a},
	}
	if *smoke {
		opts.OnDomReady = func(ctx context.Context) { runtime.Quit(ctx) }
	}
	if err := wails.Run(opts); err != nil {
		fmt.Fprintln(stderr, "tradeloom:", err)
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race -count=1 -tags noorders ./... `
Expected: every package `ok` (`internal/clock` reports `[no test files]`); archtest confirms `internal/app` imports only `bus`, `logging`, `secrets`, `settings`, `store` from `internal/` (rule 7).

- [ ] **Step 5: Smoke-run the real binary**

```bash
make build
export SMOKE_DIR="$(mktemp -d)/tradeloom"
TRADELOOM_DATA_DIR="$SMOKE_DIR" build/bin/tradeloom --smoke          # macOS: build/bin/tradeloom.app/Contents/MacOS/tradeloom --smoke
echo "exit=$?"; cat "$SMOKE_DIR/lock"; ls "$SMOKE_DIR"; head -3 "$SMOKE_DIR/logs/tradeloom.log"
```
Expected: the window flashes open and closes; `exit=0`; lock file contains `clean`; directory lists `app.db backups candles exports instruments library lock logs`; the log shows JSON lines with `"event":"app.starting"`, `"event":"store.migrated"` (`"from":0,"to":1`) and `"event":"secrets.detected"`.
Then, with the first instance still running (`TRADELOOM_DATA_DIR="$SMOKE_DIR" build/bin/tradeloom &`), a second `TRADELOOM_DATA_DIR="$SMOKE_DIR" build/bin/tradeloom` prints `Tradeloom is already running (data dir …)` and exits 1. Close the first window.

- [ ] **Step 6: Commit**

```bash
git add internal/app/boot.go internal/app/boot_test.go cmd/tradeloom/main.go cmd/tradeloom/main_test.go
git commit -m "feat(app): startup sequence (lock, logs, crash detection, app.db, bus, settings, secrets) and --smoke" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
