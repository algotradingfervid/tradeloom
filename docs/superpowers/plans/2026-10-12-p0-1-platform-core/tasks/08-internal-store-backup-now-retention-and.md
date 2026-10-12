> **P0-1 Platform Core Implementation Plan — Task 8.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 8: `internal/store` — Backup now, retention and scheduled backup

**Files:**
- Create: `internal/store/backup.go`, `internal/store/backup_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: Task 7 `DB`, `Open`, `Options`, `dsn`, `SupportedSchema`; Task 6 `Layout`, `NewLayout`.
- Produces (`package store`):
  - `const ManifestFormat = 1`
  - `type Manifest struct { Format int; CreatedAt string; AppVersion string; Schema int; CalendarVersion string; LibraryVersion string; IncludesCandles bool; Counts map[string]int }` (JSON `format`, `created_at`, `app_version`, `schema`, `calendar_version`, `library_version`, `includes_candles`, `counts`; count keys `strategies`, `scanners`, `backtest_runs`, `deployments`, `watchlists`, `order_journal`)
  - `type BackupOptions struct { Dir string; IncludeCandles bool; CandlesDB string; AppVersion string; CalendarVersion string; LibraryVersion string; Now func() time.Time }`
  - `func (db *DB) Counts(ctx context.Context) (map[string]int, error)`
  - `func (db *DB) Backup(ctx context.Context, o BackupOptions) (string, error)` → `<Dir>/tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst` with `manifest.json`, `app.db`, optional `candles/candles.db`
  - `func PruneBackups(dir string, keep int) ([]string, error)`; `func (db *DB) ScheduledBackup(ctx context.Context, o BackupOptions, keep int) (string, []string, error)` — P0-3 calls it daily after 16:00 or on quit (FR-DATA-005), using the `backup.schedule.*` settings keys from Task 10.

- [ ] **Step 1: Write the failing test**

`internal/store/backup_test.go`:

```go
package store

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/klauspost/compress/zstd"
)

const (
	secretAPI    = "apiS3cretForGrep001"
	secretAccess = "accessT0kenForGrep002"
	secretTG     = "987654321:TelegramBotTokenForGrep003xyz"
)

