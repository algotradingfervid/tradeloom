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
