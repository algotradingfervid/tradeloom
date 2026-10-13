package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
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

// A start-up refused after the lock was taken (damaged app.db) releases the lock and restores the
// previous exit marker: the next start must not report a crash, and a first run stays a first run.
func TestStart_FailedStartRestoresExitMarker(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tradeloom")
	b, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Shutdown(); err != nil {
		t.Fatal(err)
	}
	appDB := filepath.Join(dir, "app.db")
	good, err := os.ReadFile(appDB)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(appDB, bytes.Repeat([]byte("garbage!"), 1024), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // the retry must not hit our own stale lock
		if _, err := Start(context.Background(), bootOpts(dir)); !errors.Is(err, store.ErrCorrupt) {
			t.Fatalf("attempt %d: want ErrCorrupt, got %v", i, err)
		}
	}
	if err := os.WriteFile(appDB, good, 0o600); err != nil {
		t.Fatal(err)
	}
	b2, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Shutdown()
	if b2.PreviousExit != store.ExitClean || b2.CrashedLastTime {
		t.Fatalf("after refused start: %s crashed=%v", b2.PreviousExit, b2.CrashedLastTime)
	}

	fresh := filepath.Join(t.TempDir(), "tradeloom")
	if err := os.MkdirAll(fresh, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fresh, "app.db"), bytes.Repeat([]byte("garbage!"), 1024), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(context.Background(), bootOpts(fresh)); !errors.Is(err, store.ErrCorrupt) {
		t.Fatalf("fresh: want ErrCorrupt, got %v", err)
	}
	os.Remove(filepath.Join(fresh, "app.db"))
	b3, err := Start(context.Background(), bootOpts(fresh))
	if err != nil {
		t.Fatal(err)
	}
	defer b3.Shutdown()
	if b3.PreviousExit != store.ExitFirstRun {
		t.Fatalf("fresh after refused start: %s", b3.PreviousExit)
	}
}

// A restore that crashed between its two renames (root gone, <root>.replaced-* present) is undone
// before Layout.Ensure would create an empty root.
func TestStart_RecoversInterruptedRestore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tradeloom")
	b, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exports", "keep.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := b.Shutdown(); err != nil {
		t.Fatal(err)
	}
	replaced := dir + ".replaced-20261012T100000Z"
	if err := os.Rename(dir, replaced); err != nil {
		t.Fatal(err)
	}
	b2, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Shutdown()
	if got, err := os.ReadFile(filepath.Join(dir, "exports", "keep.txt")); err != nil || string(got) != "mine" {
		t.Fatalf("data not recovered: %q %v", got, err)
	}
	if _, err := os.Stat(replaced); !os.IsNotExist(err) {
		t.Fatalf("replaced dir still present: %v", err)
	}
	if b2.PreviousExit != store.ExitClean {
		t.Fatalf("recovered marker: %s", b2.PreviousExit)
	}
	logb, _ := os.ReadFile(filepath.Join(dir, "logs", "tradeloom.log"))
	if !strings.Contains(string(logb), `"level":"WARN","event":"store.restore_recovered"`) ||
		!strings.Contains(string(logb), replaced) {
		t.Fatalf("no WARN recovery line in log:\n%s", logb)
	}
}

// Stale <root>.restoring-* staging is swept only once the lock is held: a running instance may be
// extracting a restore into it.
func TestStart_SweepsStaleStagingOnlyUnderLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tradeloom")
	b, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	staging := dir + ".restoring-20261012T100000Z"
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(context.Background(), bootOpts(dir)); !errors.Is(err, store.ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err)
	}
	if _, err := os.Stat(staging); err != nil {
		t.Fatalf("staging of the running instance was swept: %v", err)
	}
	if err := b.Shutdown(); err != nil {
		t.Fatal(err)
	}
	b2, err := Start(context.Background(), bootOpts(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Shutdown()
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("stale staging not swept: %v", err)
	}
}

func TestExitMessage_CorruptAndDefault(t *testing.T) {
	err := &store.CorruptError{Path: "/d/app.db", Detail: "bad header", LatestBackup: "/d/backups/app-1.db"}
	m := ExitMessage("/d", fmt.Errorf("open: %w", err))
	if m != "Tradeloom could not start: open: "+err.Error() {
		t.Fatal(m)
	}
}

// Shutdown and close may run more than once (OnShutdown + defer, crash path then exit).
func TestShutdown_Idempotent(t *testing.T) {
	b, err := Start(context.Background(), bootOpts(filepath.Join(t.TempDir(), "tl")))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Shutdown(); err != nil {
		t.Fatal(err)
	}
	b.close()
	if err := b.Shutdown(); err != nil {
		t.Fatal(err)
	}
}
