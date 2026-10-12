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
