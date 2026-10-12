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
	// app.Start runs store.RecoverInterruptedRestore before Ensure and again under the lock (P0-1 Ruling 30/31).
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
	defer app.CaptureStdStreams(boot.Log.Logger)() // before the first kiteconnect client; restored (LIFO) before boot.Shutdown closes the log

	platform, err := buildPlatform(context.Background(), boot)
	if err != nil {
		boot.Log.Error("platform.build_failed", "component", "app", "err", err)
		fmt.Fprintln(stderr, "Tradeloom could not start:", err)
		return 1
	}
	a := wireP03(platform)
	opts := app.Options(a, frontend.Assets(), app.UniqueID(dataDir), *smoke)
	closeLiveOnReload(opts, platform.Service.CloseAllLive)
	if err := wails.Run(opts); err != nil {
		boot.Log.Error("wails.exited", "component", "app", "err", err)
		fmt.Fprintln(stderr, "tradeloom:", err)
		platform.stopAll(context.Background()) // App.Shutdown may not have run
		return 1
	}
	platform.stopAll(context.Background()) // no-op after App.Shutdown; covers a Wails exit without OnShutdown
	return finalExit(a.RestartRequested())
}

// finalExit maps a clean window exit to the process exit code.
func finalExit(restart bool) int {
	if restart {
		return exitRestart
	}
	return 0
}
