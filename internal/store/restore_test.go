package store

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

// FR-DATA-005 AC1 (engine side): restore on "machine B" brings back every row, keeps no secret.
func TestRestore_RoundTripToFreshDataDir(t *testing.T) {
	la, dbA := seedDataDir(t)
	archive, err := dbA.Backup(context.Background(), backupOpts(la, t.TempDir(), false))
	if err != nil {
		t.Fatal(err)
	}
	dbA.Close()

	lb := NewLayout(filepath.Join(t.TempDir(), "tradeloom"))
	lb.Ensure()
	dbB, _ := Open(context.Background(), lb.AppDB, Options{AppVersion: "v0.1.0", BackupDir: lb.BackupsDir, Now: fixedNow})
	dbB.Close()

	later := func() time.Time { return fixedNow().Add(time.Hour) }
	res, err := Restore(context.Background(), RestoreRequest{Archive: archive, DataDir: lb.Root,
		Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: later})
	if err != nil {
		t.Fatal(err)
	}
	if res.Manifest.Schema != 1 || res.Manifest.Counts["watchlists"] != 1 {
		t.Fatalf("manifest %+v", res.Manifest)
	}
	if m, err := InspectBackup(archive); err != nil || m.Counts["order_journal"] != 1 {
		t.Fatalf("InspectBackup: %+v %v", m, err)
	}
	if res.ReplacedDir != lb.Root+".replaced-20261012T110000Z" {
		t.Fatalf("replaced dir %s", res.ReplacedDir)
	}
	if _, err := os.Stat(filepath.Join(res.ReplacedDir, "app.db")); err != nil {
		t.Fatal("previous data dir not kept")
	}
	if _, err := os.Stat(lb.SecretsFile); !os.IsNotExist(err) {
		t.Fatal("restored dir must not contain secrets.enc")
	}
	db, err := Open(context.Background(), lb.AppDB, Options{AppVersion: "v0.1.0", BackupDir: lb.BackupsDir, Now: later})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var wl, items, journal int
	var theme string
	db.R.QueryRow("SELECT count(*) FROM watchlists").Scan(&wl)
	db.R.QueryRow("SELECT count(*) FROM watchlist_items").Scan(&items)
	db.R.QueryRow("SELECT count(*) FROM order_journal").Scan(&journal)
	db.R.QueryRow("SELECT value FROM settings WHERE key='ui.theme'").Scan(&theme)
	if wl != 1 || items != 1 || journal != 1 || theme != `"dark"` {
		t.Fatalf("restored rows: wl=%d items=%d journal=%d theme=%s", wl, items, journal, theme)
	}
}

func TestRestore_Refusals(t *testing.T) {
	l, db := seedDataDir(t)
	archive, _ := db.Backup(context.Background(), backupOpts(l, t.TempDir(), false))
	db.Close()
	base := RestoreRequest{Archive: archive, DataDir: l.Root, Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: fixedNow}
	r1 := base
	r1.Confirmation = "restore"
	if _, err := Restore(context.Background(), r1); !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("want ErrNotConfirmed, got %v", err)
	}
	r2 := base
	r2.DeploymentsFlat = false
	if _, err := Restore(context.Background(), r2); !errors.Is(err, ErrDeploymentsActive) {
		t.Fatalf("want ErrDeploymentsActive, got %v", err)
	}
}

func writeTarZst(t *testing.T, path string, entries []tar.Header, bodies [][]byte) {
	t.Helper()
	fh, _ := os.Create(path)
	defer fh.Close()
	zw, _ := zstd.NewWriter(fh)
	tw := tar.NewWriter(zw)
	for i, h := range entries {
		h := h
		h.Size = int64(len(bodies[i]))
		if h.Typeflag == 0 {
			h.Typeflag = tar.TypeReg
		}
		h.Mode = 0o600
		tw.WriteHeader(&h)
		tw.Write(bodies[i])
	}
	tw.Close()
	zw.Close()
}

