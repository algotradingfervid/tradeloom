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
