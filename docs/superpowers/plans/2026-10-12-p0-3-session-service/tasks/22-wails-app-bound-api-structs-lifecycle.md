> **P0-3 Session & Service Layer Implementation Plan — Task 22.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 22: Wails app — bound API structs, lifecycle, error format, options, generated bindings

**Execution order:** Tasks 27–32 and Task 33 Steps 1–4 (service-side code only) are executed before this task; this task binds their façade methods. Task 33 Steps 5–8 run after Task 23.

**Files:**
- Modify (replace): `internal/app/app.go`, `internal/app/app_test.go` from P0-1 Task 1 — the hello-window `App` (`New()`, `OnStartup`, `Hello`, `TestHello`) is replaced by the `App` below; `var Version = "dev"` (set by `-ldflags "-X tradeloom/internal/app.Version=<v>"`) is kept in `app.go`. P0-1 Task 15's `internal/app/boot.go` / `boot_test.go` are untouched.
- Create: `internal/app/errors.go`, `internal/app/options.go`, `internal/app/api/session.go`, `internal/app/api/portfolio.go`, `internal/app/api/settings.go`, `internal/app/api/chart.go`, `internal/app/api/instruments.go`, `internal/app/api/quotes.go`, `internal/app/api/watchlists.go`, `internal/app/api/data.go`, `internal/app/api/calendar.go`, `internal/app/api/diagnostics.go`, `internal/app/api/system.go`, `internal/service/servicetest/fakes.go`
- Modify: `Makefile` (P0-1) — add the `bindings` target
- Test: `internal/app/app_test.go` (replacement), `internal/app/api/api_test.go`
- Generated: `frontend/wailsjs/go/api/*.{js,d.ts}`, `frontend/wailsjs/go/models.ts`

