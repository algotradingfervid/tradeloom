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
