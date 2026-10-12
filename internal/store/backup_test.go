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
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("archive mode %v err %v (want 0600)", st.Mode().Perm(), err)
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