// Review Focus 4: newer schema (by manifest or by the app.db inside), path traversal and missing
// app.db are refused and the current data directory is untouched.
func TestRestore_RejectsHostileOrNewerArchives(t *testing.T) {
	l, db := seedDataDir(t)
	good, _ := db.Backup(context.Background(), backupOpts(l, t.TempDir(), false))
	db.Close()
	ents := archiveEntries(t, good)
	newerDB := filepath.Join(t.TempDir(), "newer.db")
	nd, _ := Open(context.Background(), newerDB, Options{AppVersion: "v9.0.0", BackupDir: t.TempDir(), Now: fixedNow,
		Migrations: withStep3("CREATE TABLE c (y INTEGER);")})
	nd.Close()
	newerBytes, _ := os.ReadFile(newerDB)
	manifestV3 := bytes.Replace(ents["manifest.json"], []byte(`"schema": 1`), []byte(`"schema": 3`), 1)

	cases := []struct {
		name    string
		entries []tar.Header
		bodies  [][]byte
		want    func(error) bool
	}{
		{"manifest schema newer", []tar.Header{{Name: "manifest.json"}, {Name: "app.db"}}, [][]byte{manifestV3, ents["app.db"]},
			func(err error) bool { var e *SchemaTooNewError; return errors.As(err, &e) }},
		{"app.db newer than manifest says", []tar.Header{{Name: "manifest.json"}, {Name: "app.db"}}, [][]byte{ents["manifest.json"], newerBytes},
			func(err error) bool { var e *SchemaTooNewError; return errors.As(err, &e) }},
		{"path traversal", []tar.Header{{Name: "manifest.json"}, {Name: "../../evil"}}, [][]byte{ents["manifest.json"], []byte("x")},
			func(err error) bool { return errors.Is(err, ErrBadArchive) }},
		{"symlink entry", []tar.Header{{Name: "manifest.json"}, {Name: "app.db", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}}, [][]byte{ents["manifest.json"], nil},
			func(err error) bool { return errors.Is(err, ErrBadArchive) }},
		{"missing app.db", []tar.Header{{Name: "manifest.json"}}, [][]byte{ents["manifest.json"]},
			func(err error) bool { return errors.Is(err, ErrBadArchive) }},
	}
	before, _ := os.ReadFile(l.AppDB)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "x.tar.zst")
			writeTarZst(t, p, tc.entries, tc.bodies)
			_, err := Restore(context.Background(), RestoreRequest{Archive: p, DataDir: l.Root,
				Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: fixedNow})
			if !tc.want(err) {
				t.Fatalf("unexpected error %v", err)
			}
			after, _ := os.ReadFile(l.AppDB)
			if !bytes.Equal(before, after) {
				t.Fatal("current app.db changed")
			}
			if left, _ := filepath.Glob(l.Root + ".restoring-*"); len(left) != 0 {
				t.Fatalf("staging left behind: %v", left)
			}
			if strings.Contains(tc.name, "traversal") {
				if _, err := os.Stat(filepath.Join(filepath.Dir(l.Root), "evil")); err == nil {
					t.Fatal("traversal entry was written")
				}
			}
		})
	}
}