**Interfaces:**
- Consumes: every `service` method in "Produces"; `service.EventSources`, `Restore`, `RunSession`, `Run`, `RunQuotes`, `RunBackups`, `BackupOnQuit`, `GetUiState`, `SetUiState`, `EvSessionState`, `EvJobProgress`, `JobProgress`, `NewSecretSwitch` (Task 27, test helper); P0-1 `app.Boot` (carried for Task 23's wiring), `app.Version`, `bus.New(nil)`, `settings.Load`, `storetest.Open`, `store.NewLayout`, `Layout.Ensure`, `secrets.NewNone`, `secrets.Detection`; P0-2 `calendar.NewDefault`, `ticker.FeedState`, `clock.Offset` (allowed `time.Time` payloads in bindings). From Wails v2.16.0 (verified source):
  - `pkg/runtime`: `EventsEmit(ctx, name string, data ...interface{})`, `BrowserOpenURL(ctx, url string)`, `WindowShow(ctx)`, `WindowUnminimise(ctx)`, `Quit(ctx)`, `OpenDirectoryDialog(ctx, OpenDialogOptions) (string, error)` and `OpenFileDialog(ctx, OpenDialogOptions) (string, error)` (both return `""` when the user cancels; the options struct carries a dialog title and a list of file filters, each a display name plus `;`-separated glob patterns), `WindowGetSize(ctx) (int, int)`, `WindowGetPosition(ctx) (int, int)`, `WindowIsMaximised(ctx) bool`, `WindowSetSize(ctx, w, h)`, `WindowSetPosition(ctx, x, y)`, `WindowMaximise(ctx)`.
  - `pkg/options`: `App{Title, Width, Height, MinWidth, MinHeight, AssetServer *assetserver.Options, OnStartup, OnDomReady, OnShutdown, OnBeforeClose, Bind, ErrorFormatter, EnableDefaultContextMenu, SingleInstanceLock *SingleInstanceLock, LogLevel, LogLevelProduction}`, `ErrorFormatter func(error) any`, `SingleInstanceLock{UniqueId string; OnSecondInstanceLaunch func(SecondInstanceData)}`.
  - `pkg/options/assetserver.Options{Assets fs.FS}`; `pkg/logger.ERROR`.
  - The binding generator writes `wailsjs/go/<package>/<Struct>.{js,d.ts}`, where `<package>` comes from the receiver type string (`*api.Session` → `api`; `internal/binding/reflect.go getPackageName`). `(T, error)` becomes `Promise<T>`, and an error alone becomes `Promise<void>`.
  - CLI: `wails generate module -tags "…"`.
- Produces:
  ```go
  // package app
  var Version = "dev" // unchanged from P0-1 Task 1
  type Runtime struct{ Emit func(name string, payload any); OpenURL func(url string); Focus func(); Quit func()
      OpenDirectoryDialog func(title string) (string, error); OpenFileDialog func(title string, patterns []string) (string, error)
      WindowGeometry func() (w, h, x, y int, maximised bool); SetWindowGeometry func(w, h, x, y int, maximised bool) }
  type Deps struct{ Boot *Boot; Service *service.Service; Log *slog.Logger; StartPlatform func(ctx context.Context) error; StopPlatform func(ctx context.Context) }
  const UiStateWindowGeometry = "window.geometry"
  type WindowGeometry struct{ W, H, X, Y int; Maximised bool } // JSON w, h, x, y, maximised
  func New(d Deps) *App
  func (a *App) Bind() []any                      // *api.Session, *api.Portfolio, *api.Settings, *api.Chart, *api.Instruments, *api.Quotes, *api.Watchlists, *api.Data, *api.Calendar, *api.Diagnostics, *api.System
  func (a *App) Startup(ctx context.Context)      // options.App.OnStartup; restores window.geometry
  func (a *App) Shutdown(ctx context.Context)     // options.App.OnShutdown (StopPlatform; boot.Shutdown stays in cmd/tradeloom.run)
  func (a *App) BeforeClose(ctx context.Context) bool // saves window.geometry, runs the scheduled backup if due; never prevents close in P0
  func (a *App) RequestRestart(); func (a *App) RestartRequested() bool // a finished restore asks main to re-exec (exit 3)
  func ErrorFormatter(err error) any              // JSON string {"code","stage","reason"}
  func Options(a *App, assets fs.FS, uniqueID string, smoke bool) *options.App // smoke: OnDomReady = runtime.Quit (P0-1 --smoke)
  // package api
  type Dialogs struct{ Directory func(title string) (string, error); File func(title string, patterns []string) (string, error) }
  func NewSession(svc *service.Service, ctx func() context.Context, open func(string)) *Session
  func NewPortfolio(svc, ctx) *Portfolio; func NewSettings(svc, ctx) *Settings; func NewChart(svc, ctx) *Chart; func NewInstruments(svc, ctx) *Instruments
  func NewQuotes(svc, ctx) *Quotes; func NewWatchlists(svc, ctx) *Watchlists; func NewData(svc, ctx, d Dialogs) *Data; func NewCalendar(svc, ctx, d Dialogs) *Calendar
  func NewDiagnostics(svc, ctx, open func(string)) *Diagnostics; func NewSystem(svc, ctx) *System
  // package servicetest (test helper)
  func New(t testing.TB) (*service.Service, *FakeSession, *bus.Bus)
  ```
  The TS binding names are listed in "Produces (for P0-4 / P0-5)".

`Session.Login()` calls `service.BeginLogin` and then opens the returned `LoginURL` in the system browser (`runtime.BrowserOpenURL`). When the state moves from `logging_in` to `ready`, the app focuses its window (`WindowUnminimise` + `WindowShow`, FR-ACCT-006 "focuses the app"). When a restore job reports `stage: "restart"`, the app forwards that event, records the restart request and calls `runtime.Quit` (A5: `main` re-execs on exit code 3). `Startup` restores the saved window size, position and maximised state and `BeforeClose` saves them (A6, FR-SHELL-019) through `Settings.SetUiState` key `window.geometry`. `LogLevelProduction` is `logger.ERROR`, so binding arguments (which include the API secret and passphrases) are never logged by Wails in production builds. The window options are the ones P0-1 settled on (`1440×900`, minimum `1280×720`). `Diagnostics.OpenLogsFolder` / `OpenCrashReport` open `file://` URLs through `BrowserOpenURL` (the desktop's file manager / viewer).

- [ ] **Step 1: Write the test helper**

```go
// internal/service/servicetest/fakes.go
// Package servicetest builds a *service.Service with a fake session for tests of internal/app.
package servicetest

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/calendar"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/orders"
	"tradeloom/internal/secrets"
	"tradeloom/internal/service"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

type FakeSession struct {
	mu        sync.Mutex
	St        kite.SessionState
	Err       error
	LoginURL  string
	completed []string
}

func (f *FakeSession) Set(st kite.SessionState)       { f.mu.Lock(); defer f.mu.Unlock(); f.St = st }
func (f *FakeSession) State() kite.SessionState        { f.mu.Lock(); defer f.mu.Unlock(); return f.St }
func (f *FakeSession) Completed() []string             { f.mu.Lock(); defer f.mu.Unlock(); return append([]string(nil), f.completed...) }
func (f *FakeSession) CredentialsInfo() kite.CredentialsInfo {
	return kite.CredentialsInfo{Port: 7420, RedirectURL: kite.RedirectURL(7420), SecretBackend: "memory"}
}
func (f *FakeSession) SaveCredentials(context.Context, string, string) (kite.SessionState, error) { return f.State(), f.Err }
func (f *FakeSession) RemoveCredentials(context.Context) (kite.SessionState, error)               { return f.State(), f.Err }
func (f *FakeSession) SetRedirectPort(int) (kite.CredentialsInfo, error)                          { return f.CredentialsInfo(), f.Err }
func (f *FakeSession) BeginLogin(context.Context) (kite.LoginStart, error) {
	return kite.LoginStart{LoginURL: f.LoginURL, RedirectURL: kite.RedirectURL(7420), ListenerOK: true}, f.Err
}
func (f *FakeSession) CompleteLogin(_ context.Context, p string) (kite.SessionState, error) {
	f.mu.Lock()
	f.completed = append(f.completed, p)
	f.mu.Unlock()
	return f.State(), f.Err
}
func (f *FakeSession) CancelLogin() kite.SessionState                     { return f.State() }
func (f *FakeSession) Logout(context.Context) (kite.LogoutResult, error)  { return kite.LogoutResult{}, f.Err }
func (f *FakeSession) AckRisk(context.Context) (kite.SessionState, error) { return f.State(), f.Err }
func (f *FakeSession) Profile(context.Context) (kite.Profile, error)      { return kite.Profile{}, f.Err }
func (f *FakeSession) Restore(context.Context) kite.SessionState          { return f.State() }
func (f *FakeSession) Run(ctx context.Context)                            { <-ctx.Done() }

type noSync struct{}

func (noSync) Run(ctx context.Context)                       { <-ctx.Done() }
func (noSync) SetDataPlan(kite.DataPlan)                     {}
func (noSync) Refresh(context.Context, orders.Scope) error   { return nil }
func (noSync) Book() orders.BookSnapshot                     { return orders.BookSnapshot{} }
func (noSync) Positions() orders.PositionsSnapshot           { return orders.PositionsSnapshot{} }
func (noSync) Holdings() orders.HoldingsSnapshot             { return orders.HoldingsSnapshot{} }

// New returns a Service over real settings/watchlists/shortcuts/UI-state storage in a temporary data
// directory, the shipped calendar, a "no storage" secrets switch and a fake session. Chart, quote,
// candle-store, refresher and feed dependencies are nil (the app tests never reach them).
func New(t testing.TB) (*service.Service, *FakeSession, *bus.Bus) {
	t.Helper()
	db := storetest.Open(t)
	b := bus.New(nil)
	t.Cleanup(b.Close)
	st, err := settings.Load(context.Background(), db.SettingsRepo(), b)
	if err != nil {
		t.Fatal(err)
	}
	cal, err := calendar.NewDefault()
	if err != nil {
		t.Fatal(err)
	}
	layout := store.NewLayout(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	fs := &FakeSession{St: kite.SessionState{State: kite.StateLoggedOut}, LoginURL: "https://kite.zerodha.com/connect/login?api_key=k&v=3"}
	svc := service.New(service.Deps{
		Session: fs, Orders: noSync{}, Settings: st, Watchlists: store.NewWatchlistsRepo(db), Calendar: cal,
		Secrets: service.NewSecretSwitch(secrets.NewNone(), false), SecretsFile: layout.SecretsFile,
		Detection: secrets.Detection{Mode: secrets.ModeChoose, Choices: []secrets.Choice{secrets.ChoiceEncryptedFile, secrets.ChoiceNoStorage}},
		Shortcuts: store.NewShortcutsRepo(db), UiState: store.NewWindowStateRepo(db), DB: db, Layout: layout, AppVersion: "test",
		Clock: clock.NewFake(time.Date(2026, 10, 12, 9, 0, 0, 0, clock.IST)), Bus: b, Log: slog.Default(),
		NewID: func() string { return "id-1" },
	})
	return svc, fs, b
}
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/app/api/api_test.go
package api

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/kite/ticker"
	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

func TestSessionLogin_OpensSystemBrowser(t *testing.T) {
	svc, fs, _ := servicetest.New(t)
	var opened []string
	s := NewSession(svc, context.Background, func(u string) { opened = append(opened, u) })
	ls, err := s.Login()
	if err != nil || len(opened) != 1 || opened[0] != fs.LoginURL || ls.LoginURL != fs.LoginURL {
		t.Fatalf("%+v %v %v", ls, err, opened)
	}
	fs.Err = &kite.LoginError{Code: kite.CodeNoCredentials, Message: "Set up first"}
	if _, err := s.Login(); err == nil || len(opened) != 1 {
		t.Fatalf("browser opened on error: %v %v", err, opened)
	}
}

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
	timeType  = reflect.TypeOf(time.Time{})
	// P0-2 payloads that already carry time.Time (RFC3339 strings) and are passed through unchanged (contract §C).
	p02Payloads = map[reflect.Type]bool{reflect.TypeOf(ticker.FeedState{}): true, reflect.TypeOf(clock.Offset{}): true}
)

func checkType(t *testing.T, where string, ty reflect.Type, seen map[reflect.Type]bool) {
	if seen[ty] || p02Payloads[ty] {
		return
	}
	seen[ty] = true
	switch ty.Kind() {
	case reflect.Interface, reflect.Chan, reflect.Func, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		t.Errorf("%s: %s is not JSON-friendly", where, ty)
	case reflect.Slice, reflect.Array, reflect.Pointer, reflect.Map:
		checkType(t, where, ty.Elem(), seen)
	case reflect.Struct:
		if ty == timeType {
			t.Errorf("%s: time.Time in a binding (use ms epoch)", where)
			return
		}
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			if !f.IsExported() {
				continue
			}
			if tag := f.Tag.Get("json"); tag == "" || strings.HasPrefix(tag, ",") {
				t.Errorf("%s: %s.%s has no json name", where, ty, f.Name)
			}
			checkType(t, where+"."+f.Name, f.Type, seen)
		}
	}
}

func allBound(t *testing.T) []any {
	t.Helper()
	svc, _, _ := servicetest.New(t)
	noOpen := func(string) {}
	dialogs := Dialogs{Directory: func(string) (string, error) { return "", nil }, File: func(string, []string) (string, error) { return "", nil }}
	return []any{
		NewSession(svc, context.Background, noOpen), NewPortfolio(svc, context.Background), NewSettings(svc, context.Background),
		NewChart(svc, context.Background), NewInstruments(svc, context.Background), NewQuotes(svc, context.Background),
		NewWatchlists(svc, context.Background), NewData(svc, context.Background, dialogs), NewCalendar(svc, context.Background, dialogs),
		NewDiagnostics(svc, context.Background, noOpen), NewSystem(svc, context.Background),
	}
}

// ARCH §8.4: bound methods take/return JSON-friendly structs only (no time.Time outside the P0-2 payloads, no interfaces), error last.
func TestBindings_AreJSONFriendly(t *testing.T) {
	bound := allBound(t)
	if len(bound) != 11 {
		t.Fatalf("%d bound structs, want 11", len(bound))
	}
	for _, b := range bound {
		rt := reflect.TypeOf(b)
		if rt.NumMethod() == 0 {
			t.Fatalf("%s binds nothing", rt)
		}
		for i := 0; i < rt.NumMethod(); i++ {
			m := rt.Method(i)
			where := rt.Elem().Name() + "." + m.Name
			for j := 1; j < m.Type.NumIn(); j++ {
				checkType(t, where+" arg", m.Type.In(j), map[reflect.Type]bool{})
			}
			for j := 0; j < m.Type.NumOut(); j++ {
				out := m.Type.Out(j)
				if out == errorType {
					if j != m.Type.NumOut()-1 {
						t.Errorf("%s: error must be last", where)
					}
					continue
				}
				checkType(t, where+" result", out, map[reflect.Type]bool{})
			}
		}
	}
	_ = service.SessionStatus{}
}

// The dialog bindings pass the runtime's answer through; "" (cancelled) is not an error.
func TestDialogs_PassThrough(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	var titles []string
	dialogs := Dialogs{
		Directory: func(title string) (string, error) { titles = append(titles, title); return "/backups", nil },
		File:      func(title string, patterns []string) (string, error) { titles = append(titles, title+" "+strings.Join(patterns, ",")); return "", nil },
	}
	d := NewData(svc, context.Background, dialogs)
	if folder, err := d.ChooseFolder(); err != nil || folder != "/backups" {
		t.Fatalf("%q %v", folder, err)
	}
	if file, err := d.ChooseBackupFile(); err != nil || file != "" {
		t.Fatalf("%q %v", file, err)
	}
	c := NewCalendar(svc, context.Background, dialogs)
	if file, err := c.ChooseImportFile(); err != nil || file != "" {
		t.Fatalf("%q %v", file, err)
	}
	if len(titles) != 3 || titles[0] != "Choose a backup folder" || titles[1] != "Choose a backup archive *.tar.zst" || titles[2] != "Choose a calendar CSV *.csv" {
		t.Fatalf("titles %v", titles)
	}
}

// Diagnostics opens the logs folder as a file:// URL through the runtime's browser opener.
func TestDiagnostics_OpensLogsFolder(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	var opened []string
	d := NewDiagnostics(svc, context.Background, func(u string) { opened = append(opened, u) })
	if err := d.OpenLogsFolder(); err != nil {
		t.Fatal(err)
	}
	dir, _ := svc.OpenLogsFolder(context.Background())
	if len(opened) != 1 || opened[0] != "file://"+dir {
		t.Fatalf("opened %v", opened)
	}
	if err := d.OpenCrashReport(); err == nil || len(opened) != 1 {
		t.Fatalf("no crash report: err %v opened %v", err, opened)
	}
}
```

```go
// internal/app/app_test.go
package app

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/kite"
	"tradeloom/internal/service"
	"tradeloom/internal/service/servicetest"
)

type fakeRuntime struct {
	mu       sync.Mutex
	events   []string
	payloads map[string]any
	focuses  int
	quits    int
	opened   []string
	geom     WindowGeometry
	setGeom  []WindowGeometry
}

func (f *fakeRuntime) rt() Runtime {
	return Runtime{
		Emit: func(n string, p any) {
			f.mu.Lock()
			f.events = append(f.events, n)
			f.payloads[n] = p
			f.mu.Unlock()
		},
		OpenURL: func(u string) { f.mu.Lock(); f.opened = append(f.opened, u); f.mu.Unlock() },
		Focus:   func() { f.mu.Lock(); f.focuses++; f.mu.Unlock() },
		Quit:    func() { f.mu.Lock(); f.quits++; f.mu.Unlock() },
		OpenDirectoryDialog: func(string) (string, error) { return "", nil },
		OpenFileDialog:      func(string, []string) (string, error) { return "", nil },
		WindowGeometry: func() (int, int, int, int, bool) {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.geom.W, f.geom.H, f.geom.X, f.geom.Y, f.geom.Maximised
		},
		SetWindowGeometry: func(w, h, x, y int, max bool) {
			f.mu.Lock()
			f.setGeom = append(f.setGeom, WindowGeometry{w, h, x, y, max})
			f.mu.Unlock()
		},
	}
}
func (f *fakeRuntime) focusCount() int { f.mu.Lock(); defer f.mu.Unlock(); return f.focuses }
func (f *fakeRuntime) quitCount() int  { f.mu.Lock(); defer f.mu.Unlock(); return f.quits }
func (f *fakeRuntime) sawEvent(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.payloads[name]
	return ok
}

func newFakeRuntime() *fakeRuntime { return &fakeRuntime{payloads: map[string]any{}} }

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// FR-ACCT-006: "focuses the app" after a successful login, not after a restore.
func TestApp_FocusesWindowOnLoginSuccess(t *testing.T) {
	svc, _, b := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	bus.Publish(b, kite.TopicSessionState, kite.SessionState{State: kite.StateReady})
	time.Sleep(30 * time.Millisecond)
	if fr.focusCount() != 0 {
		t.Fatal("focused on a restore")
	}
	bus.Publish(b, kite.TopicSessionState, kite.SessionState{State: kite.StateLoggingIn})
	bus.Publish(b, kite.TopicSessionState, kite.SessionState{State: kite.StateReady})
	eventually(t, "focus", func() bool { return fr.focusCount() == 1 })
}

func TestErrorFormatter(t *testing.T) {
	var got map[string]string
	if err := json.Unmarshal([]byte(ErrorFormatter(&service.ErrRefused{Code: "c", Stage: "login", Reason: "r"}).(string)), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != "c" || got["stage"] != "login" || got["reason"] != "r" {
		t.Fatalf("%v", got)
	}
	_ = json.Unmarshal([]byte(ErrorFormatter(errors.New("boom")).(string)), &got)
	if got["code"] != "internal" || got["reason"] != "boom" {
		t.Fatalf("%v", got)
	}
}

func TestOptions(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	assets := fstest.MapFS{"index.html": {Data: []byte("<html></html>")}}
	o := Options(a, assets, "tradeloom-0123456789ab", false)
	if o.SingleInstanceLock == nil || o.SingleInstanceLock.UniqueId != "tradeloom-0123456789ab" || o.SingleInstanceLock.OnSecondInstanceLaunch == nil {
		t.Fatalf("single instance %+v", o.SingleInstanceLock)
	}
	if len(o.Bind) != 11 || o.ErrorFormatter == nil || o.OnStartup == nil || o.OnShutdown == nil || o.OnBeforeClose == nil || o.EnableDefaultContextMenu {
		t.Fatalf("options %+v", o)
	}
	if o.Width != 1440 || o.Height != 900 || o.MinWidth != 1280 || o.MinHeight != 720 {
		t.Fatalf("window %dx%d min %dx%d", o.Width, o.Height, o.MinWidth, o.MinHeight)
	}
	if o.OnDomReady != nil {
		t.Fatal("OnDomReady must be unset without --smoke")
	}
	if Options(a, assets, "tradeloom-0123456789ab", true).OnDomReady == nil {
		t.Fatal("--smoke must quit on DOM ready")
	}
}

func TestRestartRequested(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	a := New(Deps{Service: svc})
	if a.RestartRequested() {
		t.Fatal("fresh app must not request a restart")
	}
	a.RequestRestart()
	if !a.RestartRequested() {
		t.Fatal("RequestRestart not recorded")
	}
}

// FR-SHELL-019 / A6: the window geometry is restored at startup and saved before close.
func TestApp_RestoresAndSavesWindowGeometry(t *testing.T) {
	svc, _, _ := servicetest.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := svc.SetUiState(ctx, UiStateWindowGeometry, `{"w":1500,"h":950,"x":10,"y":20,"maximised":false}`); err != nil {
		t.Fatal(err)
	}
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	a.start(ctx, fr.rt())
	fr.mu.Lock()
	restored := append([]WindowGeometry(nil), fr.setGeom...)
	fr.geom = WindowGeometry{W: 1600, H: 1000, X: 5, Y: 6, Maximised: true}
	fr.mu.Unlock()
	if len(restored) != 1 || restored[0] != (WindowGeometry{W: 1500, H: 950, X: 10, Y: 20}) {
		t.Fatalf("restored %+v", restored)
	}
	if a.BeforeClose(ctx) {
		t.Fatal("P0 never prevents closing")
	}
	saved, err := svc.GetUiState(ctx, UiStateWindowGeometry)
	if err != nil || saved != `{"w":1600,"h":1000,"x":5,"y":6,"maximised":true}` {
		t.Fatalf("saved %q %v", saved, err)
	}
}

// A5: a finished restore is forwarded to the UI, then the app asks for a restart and quits.
func TestApp_QuitsAfterRestoreJob(t *testing.T) {
	svc, _, b := servicetest.New(t)
	a := New(Deps{Service: svc})
	fr := newFakeRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.start(ctx, fr.rt())
	bus.Publish(b, service.TopicJobProgress, service.JobProgress{JobID: "b1", Kind: "backup", Stage: "done", Finished: true, ResultPath: "/x.tar.zst"})
	eventually(t, "backup event", func() bool { return fr.sawEvent(service.EvJobProgress) })
	if fr.quitCount() != 0 || a.RestartRequested() {
		t.Fatal("a backup must not restart the app")
	}
	bus.Publish(b, service.TopicJobProgress, service.JobProgress{JobID: "r1", Kind: "restore", Stage: "restart", Finished: true})
	eventually(t, "quit", func() bool { return fr.quitCount() == 1 })
	if !a.RestartRequested() {
		t.Fatal("restart not requested")
	}
	fr.mu.Lock()
	jp := fr.payloads[service.EvJobProgress].(service.JobProgress)
	fr.mu.Unlock()
	if jp.Stage != "restart" {
		t.Fatalf("UI did not get the final restore event: %+v", jp)
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/app/... -v`
Expected: FAIL to compile with `undefined: NewSession`, `undefined: Dialogs`, `undefined: Deps`, `undefined: Options`, `undefined: WindowGeometry` (P0-1's `TestHello` is gone with `app_test.go`; `boot_test.go` still compiles).

- [ ] **Step 4: Write the implementation (`internal/app/app.go` replaces P0-1 Task 1's file; `Version` is kept)**

```go
// internal/app/api/session.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

// Session is bound to Wails as window.go.api.Session (frontend/wailsjs/go/api/Session).
type Session struct {
	svc  *service.Service
	ctx  func() context.Context
	open func(url string)
}

func NewSession(svc *service.Service, ctx func() context.Context, open func(string)) *Session {
	return &Session{svc: svc, ctx: ctx, open: open}
}

func (s *Session) Status() service.SessionStatus         { return s.svc.SessionStatus(s.ctx()) }
func (s *Session) Credentials() service.CredentialsInfo  { return s.svc.CredentialsInfo(s.ctx()) }
func (s *Session) CancelLogin() service.SessionStatus    { return s.svc.CancelLogin(s.ctx()) }
func (s *Session) Profile() (service.Profile, error)     { return s.svc.Profile(s.ctx()) }
func (s *Session) Logout() (service.LogoutResult, error) { return s.svc.Logout(s.ctx()) }
func (s *Session) RemoveCredentials() (service.SessionStatus, error) {
	return s.svc.RemoveCredentials(s.ctx())
}
func (s *Session) AckRiskDisclosure() (service.SessionStatus, error) {
	return s.svc.AckRiskDisclosure(s.ctx())
}

func (s *Session) SaveCredentials(in service.CredentialsInput) (service.SessionStatus, error) {
	return s.svc.SaveCredentials(s.ctx(), in)
}

func (s *Session) SetRedirectPort(port int) (service.CredentialsInfo, error) {
	return s.svc.SetRedirectPort(s.ctx(), port)
}

func (s *Session) CompleteLogin(pasted string) (service.SessionStatus, error) {
	return s.svc.CompleteLogin(s.ctx(), pasted)
}

// Login starts a login attempt and opens Kite's login page in the system browser (FR-ACCT-006).
func (s *Session) Login() (service.LoginStart, error) {
	ls, err := s.svc.BeginLogin(s.ctx())
	if err != nil {
		return service.LoginStart{}, err
	}
	s.open(ls.LoginURL)
	return ls, nil
}

// Secret storage (FR-ACCT-013, Task 27).
func (s *Session) SecretStorage() service.SecretStorageInfo { return s.svc.SecretStorage(s.ctx()) }
func (s *Session) UseEncryptedFile(passphrase string) (service.SessionStatus, error) {
	return s.svc.UseEncryptedFile(s.ctx(), passphrase)
}
func (s *Session) RefuseSecretStorage() (service.SessionStatus, error) {
	return s.svc.RefuseSecretStorage(s.ctx())
}
func (s *Session) Unlock(passphrase string) (service.SessionStatus, error) {
	return s.svc.Unlock(s.ctx(), passphrase)
}
func (s *Session) MigrateToKeyring() (service.SecretStorageInfo, error) {
	return s.svc.MigrateToKeyring(s.ctx())
}

// ReconnectFeed is the status indicator's "Retry now".
func (s *Session) ReconnectFeed() error { return s.svc.ReconnectFeed(s.ctx()) }
```

```go
// internal/app/api/portfolio.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Portfolio struct {
	svc *service.Service
	ctx func() context.Context
}

func NewPortfolio(svc *service.Service, ctx func() context.Context) *Portfolio {
	return &Portfolio{svc: svc, ctx: ctx}
}

func (p *Portfolio) OrderBook() service.BookSnapshot      { return p.svc.OrderBook(p.ctx()) }
func (p *Portfolio) Positions() service.PositionsSnapshot { return p.svc.Positions(p.ctx()) }
func (p *Portfolio) Holdings() service.HoldingsSnapshot   { return p.svc.Holdings(p.ctx()) }
func (p *Portfolio) Refresh(scope string) error           { return p.svc.RefreshPortfolio(p.ctx(), scope) }
```

```go
// internal/app/api/settings.go
package api

import (
	"context"

	"tradeloom/internal/service"
	"tradeloom/internal/settings"
)

type Settings struct {
	svc *service.Service
	ctx func() context.Context
}

func NewSettings(svc *service.Service, ctx func() context.Context) *Settings {
	return &Settings{svc: svc, ctx: ctx}
}

func (s *Settings) All() ([]service.SettingValue, error) { return s.svc.Settings(s.ctx()) }
func (s *Settings) Set(name, valueJSON string) (service.SettingValue, error) {
	return s.svc.SetSetting(s.ctx(), name, valueJSON)
}

// Shortcuts (FR-SHELL-019 keyboard, Task 28).
func (s *Settings) Shortcuts() ([]settings.ShortcutAction, error) { return s.svc.Shortcuts(s.ctx()) }
func (s *Settings) Rebind(actionID, binding string) (settings.ShortcutAction, error) {
	return s.svc.Rebind(s.ctx(), actionID, binding)
}
func (s *Settings) ResetShortcuts() ([]settings.ShortcutAction, error) { return s.svc.ResetShortcuts(s.ctx()) }

// UI state (window_state table).
func (s *Settings) GetUiState(key string) (string, error) { return s.svc.GetUiState(s.ctx(), key) }
func (s *Settings) SetUiState(key, value string) error    { return s.svc.SetUiState(s.ctx(), key, value) }
```

```go
// internal/app/api/chart.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Chart struct {
	svc *service.Service
	ctx func() context.Context
}

func NewChart(svc *service.Service, ctx func() context.Context) *Chart {
	return &Chart{svc: svc, ctx: ctx}
}

func (c *Chart) Candles(req service.CandlesRequest) (service.CandlesResult, error) {
	return c.svc.Candles(c.ctx(), req)
}
func (c *Chart) Refresh(req service.RefreshRequest) (service.CandlesResult, error) {
	return c.svc.RefreshCandles(c.ctx(), req)
}
func (c *Chart) OpenLive(token uint32, interval string) (string, error) {
	return c.svc.OpenLive(c.ctx(), token, interval)
}
func (c *Chart) CloseLive(id string) error { return c.svc.CloseLive(c.ctx(), id) }
```

```go
// internal/app/api/instruments.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Instruments struct {
	svc *service.Service
	ctx func() context.Context
}

func NewInstruments(svc *service.Service, ctx func() context.Context) *Instruments {
	return &Instruments{svc: svc, ctx: ctx}
}

func (i *Instruments) Search(q string, tags []string, limit int) ([]service.InstrumentDTO, error) {
	return i.svc.SearchInstruments(i.ctx(), q, tags, limit)
}
func (i *Instruments) ByTokens(tokens []uint32) ([]service.InstrumentDTO, error) {
	return i.svc.InstrumentsByTokens(i.ctx(), tokens)
}
func (i *Instruments) ByIdentifier(id string) (service.InstrumentDTO, error) {
	return i.svc.InstrumentByIdentifier(i.ctx(), id)
}
```

```go
// internal/app/api/quotes.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Quotes struct {
	svc *service.Service
	ctx func() context.Context
}

func NewQuotes(svc *service.Service, ctx func() context.Context) *Quotes {
	return &Quotes{svc: svc, ctx: ctx}
}

func (q *Quotes) Watch(consumer string, tokens []uint32) (service.WatchResult, error) {
	return q.svc.WatchQuotes(q.ctx(), consumer, tokens)
}
```

```go
// internal/app/api/watchlists.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Watchlists struct {
	svc *service.Service
	ctx func() context.Context
}

func NewWatchlists(svc *service.Service, ctx func() context.Context) *Watchlists {
	return &Watchlists{svc: svc, ctx: ctx}
}

func (w *Watchlists) List() (service.WatchlistsState, error) { return w.svc.Watchlists(w.ctx()) }
func (w *Watchlists) Create(name string) (service.WatchlistsState, error) {
	return w.svc.CreateWatchlist(w.ctx(), name)
}
func (w *Watchlists) Rename(id, name string) (service.WatchlistsState, error) {
	return w.svc.RenameWatchlist(w.ctx(), id, name)
}
func (w *Watchlists) Delete(id string) (service.WatchlistsState, error) {
	return w.svc.DeleteWatchlist(w.ctx(), id)
}
func (w *Watchlists) SetActive(id string) (service.WatchlistsState, error) {
	return w.svc.SetActiveWatchlist(w.ctx(), id)
}
func (w *Watchlists) Add(id string, token uint32, position int) (service.WatchlistsState, error) {
	return w.svc.AddToWatchlist(w.ctx(), id, token, position)
}
func (w *Watchlists) Remove(id string, token uint32) (service.RemovedItem, error) {
	return w.svc.RemoveFromWatchlist(w.ctx(), id, token)
}
func (w *Watchlists) Reorder(id string, tokens []uint32) (service.WatchlistsState, error) {
	return w.svc.ReorderWatchlist(w.ctx(), id, tokens)
}
func (w *Watchlists) ReorderLists(ids []string) (service.WatchlistsState, error) {
	return w.svc.ReorderWatchlists(w.ctx(), ids)
}
```

```go
// internal/app/api/data.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

// Dialogs are the native file pickers (runtime.OpenDirectoryDialog / OpenFileDialog); "" means cancelled.
type Dialogs struct {
	Directory func(title string) (string, error)
	File      func(title string, patterns []string) (string, error)
}

type Data struct {
	svc     *service.Service
	ctx     func() context.Context
	dialogs Dialogs
}

func NewData(svc *service.Service, ctx func() context.Context, d Dialogs) *Data {
	return &Data{svc: svc, ctx: ctx, dialogs: d}
}

func (d *Data) Stats() (service.DataStats, error) { return d.svc.DataStats(d.ctx()) }
func (d *Data) ClearCandles() error                { return d.svc.ClearCandles(d.ctx()) }
func (d *Data) RefreshInstruments() error          { return d.svc.RefreshInstruments(d.ctx()) }
func (d *Data) ChooseFolder() (string, error)      { return d.dialogs.Directory("Choose a backup folder") }
func (d *Data) ChooseBackupFile() (string, error) {
	return d.dialogs.File("Choose a backup archive", []string{"*.tar.zst"})
}
func (d *Data) BackupNow(folder string, includeCandles bool) (string, error) {
	return d.svc.BackupNow(d.ctx(), folder, includeCandles)
}
func (d *Data) InspectBackup(path string) (service.BackupInfo, error) {
	return d.svc.InspectBackup(d.ctx(), path)
}
func (d *Data) Restore(path, confirmation string) (string, error) {
	return d.svc.RestoreBackup(d.ctx(), path, confirmation)
}
```

```go
// internal/app/api/calendar.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Calendar struct {
	svc     *service.Service
	ctx     func() context.Context
	dialogs Dialogs
}

func NewCalendar(svc *service.Service, ctx func() context.Context, d Dialogs) *Calendar {
	return &Calendar{svc: svc, ctx: ctx, dialogs: d}
}

func (c *Calendar) Info() (service.CalendarInfo, error) { return c.svc.CalendarInfo(c.ctx()) }
func (c *Calendar) ChooseImportFile() (string, error) {
	return c.dialogs.File("Choose a calendar CSV", []string{"*.csv"})
}
func (c *Calendar) Import(path string) (service.CalendarInfo, error) {
	return c.svc.ImportCalendar(c.ctx(), path)
}
func (c *Calendar) Upsert(in service.CalendarEntryInput) (service.CalendarInfo, error) {
	return c.svc.UpsertCalendarEntry(c.ctx(), in)
}
func (c *Calendar) Remove(exchange, date string) (service.CalendarInfo, error) {
	return c.svc.RemoveCalendarEntry(c.ctx(), exchange, date)
}
```

```go
// internal/app/api/diagnostics.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Diagnostics struct {
	svc  *service.Service
	ctx  func() context.Context
	open func(url string)
}

func NewDiagnostics(svc *service.Service, ctx func() context.Context, open func(string)) *Diagnostics {
	return &Diagnostics{svc: svc, ctx: ctx, open: open}
}

// OpenLogsFolder shows the logs directory in the desktop's file manager (file:// through BrowserOpenURL).
func (d *Diagnostics) OpenLogsFolder() error {
	dir, err := d.svc.OpenLogsFolder(d.ctx())
	if err != nil {
		return err
	}
	d.open("file://" + dir)
	return nil
}

func (d *Diagnostics) CreateSupportBundle() (string, error) { return d.svc.CreateSupportBundle(d.ctx()) }
func (d *Diagnostics) LastCrash() (service.CrashInfo, error) { return d.svc.LastCrash(d.ctx()) }

func (d *Diagnostics) OpenCrashReport() error {
	path, err := d.svc.OpenCrashReport(d.ctx())
	if err != nil {
		return err
	}
	d.open("file://" + path)
	return nil
}

func (d *Diagnostics) DismissCrash() error { return d.svc.DismissCrash(d.ctx()) }
```

```go
// internal/app/api/system.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type System struct {
	svc *service.Service
	ctx func() context.Context
}

func NewSystem(svc *service.Service, ctx func() context.Context) *System {
	return &System{svc: svc, ctx: ctx}
}

func (s *System) Info() service.AppInfo          { return s.svc.AppInfo(s.ctx()) }
func (s *System) Status() service.StatusSnapshot { return s.svc.Status(s.ctx()) }
```

The dialog titles are the strings the tests assert: `Choose a backup folder`, `Choose a backup archive` + ` *.tar.zst`, `Choose a calendar CSV` + ` *.csv` (the test's fake joins title and patterns with a space).

```go
// internal/app/app.go (full replacement of P0-1 Task 1's file; boot.go from P0-1 Task 15 stays beside it)
// Package app is the Wails glue (ARCHITECTURE §2.2): lifecycle callbacks, bound API structs under
// internal/app/api and the service → runtime.EventsEmit bridge. Boot (boot.go) is P0-1's pre-window startup.
package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tradeloom/internal/app/api"
	"tradeloom/internal/logging"
	"tradeloom/internal/service"
)

// Version is set at build time: -ldflags "-X tradeloom/internal/app.Version=v0.1.0" (unchanged from P0-1).
var Version = "dev"

// UiStateWindowGeometry is the window_state key of the saved window geometry (FR-SHELL-019, A6).
const UiStateWindowGeometry = "window.geometry"

// WindowGeometry is the saved window size, position and maximised state.
type WindowGeometry struct {
	W         int  `json:"w"`
	H         int  `json:"h"`
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Maximised bool `json:"maximised"`
}

// Runtime abstracts the Wails runtime so the lifecycle is testable without a window.
type Runtime struct {
	Emit                func(name string, payload any)
	OpenURL             func(url string)
	Focus               func()
	Quit                func()
	OpenDirectoryDialog func(title string) (string, error)
	OpenFileDialog      func(title string, patterns []string) (string, error)
	WindowGeometry      func() (w, h, x, y int, maximised bool)
	SetWindowGeometry   func(w, h, x, y int, maximised bool)
}

type Deps struct {
	Boot          *Boot // P0-1 app.Start result (layout, db, bus, settings, secrets detection); nil in unit tests
	Service       *service.Service
	Log           *slog.Logger
	StartPlatform func(ctx context.Context) error // P0-2 process services (limiter, backfill, instrument refresher, builder)
	StopPlatform  func(ctx context.Context)       // §3.3 order: feed, backfill, candle store; boot.Shutdown runs in cmd/tradeloom.run
}

type App struct {
	d Deps

	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	rt        Runtime
	lastState string
	restart   atomic.Bool

	session     *api.Session
	portfolio   *api.Portfolio
	settings    *api.Settings
	chart       *api.Chart
	instruments *api.Instruments
	quotes      *api.Quotes
	watchlists  *api.Watchlists
	data        *api.Data
	calendar    *api.Calendar
	diagnostics *api.Diagnostics
	system      *api.System
}

func noopRuntime() Runtime {
	return Runtime{
		Emit: func(string, any) {}, OpenURL: func(string) {}, Focus: func() {}, Quit: func() {},
		OpenDirectoryDialog: func(string) (string, error) { return "", nil },
		OpenFileDialog:      func(string, []string) (string, error) { return "", nil },
		WindowGeometry:      func() (int, int, int, int, bool) { return 0, 0, 0, 0, false },
		SetWindowGeometry:   func(int, int, int, int, bool) {},
	}
}

func New(d Deps) *App {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	a := &App{d: d, ctx: context.Background(), rt: noopRuntime()}
	dialogs := api.Dialogs{
		Directory: func(title string) (string, error) { return a.runtime().OpenDirectoryDialog(title) },
		File:      func(title string, patterns []string) (string, error) { return a.runtime().OpenFileDialog(title, patterns) },
	}
	a.session = api.NewSession(d.Service, a.context, a.openURL)
	a.portfolio = api.NewPortfolio(d.Service, a.context)
	a.settings = api.NewSettings(d.Service, a.context)
	a.chart = api.NewChart(d.Service, a.context)
	a.instruments = api.NewInstruments(d.Service, a.context)
	a.quotes = api.NewQuotes(d.Service, a.context)
	a.watchlists = api.NewWatchlists(d.Service, a.context)
	a.data = api.NewData(d.Service, a.context, dialogs)
	a.calendar = api.NewCalendar(d.Service, a.context, dialogs)
	a.diagnostics = api.NewDiagnostics(d.Service, a.context, a.openURL)
	a.system = api.NewSystem(d.Service, a.context)
	return a
}

func (a *App) context() context.Context {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ctx
}

func (a *App) runtime() Runtime {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rt
}

func (a *App) openURL(u string) { a.runtime().OpenURL(u) }

// Bind lists the structs Wails exposes to the frontend (ARCH §8.4), in the canonical order of contract §C.
func (a *App) Bind() []any {
	return []any{a.session, a.portfolio, a.settings, a.chart, a.instruments, a.quotes, a.watchlists, a.data, a.calendar, a.diagnostics, a.system}
}

// Startup is options.App.OnStartup. Wails runs it on its own goroutine.
func (a *App) Startup(ctx context.Context) { a.start(ctx, wailsRuntime(ctx)) }

func (a *App) start(parent context.Context, rt Runtime) {
	ctx, cancel := context.WithCancel(parent)
	a.mu.Lock()
	a.ctx, a.cancel, a.rt = ctx, cancel, rt
	a.mu.Unlock()
	a.restoreGeometry(ctx, rt)
	if a.d.StartPlatform != nil {
		if err := a.d.StartPlatform(ctx); err != nil {
			a.d.Log.Error("platform start failed", "err", err)
		}
	}
	emit := a.emitter(rt)
	for _, src := range a.d.Service.EventSources() {
		src.Start(ctx, emit)
	}
	logging.Go("service.session", func() { a.d.Service.RunSession(ctx) })
	logging.Go("service.run", func() { a.d.Service.Run(ctx) })
	logging.Go("service.quotes", func() { a.d.Service.RunQuotes(ctx) })
	logging.Go("service.backups", func() { a.d.Service.RunBackups(ctx) })
	logging.Go("service.restore", func() { a.d.Service.Restore(ctx) })
}

// restoreGeometry applies the geometry saved by BeforeClose (ignored when absent or unparsable).
func (a *App) restoreGeometry(ctx context.Context, rt Runtime) {
	raw, err := a.d.Service.GetUiState(ctx, UiStateWindowGeometry)
	if err != nil || strings.TrimSpace(raw) == "" {
		return
	}
	var g WindowGeometry
	if err := json.Unmarshal([]byte(raw), &g); err != nil || g.W <= 0 || g.H <= 0 {
		a.d.Log.Warn("ignoring saved window geometry", "value", raw)
		return
	}
	rt.SetWindowGeometry(g.W, g.H, g.X, g.Y, g.Maximised)
}

// emitter forwards events to the UI, focuses the window when a login completes and quits for a restart
// once a restore job has finished (A5).
func (a *App) emitter(rt Runtime) service.Emit {
	return func(name string, payload any) {
		switch name {
		case service.EvSessionState:
			if st, ok := payload.(service.SessionStatus); ok {
				a.mu.Lock()
				prev := a.lastState
				a.lastState = st.State
				a.mu.Unlock()
				if prev == "logging_in" && st.State == "ready" {
					rt.Focus()
				}
			}
		case service.EvJobProgress:
			if jp, ok := payload.(service.JobProgress); ok && jp.Kind == "restore" && jp.Finished && jp.Error == "" && jp.Stage == "restart" {
				rt.Emit(name, payload)
				a.RequestRestart()
				a.d.Log.Info("restore finished; restarting the application")
				rt.Quit()
				return
			}
		}
		rt.Emit(name, payload)
	}
}

func (a *App) Shutdown(ctx context.Context) {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if a.d.StopPlatform != nil {
		a.d.StopPlatform(ctx)
	}
}

// BeforeClose saves the window geometry and runs the scheduled backup when it is due (FR-DATA-005 "on quit").
// It never prevents closing in P0; P4 adds the active-deployments prompt (FR-DEP-020).
func (a *App) BeforeClose(ctx context.Context) bool {
	rt := a.runtime()
	w, h, x, y, max := rt.WindowGeometry()
	if w > 0 && h > 0 {
		b, _ := json.Marshal(WindowGeometry{W: w, H: h, X: x, Y: y, Maximised: max})
		if err := a.d.Service.SetUiState(ctx, UiStateWindowGeometry, string(b)); err != nil {
			a.d.Log.Warn("saving the window geometry failed", "err", err)
		}
	}
	a.d.Service.BackupOnQuit(ctx)
	return false
}

// RequestRestart marks that the process must re-exec itself after the window closes (a finished
// restore, FR-DATA-005): cmd/tradeloom.run returns exit code 3 and main re-execs the binary.
func (a *App) RequestRestart() { a.restart.Store(true) }

// RestartRequested reports whether RequestRestart was called.
func (a *App) RestartRequested() bool { return a.restart.Load() }

func wailsRuntime(ctx context.Context) Runtime {
	return Runtime{
		Emit:    func(n string, p any) { runtime.EventsEmit(ctx, n, p) },
		OpenURL: func(u string) { runtime.BrowserOpenURL(ctx, u) },
		Focus:   func() { runtime.WindowUnminimise(ctx); runtime.WindowShow(ctx) },
		Quit:    func() { runtime.Quit(ctx) },
		OpenDirectoryDialog: func(title string) (string, error) {
			return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: title})
		},
		OpenFileDialog: func(title string, patterns []string) (string, error) {
			return runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{Title: title,
				Filters: []runtime.FileFilter{{DisplayName: title, Pattern: strings.Join(patterns, ";")}}})
		},
		WindowGeometry: func() (int, int, int, int, bool) {
			w, h := runtime.WindowGetSize(ctx)
			x, y := runtime.WindowGetPosition(ctx)
			return w, h, x, y, runtime.WindowIsMaximised(ctx)
		},
		SetWindowGeometry: func(w, h, x, y int, maximised bool) {
			runtime.WindowSetSize(ctx, w, h)
			runtime.WindowSetPosition(ctx, x, y)
			if maximised {
				runtime.WindowMaximise(ctx)
			}
		},
	}
}
```

```go
// internal/app/errors.go
package app

import (
	"encoding/json"
	"errors"

	"tradeloom/internal/service"
)

// ErrorFormatter turns a Go error into the JSON string the UI parses (ARCH §8.4).
func ErrorFormatter(err error) any {
	m := map[string]string{"code": "internal", "stage": "", "reason": err.Error()}
	var r *service.ErrRefused
	if errors.As(err, &r) {
		m = map[string]string{"code": r.Code, "stage": r.Stage, "reason": r.Reason}
	}
	b, _ := json.Marshal(m)
	return string(b)
}
```

```go
// internal/app/options.go
package app

import (
	"context"
	"io/fs"

	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Options builds the Wails application options (single instance per data directory, ARCH §3.7).
// smoke is P0-1's --smoke flag: the window quits as soon as the DOM is ready (install tests).
func Options(a *App, assets fs.FS, uniqueID string, smoke bool) *options.App {
	o := &options.App{
		Title:                    "Tradeloom",
		Width:                    1440,
		Height:                   900,
		MinWidth:                 1280,
		MinHeight:                720,
		AssetServer:              &assetserver.Options{Assets: assets},
		OnStartup:                a.Startup,
		OnShutdown:               a.Shutdown,
		OnBeforeClose:            a.BeforeClose,
		Bind:                     a.Bind(),
		ErrorFormatter:           ErrorFormatter,
		EnableDefaultContextMenu: false,
		LogLevel:                 logger.INFO,
		LogLevelProduction:       logger.ERROR,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               uniqueID,
			OnSecondInstanceLaunch: a.SecondInstance,
		},
	}
	if smoke {
		o.OnDomReady = func(ctx context.Context) { runtime.Quit(ctx) }
	}
	return o
}
```

`a.SecondInstance` is implemented in Task 23. Until then, add this temporary stub at the end of `internal/app/options.go` so the package compiles. Task 23 Step 3 deletes it.

```go
// SecondInstance is replaced by the real handler in internal/app/singleinstance.go (Task 23).
func (a *App) SecondInstance(options.SecondInstanceData) { a.runtime().Focus() }
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/app/... -v`
Expected: PASS (`TestSessionLogin_OpensSystemBrowser`, `TestBindings_AreJSONFriendly`, `TestDialogs_PassThrough`, `TestDiagnostics_OpensLogsFolder`, `TestApp_FocusesWindowOnLoginSuccess`, `TestErrorFormatter`, `TestOptions`, `TestRestartRequested`, `TestApp_RestoresAndSavesWindowGeometry`, `TestApp_QuitsAfterRestoreJob`, `TestGenerate`, plus P0-1's `TestStart_*` and `TestExitMessage` from `boot_test.go`). `internal/app` imports `pkg/runtime`, which has a cgo file on Linux (`signal_linux.go`), so run this in the P0-1 Linux build container if the host lacks a C toolchain. `cmd/tradeloom` does not compile between this step and Task 23 Step 5 (it still calls P0-1's `app.New()`); Task 23 fixes it before the next `go build ./...`.

- [ ] **Step 6: Add the `bindings` Makefile target and generate the TypeScript bindings**

Append to `Makefile` (P0-1 defines `WAILS_PROJECT_DIR` as the directory holding `wails.json`; add the variable if it is missing):

```make
WAILS_PROJECT_DIR ?= .

.PHONY: bindings
bindings: ## Regenerate frontend/wailsjs (Go → TS) and frontend/src/api/events.gen.ts
	cd $(WAILS_PROJECT_DIR) && wails generate module -tags "webkit2_41,noorders"
	go generate -tags webkit2_41,noorders ./internal/app/
```

Run: `make bindings && ls frontend/wailsjs/go/api/ && grep -c "export function" frontend/wailsjs/go/api/*.d.ts && cd frontend && npx tsc --noEmit`
Expected: the 22 files `Calendar.d.ts Calendar.js Chart.d.ts Chart.js Data.d.ts Data.js Diagnostics.d.ts Diagnostics.js Instruments.d.ts Instruments.js Portfolio.d.ts Portfolio.js Quotes.d.ts Quotes.js Session.d.ts Session.js Settings.d.ts Settings.js System.d.ts System.js Watchlists.d.ts Watchlists.js`, then the per-file function counts `Calendar.d.ts:5 Chart.d.ts:4 Data.d.ts:8 Diagnostics.d.ts:5 Instruments.d.ts:3 Portfolio.d.ts:4 Quotes.d.ts:1 Session.d.ts:17 Settings.d.ts:7 System.d.ts:2 Watchlists.d.ts:9` (65 bindings), then `tsc` exits 0. Also check `grep "export function Login():Promise<kite.LoginStart>" frontend/wailsjs/go/api/Session.d.ts` prints one line and `grep "^export namespace" frontend/wailsjs/go/models.ts` prints exactly six lines: `service`, `kite`, `orders`, `settings`, `ticker` and `clock` (the calendar DTOs are `service.*`; no binding returns a `backfill` or `calendar` type).

- [ ] **Step 7: Commit**

```bash
git add internal/app internal/service/servicetest Makefile frontend/wailsjs
git commit -m "feat(app): eleven Wails-bound API structs, lifecycle with window geometry and restart, error format and generated TS API

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
