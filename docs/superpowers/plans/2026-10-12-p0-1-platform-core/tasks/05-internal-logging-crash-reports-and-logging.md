> **P0-1 Platform Core Implementation Plan — Task 5.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 5: `internal/logging` — crash reports and `logging.Go`

**Files:**
- Create: `internal/logging/crash.go`, `internal/logging/crash_test.go`

**Interfaces:**
- Consumes: Task 4 `Redactor`, `Default`, `RedactString`, `ScrubBytes`, `NewRedactor`.
- Produces (`package logging`):
  - `type CrashOptions struct { Dir string; AppVersion string; SchemaVersion func() int; State func() map[string]any; Now func() time.Time; Redactor *Redactor }`
  - `type CrashReport struct { At string; AppVersion string; SchemaVersion int; Goroutine string; Panic string; Stack string; State map[string]any }` (JSON keys `at`, `app_version`, `schema_version`, `goroutine`, `panic`, `stack`, `state`)
  - `func InstallCrashReporter(o CrashOptions)`; `func Go(name string, fn func())`; `func Recover(name string)` (use as `defer logging.Recover("name")`; re-panics after writing the report)
  - `func WriteCrashReport(goroutine string, v any, stack []byte) (string, error)` → `logs/crash-<UTC 20060102T150405.000Z>.json`
  - `func UnseenCrashReports(dir string) ([]string, error)` (newest first); `func MarkCrashSeen(path string) error` (renames to `crash-….seen.json`) — P0-4's "The app crashed last time — view report" uses these two.

- [ ] **Step 1: Write the failing test**

`internal/logging/crash_test.go`:

```go
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders -run TestGo_PanicWritesCrashReport ./internal/logging/`
Expected: FAIL — build error `undefined: InstallCrashReporter`.

- [ ] **Step 3: Write the implementation**

`internal/logging/crash.go`:

```go
package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

// CrashOptions configures the crash reporter installed by InstallCrashReporter.
type CrashOptions struct {
	Dir           string                // <datadir>/logs
	AppVersion    string                // internal/app.Version
	SchemaVersion func() int            // current app.db schema; nil reports 0
	State         func() map[string]any // state summary without secrets; nil reports none
	Now           func() time.Time      // clock.SystemNow in production
	Redactor      *Redactor             // nil means Default
}

// CrashReport is the JSON document written to logs/crash-<timestamp>.json.
type CrashReport struct {
	At            string         `json:"at"`
	AppVersion    string         `json:"app_version"`
	SchemaVersion int            `json:"schema_version"`
	Goroutine     string         `json:"goroutine"`
	Panic         string         `json:"panic"`
	Stack         string         `json:"stack"`
	State         map[string]any `json:"state,omitempty"`
}

var (
	crashMu  sync.RWMutex
	crashCfg *CrashOptions
	// rethrow re-raises the panic after the report is flushed; tests replace it.
	rethrow = func(v any) { panic(v) }
)

// InstallCrashReporter makes Go/Recover write crash reports. Call once at startup, before any logging.Go.
func InstallCrashReporter(o CrashOptions) {
	if o.Redactor == nil {
		o.Redactor = Default
	}
	crashMu.Lock()
	crashCfg = &o
	crashMu.Unlock()
}

// Go starts fn on a new goroutine with crash reporting. It is the only sanctioned
// way to start a goroutine outside internal/logging and internal/bus (ARCHITECTURE §3.3).
func Go(name string, fn func()) {
	go func() {
		defer Recover(name)
		fn()
	}()
}

// Recover must be deferred directly. On panic it writes a crash report and re-panics.
func Recover(name string) {
	v := recover()
	if v == nil {
		return
	}
	stack := debug.Stack()
	if _, err := WriteCrashReport(name, v, stack); err != nil {
		fmt.Fprintf(os.Stderr, "tradeloom: crash report failed: %v\n", err)
	}
	rethrow(v)
}

// WriteCrashReport writes logs/crash-<UTC yyyymmddThhmmss.mmmZ>.json and returns its path.
func WriteCrashReport(goroutine string, v any, stack []byte) (string, error) {
	crashMu.RLock()
	cfg := crashCfg
	crashMu.RUnlock()
	if cfg == nil {
		return "", fmt.Errorf("logging: crash reporter not installed")
	}
	now := cfg.Now().UTC()
	rep := CrashReport{
		At:         now.Format(time.RFC3339Nano),
		AppVersion: cfg.AppVersion,
		Goroutine:  goroutine,
		Panic:      cfg.Redactor.RedactString(fmt.Sprint(v)),
		Stack:      cfg.Redactor.RedactString(string(stack)),
	}
	if cfg.SchemaVersion != nil {
		rep.SchemaVersion = cfg.SchemaVersion()
	}
	if cfg.State != nil {
		rep.State = cfg.State()
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", err
	}
	b = cfg.Redactor.ScrubBytes(b)
	if err := os.MkdirAll(cfg.Dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(cfg.Dir, "crash-"+now.Format("20060102T150405.000Z")+".json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// UnseenCrashReports lists crash reports not yet shown to the user, newest first.
func UnseenCrashReports(dir string) ([]string, error) {
	all, err := filepath.Glob(filepath.Join(dir, "crash-*.json"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range all {
		if !strings.HasSuffix(p, ".seen.json") {
			out = append(out, p)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}

// MarkCrashSeen renames crash-X.json to crash-X.seen.json so the next start does not offer it again.
func MarkCrashSeen(path string) error {
	return os.Rename(path, strings.TrimSuffix(path, ".json")+".seen.json")
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race -count=1 -tags noorders ./internal/logging/ ./internal/archtest/`
Expected: `ok` for both (archtest confirms `crash.go` calls no banned `time` function — it only formats the injected `Now()`).

- [ ] **Step 5: Commit**

```bash
git add internal/logging/crash.go internal/logging/crash_test.go
git commit -m "feat(logging): crash reports via logging.Go/Recover and unseen-report listing" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