// Review Focus 4 (more hostile shapes): absolute path, hardlink, device, directory, duplicate,
// unknown entry, oversized/bomb entries, garbage app.db, manifest/app.db schema mismatch, non-zstd.
func TestRestore_RejectsMoreHostileShapes(t *testing.T) {
	l, db := seedDataDir(t)
	good, _ := db.Backup(context.Background(), backupOpts(l, t.TempDir(), false))
	db.Close()
	ents := archiveEntries(t, good)
	man, app := ents["manifest.json"], ents["app.db"]
	manV2 := bytes.Replace(man, []byte(`"schema": 1`), []byte(`"schema": 0`), 1)
	trunc := app[:len(app)/2]

	mf := tar.Header{Name: "manifest.json"}
	cases := []struct {
		name    string
		entries []tar.Header
		bodies  [][]byte
	}{
		{"absolute path", []tar.Header{mf, {Name: "/tmp/tradeloom-evil"}}, [][]byte{man, []byte("x")}},
		{"hardlink entry", []tar.Header{mf, {Name: "app.db", Typeflag: tar.TypeLink, Linkname: "/etc/passwd"}}, [][]byte{man, nil}},
		{"char device entry", []tar.Header{mf, {Name: "app.db", Typeflag: tar.TypeChar}}, [][]byte{man, nil}},
		{"directory entry", []tar.Header{mf, {Name: "candles/", Typeflag: tar.TypeDir}, {Name: "app.db"}}, [][]byte{man, nil, app}},
		{"duplicate app.db", []tar.Header{mf, {Name: "app.db"}, {Name: "app.db"}}, [][]byte{man, app, app}},
		{"unknown entry", []tar.Header{mf, {Name: "app.db"}, {Name: "secrets.enc"}}, [][]byte{man, app, []byte("x")}},
		{"dot-dot inside candles", []tar.Header{mf, {Name: "app.db"}, {Name: "candles/../../evil"}}, [][]byte{man, app, []byte("x")}},
		{"garbage app.db", []tar.Header{mf, {Name: "app.db"}}, [][]byte{man, []byte("this is not sqlite at all, not even close, no way")}},
		{"truncated app.db", []tar.Header{mf, {Name: "app.db"}}, [][]byte{man, trunc}},
		{"empty app.db", []tar.Header{mf, {Name: "app.db"}}, [][]byte{man, nil}},
		{"manifest schema mismatch", []tar.Header{{Name: "manifest.json"}, {Name: "app.db"}}, [][]byte{manV2, app}},
		{"manifest not json", []tar.Header{mf, {Name: "app.db"}}, [][]byte{[]byte("{"), app}},
		{"no manifest", []tar.Header{{Name: "app.db"}}, [][]byte{app}},
		{"garbage candles.db", []tar.Header{mf, {Name: "app.db"}, {Name: "candles/candles.db"}}, [][]byte{man, app, []byte("junk junk junk junk")}},
	}
	before, _ := os.ReadFile(l.AppDB)
	check := func(t *testing.T, p string, wantBad bool) {
		t.Helper()
		_, err := Restore(context.Background(), RestoreRequest{Archive: p, DataDir: l.Root,
			Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: fixedNow})
		if wantBad && !errors.Is(err, ErrBadArchive) {
			t.Fatalf("want ErrBadArchive, got %v", err)
		}
		after, _ := os.ReadFile(l.AppDB)
		if !bytes.Equal(before, after) {
			t.Fatal("current app.db changed")
		}
		if left, _ := filepath.Glob(l.Root + ".restoring-*"); len(left) != 0 {
			t.Fatalf("staging left behind: %v", left)
		}
		if left, _ := filepath.Glob(l.Root + ".replaced-*"); len(left) != 0 {
			t.Fatalf("data dir was replaced: %v", left)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(l.Root), "evil")); err == nil {
			t.Fatal("traversal entry was written")
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "x.tar.zst")
			writeTarZst(t, p, tc.entries, tc.bodies)
			check(t, p, true)
		})
	}
	t.Run("not zstd", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "x.tar.zst")
		os.WriteFile(p, []byte("plain text, not an archive"), 0o600)
		check(t, p, true)
	})
	t.Run("oversized entry", func(t *testing.T) {
		old := maxEntryBytes
		maxEntryBytes = int64(len(app)) - 1
		defer func() { maxEntryBytes = old }()
		p := filepath.Join(t.TempDir(), "x.tar.zst")
		writeTarZst(t, p, []tar.Header{mf, {Name: "app.db"}}, [][]byte{man, app})
		check(t, p, true)
	})
	t.Run("total size bomb", func(t *testing.T) {
		old := maxTotalBytes
		maxTotalBytes = int64(len(man)+len(app)) - 1
		defer func() { maxTotalBytes = old }()
		p := filepath.Join(t.TempDir(), "x.tar.zst")
		writeTarZst(t, p, []tar.Header{mf, {Name: "app.db"}}, [][]byte{man, app})
		check(t, p, true)
	})
	t.Run("good archive inspects after limits restored", func(t *testing.T) {
		if _, err := InspectBackup(good); err != nil {
			t.Fatal(err)
		}
	})
}

