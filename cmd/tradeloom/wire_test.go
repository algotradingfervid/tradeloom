package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v2/pkg/options"

	"tradeloom/internal/app"
	"tradeloom/internal/secrets"
)

// testBoot runs P0-1's boot on a fresh temp data dir (never the user's) with the keyring probe stubbed out.
func testBoot(t *testing.T) *app.Boot {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tradeloom")
	boot, err := app.Start(context.Background(), app.BootOptions{DataDir: dir, AppVersion: "test", PID: 1,
		Now: func() time.Time { return time.Unix(0, 0) }, ProbeKeyring: func() error { return secrets.ErrUnavailable }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = boot.Shutdown() })
	return boot
}

func lockMarker(t *testing.T, boot *app.Boot) string {
	t.Helper()
	b, err := os.ReadFile(boot.Layout.LockFile)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

// The whole composition root builds on a real Boot without network, and the §3.3 stop is idempotent.
func TestWire_BuildsAndStopsCleanly(t *testing.T) {
	boot := testBoot(t)
	p, err := buildPlatform(context.Background(), boot)
	if err != nil {
		t.Fatal(err)
	}
	a := wireP03(p)
	if a == nil || p.Service == nil || p.Session == nil || len(a.Bind()) != 11 {
		t.Fatal("wiring incomplete")
	}
	if p.Secrets.Locked() || p.Secrets.Backend() != secrets.BackendNone {
		t.Fatalf("choose mode must start as no storage: locked %v backend %s", p.Secrets.Locked(), p.Secrets.Backend())
	}
	p.stopAll(context.Background())
	p.stopAll(context.Background())
	if err := boot.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if m := lockMarker(t, boot); m != "clean" {
		t.Fatalf("lock marker %q", m)
	}
}

// A restore closes the platform and app.db early; the later shutdown still marks a clean exit.
func TestWire_CloseForRestoreThenShutdownIsClean(t *testing.T) {
	boot := testBoot(t)
	p, err := buildPlatform(context.Background(), boot)
	if err != nil {
		t.Fatal(err)
	}
	wireP03(p)
	if err := p.CloseForRestore(); err != nil {
		t.Fatal(err)
	}
	p.stopAll(context.Background())
	if err := boot.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if m := lockMarker(t, boot); m != "clean" {
		t.Fatalf("lock marker %q", m)
	}
}

// Task 27 / Ruling 63: an existing secrets.enc starts locked; the keyring is usable at once; choose = no storage.
func TestSecretSwitch_Modes(t *testing.T) {
	if sw := secretSwitch(secrets.Detection{Mode: secrets.ModeFile, FileExists: true}); !sw.Locked() {
		t.Fatal("file mode must start locked")
	}
	if sw := secretSwitch(secrets.Detection{Mode: secrets.ModeKeyring, KeyringAvailable: true}); sw.Locked() || sw.Backend() != secrets.BackendKeyring {
		t.Fatalf("keyring: locked %v backend %s", sw.Locked(), sw.Backend())
	}
	if sw := secretSwitch(secrets.Detection{Mode: secrets.ModeChoose}); sw.Locked() || sw.Backend() != secrets.BackendNone {
		t.Fatalf("choose: locked %v backend %s", sw.Locked(), sw.Backend())
	}
}

// Ruling 66: every page load (a reload) closes the live charts before the existing OnDomReady runs.
func TestCloseLiveOnReload_ChainsDomReady(t *testing.T) {
	var calls []string
	o := &options.App{OnDomReady: func(context.Context) { calls = append(calls, "prev") }}
	closeLiveOnReload(o, func(context.Context) int { calls = append(calls, "close"); return 0 })
	o.OnDomReady(context.Background())
	if strings.Join(calls, ",") != "close,prev" {
		t.Fatalf("calls %v", calls)
	}
	o2 := &options.App{}
	closeLiveOnReload(o2, func(context.Context) int { calls = append(calls, "close2"); return 0 })
	o2.OnDomReady(context.Background())
	if calls[len(calls)-1] != "close2" {
		t.Fatalf("calls %v", calls)
	}
}
