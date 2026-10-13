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