// Restored files and directories are owner-only (controller ruling 6) and carry no WAL sidecars.
func TestRestore_ModesAndNoSidecars(t *testing.T) {
	l, db := seedDataDir(t)
	archive, _ := db.Backup(context.Background(), backupOpts(l, t.TempDir(), false))
	db.Close()
	if _, err := Restore(context.Background(), RestoreRequest{Archive: archive, DataDir: l.Root,
		Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true, Now: fixedNow}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{l.Root, l.CandlesDir, l.BackupsDir} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o700 {
			t.Fatalf("%s: %v %v", p, st, err)
		}
	}
	if st, err := os.Stat(l.AppDB); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("app.db mode: %v %v", st, err)
	}
	for _, s := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(l.AppDB + s); !os.IsNotExist(err) {
			t.Fatalf("sidecar %s left in restored dir", s)
		}
	}
}

func TestRecoverInterruptedRestore(t *testing.T) {
	mk := func(p, content string) {
		os.MkdirAll(p, 0o700)
		os.WriteFile(filepath.Join(p, "id"), []byte(content), 0o600)
	}
	read := func(p string) string { b, _ := os.ReadFile(filepath.Join(p, "id")); return string(b) }

	t.Run("root missing, two replaced", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "tradeloom")
		mk(root+".replaced-20261012T100000Z", "old")
		mk(root+".replaced-20261012T120000Z", "newest")
		mk(root+".restoring-20261012T120000Z", "partial")
		got, err := RecoverInterruptedRestore(root)
		if err != nil || got != root+".replaced-20261012T120000Z" {
			t.Fatalf("got %q %v", got, err)
		}
		if read(root) != "newest" {
			t.Fatal("newest not restored")
		}
		if _, err := os.Stat(root + ".replaced-20261012T100000Z"); err != nil {
			t.Fatal("older replaced dir must be kept")
		}
		if left, _ := filepath.Glob(root + ".restoring-*"); len(left) != 0 {
			t.Fatalf("staging left: %v", left)
		}
	})
	t.Run("root present untouched", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "tradeloom")
		mk(root, "current")
		mk(root+".replaced-20261012T100000Z", "old")
		got, err := RecoverInterruptedRestore(root)
		if err != nil || got != "" || read(root) != "current" {
			t.Fatalf("got %q %v root=%s", got, err, read(root))
		}
		if _, err := os.Stat(root + ".replaced-20261012T100000Z"); err != nil {
			t.Fatal("replaced dir must stay")
		}
	})
	t.Run("stale restoring swept, root present", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "tradeloom")
		mk(root, "current")
		mk(root+".restoring-20261012T100000Z", "partial")
		if got, err := RecoverInterruptedRestore(root); err != nil || got != "" {
			t.Fatalf("got %q %v", got, err)
		}
		if _, err := os.Stat(root + ".restoring-20261012T100000Z"); !os.IsNotExist(err) {
			t.Fatal("staging not swept")
		}
		if read(root) != "current" {
			t.Fatal("root changed")
		}
	})
	t.Run("nothing at all", func(t *testing.T) {
		if got, err := RecoverInterruptedRestore(filepath.Join(t.TempDir(), "tradeloom")); err != nil || got != "" {
			t.Fatalf("got %q %v", got, err)
		}
	})
}

func TestRestore_NilNowDefaultsToSystemClock(t *testing.T) {
	l, db := seedDataDir(t)
	archive, _ := db.Backup(context.Background(), backupOpts(l, t.TempDir(), false))
	db.Close()
	res, err := Restore(context.Background(), RestoreRequest{Archive: archive, DataDir: l.Root,
		Confirmation: RestoreConfirmPhrase, DeploymentsFlat: true})
	if err != nil || !strings.Contains(res.ReplacedDir, ".replaced-") {
		t.Fatalf("%+v %v", res, err)
	}
}