// seedDataDir creates a data dir with app.db holding a watchlist, a setting and a journal row,
// plus secret-bearing files that must never reach an archive.
func seedDataDir(t *testing.T) (Layout, *DB) {
	t.Helper()
	l := NewLayout(filepath.Join(t.TempDir(), "tradeloom"))
	if err := l.Ensure(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), l.AppDB, Options{AppVersion: "v0.1.0", BackupDir: l.BackupsDir, Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO watchlists(id,name,position,created_at,updated_at) VALUES('w1','Banks',0,1,1)`)
	mustExec(t, db, `INSERT INTO watchlist_items(watchlist_id,token,position) VALUES('w1',738561,0)`)
	mustExec(t, db, `INSERT INTO settings(key,value,updated_at) VALUES('ui.theme','"dark"',1)`)
	mustExec(t, db, `INSERT INTO order_journal(at,tag,kind,source) VALUES(1,'t1','intent','manual')`)
	os.WriteFile(l.SecretsFile, []byte("TLSEC1"+secretAPI+secretAccess+secretTG), 0o600)
	os.WriteFile(filepath.Join(l.LogsDir, "tradeloom.log"), []byte(secretAccess), 0o600)
	os.WriteFile(l.LockFile, []byte("running pid=1"), 0o600)
	return l, db
}

func mustExec(t *testing.T, db *DB, q string, args ...any) {
	t.Helper()
	if _, err := db.W.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}

func archiveEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	fh, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	zr, err := zstd.NewReader(fh)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	tr := tar.NewReader(zr)
	out := map[string][]byte{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		out[h.Name] = b
	}
}

func backupOpts(l Layout, dir string, candles bool) BackupOptions {
	return BackupOptions{Dir: dir, IncludeCandles: candles, CandlesDB: l.CandlesDB, AppVersion: "v0.1.0",
		CalendarVersion: "nse-bse-2026.1", LibraryVersion: "0", Now: fixedNow}
}

// FR-DATA-005 AC3 + archive layout.
func TestBackup_ContentsAndNoSecrets(t *testing.T) {
	l, db := seedDataDir(t)
	defer db.Close()
	path, err := db.Backup(context.Background(), backupOpts(l, l.BackupsDir, false))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "tradeloom-backup-20261012-100000.tar.zst" {
		t.Fatalf("name %s", path)
	}
	ents := archiveEntries(t, path)
	if len(ents) != 2 || ents["manifest.json"] == nil || ents["app.db"] == nil {
		t.Fatalf("entries %v", keys(ents))
	}
	raw, _ := os.ReadFile(path)
	for _, blob := range append([][]byte{raw}, ents["manifest.json"], ents["app.db"]) {
		for _, s := range []string{secretAPI, secretAccess, secretTG} {
			if bytes.Contains(blob, []byte(s)) {
				t.Fatalf("archive contains secret %q", s)
			}
		}
	}
	var m Manifest
	if err := json.Unmarshal(ents["manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	if m.Schema != 1 || m.AppVersion != "v0.1.0" || m.Counts["watchlists"] != 1 || m.Counts["order_journal"] != 1 || m.IncludesCandles {
		t.Fatalf("manifest %+v", m)
	}
}

func keys(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestBackup_OptionalCandles(t *testing.T) {
	l, db := seedDataDir(t)
	defer db.Close()
	c, _ := Open(context.Background(), l.CandlesDB, Options{BackupDir: l.BackupsDir, Now: fixedNow,
		Migrations: migV2})
	c.Close()
	path, err := db.Backup(context.Background(), backupOpts(l, l.BackupsDir, true))
	if err != nil {
		t.Fatal(err)
	}
	if ents := archiveEntries(t, path); ents["candles/candles.db"] == nil {
		t.Fatalf("candles missing: %v", keys(ents))
	}
}

type failingFile struct {
	*os.File
	left int
}

func (f *failingFile) Write(p []byte) (int, error) {
	if len(p) > f.left {
		n, _ := f.File.Write(p[:f.left])
		f.left = 0
		return n, &os.PathError{Op: "write", Path: f.Name(), Err: syscall.ENOSPC}
	}
	f.left -= len(p)
	return f.File.Write(p)
}

// Review Focus 3: disk full during backup leaves no archive with the final name and no temp file.
func TestBackup_DiskFullLeavesNothing(t *testing.T) {
	l, db := seedDataDir(t)
	defer db.Close()
	dest := t.TempDir()
	old := createTemp
	createTemp = func(dir, pattern string) (tempFile, error) {
		f, err := os.CreateTemp(dir, pattern)
		return &failingFile{File: f, left: 512}, err
	}
	defer func() { createTemp = old }()
	_, err := db.Backup(context.Background(), backupOpts(l, dest, false))
	if !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("want ENOSPC, got %v", err)
	}
	left, _ := os.ReadDir(dest)
	if len(left) != 0 {
		t.Fatalf("leftovers after failed backup: %v", left)
	}
}

// FR-DATA-005 AC5: N = 7 with 8 existing backups → after the new one is written exactly 7 remain,
// the deleted ones are the oldest, and nothing is pruned when the backup fails.
func TestScheduledBackup_Retention(t *testing.T) {
	l, db := seedDataDir(t)
	defer db.Close()
	dest := t.TempDir()
	var existing []string
	for i := 1; i <= 8; i++ {
		p := filepath.Join(dest, "tradeloom-backup-202610"+twoDigits(i)+"-160000.tar.zst")
		os.WriteFile(p, []byte("old"), 0o600)
		existing = append(existing, p)
	}
	os.WriteFile(filepath.Join(dest, "unrelated.txt"), []byte("keep me"), 0o600)
	path, removed, err := db.ScheduledBackup(context.Background(), backupOpts(l, dest, false), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 || removed[0] != existing[0] || removed[1] != existing[1] {
		t.Fatalf("removed %v", removed)
	}
	got, _ := filepath.Glob(filepath.Join(dest, "tradeloom-backup-*.tar.zst"))
	if len(got) != 7 || got[len(got)-1] != path {
		t.Fatalf("remaining %v", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "unrelated.txt")); err != nil {
		t.Fatal("pruning touched an unrelated file")
	}

	old := createTemp
	createTemp = func(dir, pattern string) (tempFile, error) { return nil, syscall.ENOSPC }
	defer func() { createTemp = old }()
	if _, _, err := db.ScheduledBackup(context.Background(), backupOpts(l, dest, false), 1); err == nil {
		t.Fatal("want error")
	}
	if got2, _ := filepath.Glob(filepath.Join(dest, "tradeloom-backup-*.tar.zst")); len(got2) != 7 {
		t.Fatalf("failed backup must not prune: %d left", len(got2))
	}
}

func twoDigits(i int) string { return string([]byte{byte('0' + i/10), byte('0' + i%10)}) }
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders ./internal/store/`
Expected: FAIL — `no required module provides package github.com/klauspost/compress/zstd` (the test reads archives with zstd); after Step 3's `go get` and before `backup.go` exists the errors become `undefined: BackupOptions`, `undefined: Manifest`, `undefined: createTemp`.

- [ ] **Step 3: Write the implementation**

```bash
go get github.com/klauspost/compress@v1.20.3
```

`internal/store/backup.go`:

```go
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
	_, err = c.ExecContext(ctx, "VACUUM INTO ?", dst)
	return err
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go mod tidy && go test -race -count=1 -tags noorders ./internal/store/`
Expected: `ok  tradeloom/internal/store`.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/store/backup.go internal/store/backup_test.go
git commit -m "feat(store): Backup now (tar.zst, manifest, optional candles), retention and scheduled backup" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
