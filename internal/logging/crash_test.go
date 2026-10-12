package logging

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// FR-SHELL-018 AC2 (engine side): a panic in a background goroutine leaves a crash report
// that the next start finds via UnseenCrashReports.
func TestGo_PanicWritesCrashReport(t *testing.T) {
	dir := t.TempDir()
	r := NewRedactor()
	r.Register(tAccess)
	InstallCrashReporter(CrashOptions{
		Dir: dir, AppVersion: "v0.1.0", Redactor: r,
		SchemaVersion: func() int { return 1 },
		State:         func() map[string]any { return map[string]any{"session": "ready"} },
		Now:           func() time.Time { return time.Date(2026, 10, 12, 4, 30, 0, 0, time.UTC) },
	})
	got := make(chan any, 1)
	old := rethrow
	rethrow = func(v any) { got <- v }
	t.Cleanup(func() { rethrow = old })

	Go("backfill.worker", func() { panic("boom with " + tAccess) })
	select {
	case v := <-got:
		if !strings.Contains(v.(string), "boom") {
			t.Fatalf("rethrown value %v", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("panic was not recovered")
	}
	reps, err := UnseenCrashReports(dir)
	if err != nil || len(reps) != 1 {
		t.Fatalf("want 1 crash report, got %v (%v)", reps, err)
	}
	if !strings.HasSuffix(reps[0], "crash-20261012T043000.000Z.json") {
		t.Fatalf("unexpected name %s", reps[0])
	}
	if fi, err := os.Stat(reps[0]); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("crash report mode %v (%v), want 0600", fi.Mode().Perm(), err)
	}
	b, _ := os.ReadFile(reps[0])
	if strings.Contains(string(b), tAccess) {
		t.Fatal("crash report contains a registered secret")
	}
	var rep CrashReport
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	if rep.AppVersion != "v0.1.0" || rep.SchemaVersion != 1 || rep.Goroutine != "backfill.worker" || rep.Stack == "" || rep.State["session"] != "ready" {
		t.Fatalf("incomplete report: %+v", rep)
	}
	if err := MarkCrashSeen(reps[0]); err != nil {
		t.Fatal(err)
	}
	if reps, _ := UnseenCrashReports(dir); len(reps) != 0 {
		t.Fatalf("seen report still listed: %v", reps)
	}
}
