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
