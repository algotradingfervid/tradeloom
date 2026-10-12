> **P0-1 Platform Core Implementation Plan — Task 6.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 6: `internal/store` — data directory and single-instance lock

**Files:**
- Create: `internal/store/datadir.go`, `internal/store/lock.go`, `internal/store/datadir_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces (`package store`):
  - `const EnvDataDir = "TRADELOOM_DATA_DIR"`; `func ResolveDataDir(getenv func(string) string, home string) (string, error)`
  - `type Layout struct { Root, AppDB, CandlesDir, CandlesDB, LogsDir, ExportsDir, BackupsDir, LibraryDir, InstrumentsDir, SecretsFile, LockFile string }`; `func NewLayout(root string) Layout`; `func (l Layout) Ensure() error`
  - `var ErrLocked`; `type ExitState string` with `ExitFirstRun = "first_run"`, `ExitClean = "clean"`, `ExitUnclean = "unclean"`
  - `type Lock struct{…}`; `func AcquireLock(l Layout, pid int) (*Lock, error)`; `func (k *Lock) PreviousExit() ExitState`; `func (k *Lock) MarkClean() error`; `func (k *Lock) Release() error`
  - P0-3's Wails `SingleInstanceLock` (focus the running window) sits on top of this lock; this lock stays authoritative.

- [ ] **Step 1: Write the failing test**

`internal/store/datadir_test.go`:

```go
package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDataDir(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{"default", nil, "/home/u/.local/share/tradeloom", false},
		{"xdg", map[string]string{"XDG_DATA_HOME": "/x"}, "/x/tradeloom", false},
		{"override wins", map[string]string{"XDG_DATA_HOME": "/x", EnvDataDir: "/d/t"}, "/d/t", false},
		{"relative override rejected", map[string]string{EnvDataDir: "rel"}, "", true},
		{"relative xdg ignored", map[string]string{"XDG_DATA_HOME": "rel"}, "/home/u/.local/share/tradeloom", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveDataDir(env(tc.env), "/home/u")
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestLayoutEnsure(t *testing.T) {
	l := NewLayout(filepath.Join(t.TempDir(), "tl"))
	if err := l.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{l.Root, l.CandlesDir, l.LogsDir, l.ExportsDir, l.BackupsDir, l.LibraryDir, l.InstrumentsDir} {
		st, err := os.Stat(d)
		if err != nil || !st.IsDir() || st.Mode().Perm() != 0o700 {
			t.Fatalf("%s: %v %v", d, st, err)
		}
	}
}

func TestLock_SingleHolderAndExitMarker(t *testing.T) {
	l := NewLayout(t.TempDir())
	k1, err := AcquireLock(l, 100)
	if err != nil {
		t.Fatal(err)
	}
	if k1.PreviousExit() != ExitFirstRun {
		t.Fatalf("first run: %s", k1.PreviousExit())
	}
	if _, err := AcquireLock(l, 101); !errors.Is(err, ErrLocked) {
		t.Fatalf("second holder: want ErrLocked, got %v", err)
	}
	// Simulated crash: release without MarkClean.
	if err := k1.Release(); err != nil {
		t.Fatal(err)
	}
	k2, err := AcquireLock(l, 102)
	if err != nil {
		t.Fatal(err)
	}
	if k2.PreviousExit() != ExitUnclean {
		t.Fatalf("after crash: %s", k2.PreviousExit())
	}
	if err := k2.MarkClean(); err != nil {
		t.Fatal(err)
	}
	k2.Release()
	k3, err := AcquireLock(l, 103)
	if err != nil {
		t.Fatal(err)
	}
	defer k3.Release()
	if k3.PreviousExit() != ExitClean {
		t.Fatalf("after clean exit: %s", k3.PreviousExit())
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders ./internal/store/`
Expected: FAIL — build error `undefined: ResolveDataDir`.

- [ ] **Step 3: Write the implementation**

```bash
go get github.com/gofrs/flock@v0.13.1
```

`internal/store/datadir.go`:

```go
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// EnvDataDir overrides the data directory (FR-DATA-005).
const EnvDataDir = "TRADELOOM_DATA_DIR"

// ResolveDataDir returns $TRADELOOM_DATA_DIR, else $XDG_DATA_HOME/tradeloom, else <home>/.local/share/tradeloom.
// Relative overrides are rejected so the lock and the backups never depend on the working directory.
func ResolveDataDir(getenv func(string) string, home string) (string, error) {
	if v := getenv(EnvDataDir); v != "" {
		if !filepath.IsAbs(v) {
			return "", fmt.Errorf("store: %s must be an absolute path, got %q", EnvDataDir, v)
		}
		return filepath.Clean(v), nil
	}
	if x := getenv("XDG_DATA_HOME"); x != "" && filepath.IsAbs(x) {
		return filepath.Join(x, "tradeloom"), nil
	}
	if home == "" {
		return "", errors.New("store: cannot resolve the data directory: no home directory")
	}
	return filepath.Join(home, ".local", "share", "tradeloom"), nil
}

// Layout names every path of the data directory (ARCHITECTURE §4.5).
type Layout struct {
	Root           string
	AppDB          string // app.db
	CandlesDir     string // candles/
	CandlesDB      string // candles/candles.db
	LogsDir        string // logs/
	ExportsDir     string // exports/
	BackupsDir     string // backups/ (automatic pre-migration copies)
	LibraryDir     string // library/
	InstrumentsDir string // instruments/
	SecretsFile    string // secrets.enc (only with the encrypted-file backend)
	LockFile       string // lock (flock + last-exit marker)
}

// NewLayout derives every path from root.
func NewLayout(root string) Layout {
	j := func(p ...string) string { return filepath.Join(append([]string{root}, p...)...) }
	return Layout{
		Root: root, AppDB: j("app.db"), CandlesDir: j("candles"), CandlesDB: j("candles", "candles.db"),
		LogsDir: j("logs"), ExportsDir: j("exports"), BackupsDir: j("backups"), LibraryDir: j("library"),
		InstrumentsDir: j("instruments"), SecretsFile: j("secrets.enc"), LockFile: j("lock"),
	}
}

// Ensure creates the root and every sub-directory with mode 0700.
func (l Layout) Ensure() error {
	for _, d := range []string{l.Root, l.CandlesDir, l.LogsDir, l.ExportsDir, l.BackupsDir, l.LibraryDir, l.InstrumentsDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("store: create %s: %w", d, err)
		}
	}
	return nil
}
```

`internal/store/lock.go` (flock(2) locks belong to an open file description, so a second `AcquireLock` on the same path fails even inside one process — which is what the test relies on):

```go
package store

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gofrs/flock"
)

// ErrLocked means another process holds the data-directory lock.
var ErrLocked = errors.New("store: data directory is in use by another Tradeloom process")

// ExitState is what the lock file says about the previous run.
type ExitState string

const (
	ExitFirstRun ExitState = "first_run" // no marker: new data directory
	ExitClean    ExitState = "clean"     // previous run called MarkClean
	ExitUnclean  ExitState = "unclean"   // previous run died while holding the lock (crash, kill, power loss)
)

const (
	markerRunning = "running"
	markerClean   = "clean"
)

// Lock is the authoritative single-instance lock on <datadir>/lock (ARCHITECTURE §3.7).
type Lock struct {
	fl   *flock.Flock
	path string
	prev ExitState
}

// AcquireLock takes an exclusive non-blocking flock on l.LockFile and records "running pid=<pid>".
// It returns ErrLocked (wrapped with the path) when another process holds it.
func AcquireLock(l Layout, pid int) (*Lock, error) {
	fl := flock.New(l.LockFile)
	ok, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("store: lock %s: %w", l.LockFile, err)
	}
	if !ok {
		return nil, fmt.Errorf("%w (data dir %s)", ErrLocked, l.Root)
	}
	prev := ExitFirstRun
	b, err := os.ReadFile(l.LockFile)
	if err != nil {
		fl.Close()
		return nil, err
	}
	switch s := strings.TrimSpace(string(b)); {
	case s == markerClean:
		prev = ExitClean
	case strings.HasPrefix(s, markerRunning):
		prev = ExitUnclean
	}
	if err := os.WriteFile(l.LockFile, []byte(fmt.Sprintf("%s pid=%d\n", markerRunning, pid)), 0o600); err != nil {
		fl.Close()
		return nil, err
	}
	return &Lock{fl: fl, path: l.LockFile, prev: prev}, nil
}

// PreviousExit reports how the previous run ended (FR-SHELL-018 AC2 uses ExitUnclean).
func (k *Lock) PreviousExit() ExitState { return k.prev }

// MarkClean records a clean exit; call it as the last step of OnShutdown, before Release.
func (k *Lock) MarkClean() error { return os.WriteFile(k.path, []byte(markerClean+"\n"), 0o600) }

// Release unlocks and closes the lock file. The marker is left as written.
func (k *Lock) Release() error { return k.fl.Close() }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go mod tidy && go test -count=1 -tags noorders ./internal/store/ ./internal/archtest/`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/store
git commit -m "feat(store): data-dir resolution and layout, flock single-instance lock with last-exit marker" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
