// Package app is the Wails glue (ARCHITECTURE §2.2): lifecycle callbacks, bound API structs
// and the bus → runtime.EventsEmit bridge. P0-1 provides the hello-window App (Task 1) and Boot (Task 15);
// P0-3 adds bindings, the event bridge and the Wails single-instance callback.
package app

import (
	"context"
)

// Version is set at build time: -ldflags "-X tradeloom/internal/app.Version=v0.1.0".
var Version = "dev"

// App is bound to the frontend. Exported methods become TypeScript functions in frontend/wailsjs.
type App struct {
	ctx context.Context
}

// New returns the hello-window App.
func New() *App { return &App{} }

// OnStartup is options.App.OnStartup; Wails passes the runtime context used by runtime.* calls.
func (a *App) OnStartup(ctx context.Context) { a.ctx = ctx }

// Hello returns the window greeting; it proves bindings work end to end.
func (a *App) Hello() string { return "Tradeloom " + Version }
