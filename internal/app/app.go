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
const UiStateWindowGeometry = service.UiStateWindowGeometry

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
		File: func(title string, patterns []string) (string, error) {
			return a.runtime().OpenFileDialog(title, patterns)
		},
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
// once a restore job has reached StageRestart (A5, Ruling 75).
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
			// Ruling 75: every finished restore that reached StageRestart relaunches, whether or not Error is
			// set (app.db is closed, or the data directory is put back on the next start).
			if jp, ok := payload.(service.JobProgress); ok && jp.Kind == service.JobRestore && jp.Finished && jp.Stage == service.StageRestart {
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

// BeforeClose saves the window geometry (the key is backend-owned, so through SaveWindowGeometry) and runs the scheduled backup when it is due (FR-DATA-005 "on quit").
// It never prevents closing in P0; P4 adds the active-deployments prompt (FR-DEP-020).
func (a *App) BeforeClose(ctx context.Context) bool {
	rt := a.runtime()
	w, h, x, y, max := rt.WindowGeometry()
	if w > 0 && h > 0 {
		b, _ := json.Marshal(WindowGeometry{W: w, H: h, X: x, Y: y, Maximised: max})
		if err := a.d.Service.SaveWindowGeometry(ctx, string(b)); err != nil {
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
