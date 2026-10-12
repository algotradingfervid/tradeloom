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

// SecondInstance is options.SingleInstanceLock.OnSecondInstanceLaunch: focus; route a Kite redirect to the
// login (callback rule: the current attempt's state nonce is required, Ruling 57); "--view <route>" asks
// the UI router to show that route (app.navigate).
// Wails calls it on its own goroutine, so a panic here is recorded as a crash report (Ruling 89 M2); a
// launch that arrives before Startup installed the runtime is logged and ignored.
func (a *App) SecondInstance(data options.SecondInstanceData) {
	defer logging.Recover("app.secondinstance")
	rt := a.runtime()
	if rt.Focus == nil {
		a.d.Log.Warn("second launch arrived before the window runtime was ready; ignored", "args", len(data.Args))
		return
	}
	rt.Focus()
	for i, arg := range data.Args {
		if strings.Contains(arg, "request_token=") {
			ctx := a.context()
			a.d.Log.Info("second launch carried a Kite redirect; completing the login")
			logging.Go("app.secondinstance.login", func() {
				if _, err := a.d.Service.CompleteForwardedLogin(ctx, arg); err != nil {
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
