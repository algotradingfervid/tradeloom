package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

var fixedNow = func() time.Time { return time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC) }

func openTest(t *testing.T, dir string, mig fstest.MapFS, ver string) (*DB, error) {
	t.Helper()
	o := Options{AppVersion: ver, BackupDir: filepath.Join(dir, "backups"), Now: fixedNow}
	if mig != nil {
		o.Migrations = mig
	}
	return Open(context.Background(), filepath.Join(dir, "app.db"), o)
}

func mustOpen(t *testing.T, dir string, mig fstest.MapFS, ver string) *DB {
	t.Helper()
	db, err := openTest(t, dir, mig, ver)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestEmbeddedMigrations(t *testing.T) {
	m, err := NewMigrator(EmbeddedMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if m.Latest() != 1 || SupportedSchema() != 1 {
		t.Fatalf("latest=%d", m.Latest())
	}
}

// ARCHITECTURE §4.2: v1 = the P0 tables, exactly.
func TestV1Schema_TablesAndColumns(t *testing.T) {
	db := mustOpen(t, t.TempDir(), nil, "v0.1.0")
	defer db.Close()
	want := map[string][]string{
		"settings":           {"key", "value", "updated_at"},
		"sessions":           {"id", "user_id", "logged_in_at", "expires_at", "risk_ack_at", "data_plan", "ended_at", "end_reason"},
		"shortcuts":          {"action_id", "binding", "scope"},
		"window_state":       {"k", "v"},
		"instruments":        {"token", "exchange_token", "tradingsymbol", "name", "exchange", "segment", "instrument_type", "expiry", "tick_size_paise", "lot_size", "display_name", "underlying", "is_index", "is_weekly", "imported_at"},
		"instrument_imports": {"id", "downloaded_at", "row_count", "kept_count", "source"},
		"instrument_fts":     {"tradingsymbol", "name", "display_name"},
		"watchlists":         {"id", "name", "position", "created_at", "updated_at", "deleted_at"},
		"watchlist_items":    {"watchlist_id", "token", "position"},
		"order_journal":      {"id", "at", "tag", "kind", "source", "params", "kite_order_id", "status", "message", "stage"},
		"orders_cache":       {"kite_order_id", "tag", "body", "status", "updated_at"},
		"positions_cache":    {"k", "body", "updated_at"},
		"order_counters":     {"day", "orders", "mods", "http429"},
		"notifications":      {"id", "at", "type", "category", "deployment_id", "di_id", "scanner_id", "tokens", "title", "body", "status", "expires_at", "read_at", "resolved_at", "resolved_by"},
		"calendar_overrides": {"exchange", "date", "type", "phases", "source"},
	}
	for table, cols := range want {
		rows, err := db.R.Query("SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for rows.Next() {
			var n string
			rows.Scan(&n)
			got = append(got, n)
		}
		rows.Close()
		if !reflect.DeepEqual(got, cols) {
			t.Errorf("%s columns\n got %v\nwant %v", table, got, cols)
		}
	}
	var idx []string
	rows, _ := db.R.Query(`SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'ix_%' ORDER BY name`)
	for rows.Next() {
		var n string
		rows.Scan(&n)
		idx = append(idx, n)
	}
	rows.Close()
	wantIdx := []string{"ix_instr_symbol", "ix_instr_underlying", "ix_journal_at", "ix_journal_tag", "ix_notif_at", "ix_notif_status"}
	if !reflect.DeepEqual(idx, wantIdx) {
		t.Errorf("indexes %v want %v", idx, wantIdx)
	}
	for _, later := range []string{"documents", "backtest_runs", "deployments", "telegram_state"} {
		var n int
		db.R.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = ?`, later).Scan(&n)
		if n != 0 {
			t.Errorf("later-phase table %s must not be in v1", later)
		}
	}
}

func TestOpen_PragmasAndAppVersion(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "path with spaces & #hash")
	os.MkdirAll(dir, 0o700)
	db := mustOpen(t, dir, nil, "v0.1.0")
	defer db.Close()
	if _, err := os.Stat(filepath.Join(dir, "app.db")); err != nil {
		t.Fatalf("app.db not created at the literal path: %v", err)
	}
	var mode string
	var fk int
	db.W.QueryRow("PRAGMA journal_mode").Scan(&mode)
	db.W.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if mode != "wal" || fk != 1 {
		t.Fatalf("journal_mode=%s foreign_keys=%d", mode, fk)
	}
	if v := readAppVersion(context.Background(), db.W); v != "v0.1.0" {
		t.Fatalf("app_version_last=%q", v)
	}
	if _, err := db.R.Exec("INSERT INTO window_state(k, v) VALUES('a','b')"); err == nil {
		t.Fatal("reader pool must be query_only")
	}
}

var migV2 = fstest.MapFS{
	"0001_a.sql": {Data: []byte("CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at INTEGER NOT NULL);")},
	"0002_b.sql": {Data: []byte("CREATE TABLE b (x INTEGER);")},
}

func withStep3(sql string) fstest.MapFS {
	m := fstest.MapFS{}
	for k, v := range migV2 {
		m[k] = v
	}
	m["0003_c.sql"] = &fstest.MapFile{Data: []byte(sql)}
	return m
}

// FR-DATA-005 AC2.
func TestOpen_RefusesNewerSchema(t *testing.T) {
	dir := t.TempDir()
	db := mustOpen(t, dir, withStep3("CREATE TABLE c (y INTEGER);"), "v1.4.0")
	db.Close()
	_, err := openTest(t, dir, migV2, "v1.3.0")
	var tooNew *SchemaTooNewError
	if !errors.As(err, &tooNew) {
		t.Fatalf("want SchemaTooNewError, got %v", err)
	}
	want := "Database schema 3 (written by v1.4.0) is newer than this version supports (2)"
	if err.Error() != want {
		t.Fatalf("message\n got %q\nwant %q", err.Error(), want)
	}
	// The refused database is untouched: still written by v1.4.0.
	chk, cerr := sql.Open("sqlite", "file:"+filepath.Join(dir, "app.db")+"?mode=ro")
	if cerr != nil {
		t.Fatal(cerr)
	}
	if v := readAppVersion(context.Background(), chk); v != "v1.4.0" {
		t.Fatalf("refused database modified: app_version_last=%q", v)
	}
	chk.Close()
	db = mustOpen(t, dir, withStep3("CREATE TABLE c (y INTEGER);"), "v1.4.0")
	db.Close()
}

// FR-DATA-005 AC4.
func TestOpen_FailedMigrationRollsBackAndKeepsBackup(t *testing.T) {
	dir := t.TempDir()
	db := mustOpen(t, dir, migV2, "v1.3.0")
	db.W.Exec("INSERT INTO b(x) VALUES (42)")
	db.Close()

	_, err := openTest(t, dir, withStep3("CREATE TABLE c (y INTEGER);\nINSERT INTO no_such_table VALUES (1);"), "v1.4.0")
	if err == nil || !strings.Contains(err.Error(), "0003_c.sql") {
		t.Fatalf("want migration error naming 0003_c.sql, got %v", err)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "backups", "app-2-*.db"))
	if len(backups) != 1 {
		t.Fatalf("want one pre-migration backup, got %v", backups)
	}
	db = mustOpen(t, dir, migV2, "v1.3.0")
	defer db.Close()
	if db.SchemaVersion() != 2 {
		t.Fatalf("schema %d", db.SchemaVersion())
	}
	var n, x int
	db.R.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='c'").Scan(&n)
	db.R.QueryRow("SELECT x FROM b").Scan(&x)
	if n != 0 || x != 42 {
		t.Fatalf("rollback incomplete: table c=%d, b.x=%d", n, x)
	}
}

func TestNewMigrator_Validation(t *testing.T) {
	cases := map[string]fstest.MapFS{
		"gap":    {"0001_a.sql": {Data: []byte("SELECT 1;")}, "0003_c.sql": {Data: []byte("SELECT 1;")}},
		"pragma": {"0001_a.sql": {Data: []byte("PRAGMA foreign_keys=OFF;")}},
		"attach": {"0001_a.sql": {Data: []byte("  attach 'x' as y;")}},
	}
	for name, fsys := range cases {
		if _, err := NewMigrator(fsys); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// Review Focus 1: a damaged app.db is reported as ErrCorrupt naming the newest backup; nothing is overwritten.
func TestOpen_CorruptDatabase(t *testing.T) {
	cases := map[string]func(path string) error{
		"garbage": func(p string) error { return os.WriteFile(p, []byte(strings.Repeat("not a database ", 500)), 0o600) },
		"truncated": func(p string) error {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(p, b[:len(b)/2], 0o600)
		},
	}
	for name, damage := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			db := mustOpen(t, dir, migV2, "v1")
			for i := 0; i < 2000; i++ {
				db.W.Exec("INSERT INTO b(x) VALUES (?)", i)
			}
			db.Close()
			os.MkdirAll(filepath.Join(dir, "backups"), 0o700)
			os.WriteFile(filepath.Join(dir, "backups", "app-1-20261001T000000Z.db"), []byte("x"), 0o600)
			os.WriteFile(filepath.Join(dir, "backups", "app-1-20261011T000000Z.db"), []byte("x"), 0o600)
			p := filepath.Join(dir, "app.db")
			if err := damage(p); err != nil {
				t.Fatal(err)
			}
			damaged, _ := os.ReadFile(p)
			_, err := openTest(t, dir, migV2, "v1")
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("want ErrCorrupt, got %v", err)
			}
			if !strings.Contains(err.Error(), "app-1-20261011T000000Z.db") {
				t.Fatalf("error should name the newest backup: %v", err)
			}
			after, _ := os.ReadFile(p)
			if string(after) != string(damaged) {
				t.Fatal("damaged file was modified")
			}
		})
	}
}

func TestLatestFile(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"app-1-b.db", "app-1-a.db", "app-1-c.db"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o600)
	}
	got := latestFile(dir, "app-*.db")
	all, _ := filepath.Glob(filepath.Join(dir, "*"))
	sort.Strings(all)
	if got != all[2] {
		t.Fatalf("got %s", got)
	}
}

// Controller ruling 6: app.db, -wal, -shm and pre-migration backups are owner-only.
func TestOpen_FileModes0600(t *testing.T) {
	dir := t.TempDir()
	db := mustOpen(t, dir, migV2, "v1")
	db.Close()
	// Reopen with a newer schema so a pre-migration backup is written; keep it open so -wal/-shm exist.
	db = mustOpen(t, dir, withStep3("CREATE TABLE c (y INTEGER);"), "v2")
	defer db.Close()
	p := filepath.Join(dir, "app.db")
	paths := []string{p, p + "-wal", p + "-shm"}
	backups, _ := filepath.Glob(filepath.Join(dir, "backups", "app-2-*.db"))
	if len(backups) != 1 {
		t.Fatalf("want one backup, got %v", backups)
	}
	paths = append(paths, backups...)
	for _, f := range paths {
		fi, err := os.Stat(f)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v, want 0600", filepath.Base(f), fi.Mode().Perm())
		}
	}
}

// Review Focus 1: a leftover -wal must not be replayed into a damaged app.db.
func TestOpen_CorruptWithLeftoverWAL(t *testing.T) {
	for _, mode := range []string{"truncate", "garble"} {
		t.Run(mode, func(t *testing.T) {
			src := t.TempDir()
			db := mustOpen(t, src, migV2, "v1")
			defer db.Close()
			for i := 0; i < 2000; i++ {
				db.W.Exec("INSERT INTO b(x) VALUES (?)", i)
			}
			dir := t.TempDir()
			// Copy while open in WAL mode so -wal/-shm hold unreplayed frames.
			for _, suf := range []string{"", "-wal", "-shm"} {
				b, err := os.ReadFile(filepath.Join(src, "app.db"+suf))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "app.db"+suf), b, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			p := filepath.Join(dir, "app.db")
			b, _ := os.ReadFile(p)
			if mode == "truncate" {
				b = b[:len(b)/2]
			} else {
				copy(b, "garbage garbage!")
			}
			os.WriteFile(p, b, 0o600)
			walBefore, err := os.ReadFile(p + "-wal")
			if err != nil || len(walBefore) == 0 {
				t.Fatalf("test needs a non-empty -wal: %v", err)
			}
			_, err = openTest(t, dir, migV2, "v1")
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("want ErrCorrupt, got %v", err)
			}
			after, _ := os.ReadFile(p)
			walAfter, werr := os.ReadFile(p + "-wal")
			if string(after) != string(b) {
				t.Fatal("damaged app.db was modified")
			}
			if werr != nil || string(walAfter) != string(walBefore) {
				t.Fatal("-wal was removed or modified")
			}
		})
	}
}
