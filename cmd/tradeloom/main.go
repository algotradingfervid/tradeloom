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
