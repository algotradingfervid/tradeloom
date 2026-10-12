package store

import (
	"archive/tar"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	manifestName   = "manifest.json"
	archiveAppDB   = "app.db"
	archiveCandles = "candles/candles.db"
	backupPrefix   = "tradeloom-backup-"
	backupSuffix   = ".tar.zst"
	// ManifestFormat is bumped when the archive layout changes.
	ManifestFormat = 1
)

// Manifest is manifest.json inside a backup archive.
type Manifest struct {
	Format          int            `json:"format"`
	CreatedAt       string         `json:"created_at"` // RFC 3339 UTC
	AppVersion      string         `json:"app_version"`
	Schema          int            `json:"schema"`
	CalendarVersion string         `json:"calendar_version"`
	LibraryVersion  string         `json:"library_version"`
	IncludesCandles bool           `json:"includes_candles"`
	Counts          map[string]int `json:"counts"` // strategies, scanners, backtest_runs, deployments, watchlists, order_journal
}

// BackupOptions configures (*DB).Backup.
type BackupOptions struct {
	Dir             string // destination folder (Layout.BackupsDir or the user's scheduled-backup folder)
	IncludeCandles  bool
	CandlesDB       string // Layout.CandlesDB; skipped when the file does not exist
	AppVersion      string
	CalendarVersion string
	LibraryVersion  string
	Now             func() time.Time
}

type tempFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}

// createTemp is replaced in tests to simulate a full disk.
var createTemp = func(dir, pattern string) (tempFile, error) { return os.CreateTemp(dir, pattern) }

// Counts returns the restore-preview counts; tables that do not exist yet count 0.
func (db *DB) Counts(ctx context.Context) (map[string]int, error) {
	q := map[string]string{
		"strategies":    "SELECT count(*) FROM documents WHERE kind='strategy' AND deleted_at IS NULL",
		"scanners":      "SELECT count(*) FROM documents WHERE kind='scanner' AND deleted_at IS NULL",
		"backtest_runs": "SELECT count(*) FROM backtest_runs WHERE deleted_at IS NULL",
		"deployments":   "SELECT count(*) FROM deployments WHERE deleted_at IS NULL",
		"watchlists":    "SELECT count(*) FROM watchlists WHERE deleted_at IS NULL",
		"order_journal": "SELECT count(*) FROM order_journal",
	}
	table := map[string]string{"strategies": "documents", "scanners": "documents", "backtest_runs": "backtest_runs",
		"deployments": "deployments", "watchlists": "watchlists", "order_journal": "order_journal"}
	out := map[string]int{}
	for k, stmt := range q {
		var exists int
		if err := db.R.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table[k]).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			out[k] = 0
			continue
		}
		var n int
		if err := db.R.QueryRowContext(ctx, stmt).Scan(&n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, nil
}

// Backup writes <Dir>/tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst (FR-DATA-005 "Backup now").
// Contents: manifest.json, app.db (VACUUM INTO snapshot, WAL included) and optionally candles/candles.db.
// secrets.enc, lock and logs are never read. The archive is written to a temp file and renamed:
// on any error (including a full disk) no file with the final name exists and the temp file is removed.
func (db *DB) Backup(ctx context.Context, o BackupOptions) (string, error) {
	if err := os.MkdirAll(o.Dir, 0o700); err != nil {
		return "", err
	}
	now := o.Now().UTC()
	final := filepath.Join(o.Dir, backupPrefix+now.Format("20060102-150405")+backupSuffix)
	stage, err := os.MkdirTemp(o.Dir, ".backup-stage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)

	appSnap := filepath.Join(stage, "app.db")
	if _, err := db.W.ExecContext(ctx, "VACUUM INTO ?", appSnap); err != nil {
		return "", fmt.Errorf("store: snapshot app.db: %w", err)
	}
	if err := os.Chmod(appSnap, 0o600); err != nil {
		return "", err
	}
	counts, err := db.Counts(ctx)
	if err != nil {
		return "", err
	}
	files := []struct{ name, path string }{{archiveAppDB, appSnap}}
	includes := false
	if o.IncludeCandles {
		if _, err := os.Stat(o.CandlesDB); err == nil {
			snap := filepath.Join(stage, "candles.db")
			if err := vacuumInto(ctx, o.CandlesDB, snap); err != nil {
				return "", fmt.Errorf("store: snapshot candles.db: %w", err)
			}
			files = append(files, struct{ name, path string }{archiveCandles, snap})
			includes = true
		}
	}
	man := Manifest{Format: ManifestFormat, CreatedAt: now.Format(time.RFC3339), AppVersion: o.AppVersion,
		Schema: db.schema, CalendarVersion: o.CalendarVersion, LibraryVersion: o.LibraryVersion,
		IncludesCandles: includes, Counts: counts}
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return "", err
	}

	tmp, err := createTemp(o.Dir, ".backup-*.tar.zst.tmp")
	if err != nil {
		return "", err
	}
	committed := false
	defer func() {
		if !committed {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if f, ok := tmp.(interface{ Chmod(os.FileMode) error }); ok {
		if err := f.Chmod(0o600); err != nil {
			return "", fmt.Errorf("store: chmod backup: %w", err)
		}
	}
	if err := writeArchive(tmp, manBytes, files, now); err != nil {
		return "", fmt.Errorf("store: write backup: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("store: write backup: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("store: write backup: %w", err)
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		return "", err
	}
	committed = true
	return final, nil
}

func vacuumInto(ctx context.Context, src, dst string) error {
	c, err := sql.Open("sqlite", dsn(src, false))
	if err != nil {
		return err
	}
	defer c.Close()
	if _, err = c.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
		return err
	}
	return os.Chmod(dst, 0o600)
}

func writeArchive(w io.Writer, manifest []byte, files []struct{ name, path string }, mod time.Time) error {
	zw, err := zstd.NewWriter(w)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: manifestName, Mode: 0o600, Size: int64(len(manifest)), ModTime: mod}); err != nil {
		return err
	}
	if _, err := tw.Write(manifest); err != nil {
		return err
	}
	for _, f := range files {
		if err := addFile(tw, f.name, f.path, mod); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return zw.Close()
}

func addFile(tw *tar.Writer, name, path string, mod time.Time) error {
	fh, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	st, err := fh.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: st.Size(), ModTime: mod}); err != nil {
		return err
	}
	_, err = io.Copy(tw, fh)
	return err
}

// PruneBackups keeps the newest keep archives named tradeloom-backup-*.tar.zst in dir (by name) and
// deletes the rest, oldest first. It returns the deleted paths.
func PruneBackups(dir string, keep int) ([]string, error) {
	if keep < 1 {
		return nil, errors.New("store: keep must be ≥ 1")
	}
	all, err := filepath.Glob(filepath.Join(dir, backupPrefix+"*"+backupSuffix))
	if err != nil {
		return nil, err
	}
	sort.Strings(all)
	var removed []string
	for len(all) > keep {
		if err := os.Remove(all[0]); err != nil {
			return removed, err
		}
		removed = append(removed, all[0])
		all = all[1:]
	}
	return removed, nil
}

// ScheduledBackup writes a backup to o.Dir and, only if that succeeded, prunes to keep archives (FR-DATA-005 AC5).
func (db *DB) ScheduledBackup(ctx context.Context, o BackupOptions, keep int) (string, []string, error) {
	path, err := db.Backup(ctx, o)
	if err != nil {
		return "", nil, err
	}
	removed, err := PruneBackups(o.Dir, keep)
	return path, removed, err
}
