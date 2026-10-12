> **P0-1 Platform Core Implementation Plan — Task 7.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 7: `internal/store` — open app.db, migrator and schema v1

**Files:**
- Create: `internal/store/store.go`, `internal/store/migrate.go`, `internal/store/migrations/0001_init.sql`, `internal/store/store_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: Task 6 `Layout` (callers pass `Layout.AppDB` and `Layout.BackupsDir`); Task 2 archtest (rule 4 now has a real importer).
- Produces (`package store`):
  - `const SettingAppVersionLast = "app_version_last"`; `func SupportedSchema() int` (= 1 after this task)
  - `type SchemaTooNewError struct { Found, Supported int; WrittenBy string }` — `Error()` = `Database schema <Found> (written by <WrittenBy>) is newer than this version supports (<Supported>)`
  - `var ErrCorrupt`; `type CorruptError struct { Path, Detail, LatestBackup string }` (`errors.Is(err, ErrCorrupt)`)
  - `type Options struct { AppVersion string; BackupDir string; Now func() time.Time; Migrations fs.FS; Log *slog.Logger }`
  - `type DB struct { W *sql.DB; R *sql.DB; … }` — `W`: single connection, read-write; `R`: up to 4 connections, `query_only`
  - `func Open(ctx context.Context, path string, o Options) (*DB, error)`; `func (db *DB) Path() string`; `func (db *DB) SchemaVersion() int`; `func (db *DB) Close() error`
  - `func EmbeddedMigrations() fs.FS`; `type Migrator struct{…}`; `func NewMigrator(fsys fs.FS) (*Migrator, error)`; `func (m *Migrator) Latest() int`; `func (m *Migrator) Apply(ctx context.Context, db *sql.DB, from int) error`
  - Migration files: `internal/store/migrations/NNNN_name.sql`, contiguous from 0001, no `PRAGMA/ATTACH/DETACH/BEGIN/COMMIT/ROLLBACK/VACUUM`; later sub-plans add `0002_…`. The candle DB (P0-2) reuses `NewMigrator` + `Apply` with its own embedded FS.
  - Every SQLite connection uses `_pragma` DSN parameters `busy_timeout(5000)`, `journal_mode(WAL)`, `synchronous(NORMAL)`, `foreign_keys(1)`, `temp_store(MEMORY)` (modernc.org/sqlite v1.59.0 `applyQueryParams`).

- [ ] **Step 1: Write the failing test**

`internal/store/store_test.go`:

```go
package store

import (
	"context"
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders ./internal/store/`
Expected: FAIL — build errors `undefined: Options`, `undefined: Open`, `undefined: NewMigrator`.

- [ ] **Step 3: Write the implementation**

```bash
go get modernc.org/sqlite@v1.59.0
```

`internal/store/migrations/0001_init.sql` — the ARCHITECTURE §4.2 P0 tables verbatim (settings, sessions, shortcuts, window_state, instruments, instrument_imports, instrument_fts, watchlists, watchlist_items, order_journal, orders_cache, positions_cache, order_counters, notifications, calendar_overrides; later-phase tables arrive in later migrations):

```sql
-- Tradeloom app.db schema v1 (ARCHITECTURE §4.2, P0 tables). Executed inside one transaction by store.Migrator.
-- No PRAGMA, ATTACH or transaction statements are allowed in migration files.

-- settings
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE sessions (id TEXT PRIMARY KEY, user_id TEXT, logged_in_at INTEGER, expires_at INTEGER,
  risk_ack_at INTEGER, data_plan TEXT, ended_at INTEGER, end_reason TEXT);
CREATE TABLE shortcuts (action_id TEXT PRIMARY KEY, binding TEXT NOT NULL, scope TEXT NOT NULL);
CREATE TABLE window_state (k TEXT PRIMARY KEY, v TEXT NOT NULL);

-- instruments (replaced wholesale per import, inside one transaction)
CREATE TABLE instruments (token INTEGER PRIMARY KEY, exchange_token INTEGER, tradingsymbol TEXT NOT NULL,
  name TEXT, exchange TEXT NOT NULL, segment TEXT NOT NULL, instrument_type TEXT NOT NULL,
  expiry INTEGER, tick_size_paise INTEGER NOT NULL, lot_size INTEGER NOT NULL, display_name TEXT NOT NULL,
  underlying TEXT, is_index INTEGER NOT NULL, is_weekly INTEGER, imported_at INTEGER NOT NULL);
CREATE INDEX ix_instr_symbol ON instruments(exchange, tradingsymbol);
CREATE INDEX ix_instr_underlying ON instruments(underlying, expiry);
CREATE TABLE instrument_imports (id INTEGER PRIMARY KEY, downloaded_at INTEGER, row_count INTEGER, kept_count INTEGER, source TEXT);
CREATE VIRTUAL TABLE instrument_fts USING fts5(tradingsymbol, name, display_name, content='instruments', content_rowid='token', tokenize='unicode61');

-- watchlists
CREATE TABLE watchlists (id TEXT PRIMARY KEY, name TEXT NOT NULL, position INTEGER NOT NULL, created_at, updated_at, deleted_at);
CREATE TABLE watchlist_items (watchlist_id TEXT REFERENCES watchlists(id), token INTEGER, position INTEGER, PRIMARY KEY(watchlist_id, token));

-- orders (local cache + journal)
CREATE TABLE order_journal (id INTEGER PRIMARY KEY, at INTEGER NOT NULL, tag TEXT NOT NULL, kind TEXT NOT NULL,
  source TEXT NOT NULL, params TEXT, kite_order_id TEXT, status TEXT, message TEXT, stage TEXT);
CREATE INDEX ix_journal_tag ON order_journal(tag, at);
CREATE INDEX ix_journal_at ON order_journal(at);
CREATE TABLE orders_cache (kite_order_id TEXT PRIMARY KEY, tag TEXT, body TEXT NOT NULL, status TEXT, updated_at);
CREATE TABLE positions_cache (k TEXT PRIMARY KEY, body TEXT NOT NULL, updated_at);
CREATE TABLE order_counters (day INTEGER PRIMARY KEY, orders INTEGER NOT NULL, mods TEXT, http429 INTEGER NOT NULL DEFAULT 0);

-- notifications inbox (empty until P3)
CREATE TABLE notifications (id TEXT PRIMARY KEY, at INTEGER NOT NULL, type TEXT NOT NULL, category TEXT,
  deployment_id TEXT, di_id TEXT, scanner_id TEXT, tokens TEXT, title TEXT, body TEXT NOT NULL,
  status TEXT, expires_at INTEGER, read_at INTEGER, resolved_at INTEGER, resolved_by TEXT);
CREATE INDEX ix_notif_at ON notifications(at);
CREATE INDEX ix_notif_status ON notifications(status, expires_at);

-- calendar edits
CREATE TABLE calendar_overrides (exchange TEXT, date INTEGER, type TEXT, phases TEXT, source TEXT, PRIMARY KEY(exchange, date));
```

`internal/store/migrate.go`:

```go
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var embedded embed.FS

// EmbeddedMigrations returns the migration files shipped in the binary (root = the migrations directory).
func EmbeddedMigrations() fs.FS {
	sub, err := fs.Sub(embedded, "migrations")
	if err != nil {
		panic(err) // the embed pattern guarantees the directory exists
	}
	return sub
}

var migName = regexp.MustCompile(`^(\d{4})_[a-z0-9_]+\.sql$`)

// forbidden statements: migrations run inside one transaction (ARCHITECTURE §4.4).
var forbidden = regexp.MustCompile(`(?im)^\s*(PRAGMA|ATTACH|DETACH|BEGIN|COMMIT|ROLLBACK|VACUUM)\b`)

type step struct {
	version int
	name    string
	sql     string
}

// Migrator applies NNNN_name.sql files in order inside a single transaction.
type Migrator struct{ steps []step }

// NewMigrator reads every *.sql at the root of fsys. Versions must be contiguous from 0001.
func NewMigrator(fsys fs.FS) (*Migrator, error) {
	ents, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var steps []step
	for _, e := range ents {
		m := migName.FindStringSubmatch(e.Name())
		if e.IsDir() || m == nil {
			continue
		}
		v, _ := strconv.Atoi(m[1])
		b, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		if loc := forbidden.FindIndex(b); loc != nil {
			return nil, fmt.Errorf("store: migration %s contains a forbidden statement %q", e.Name(), strings.TrimSpace(string(b[loc[0]:loc[1]])))
		}
		steps = append(steps, step{version: v, name: e.Name(), sql: string(b)})
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].version < steps[j].version })
	for i, s := range steps {
		if s.version != i+1 {
			return nil, fmt.Errorf("store: migration versions must be contiguous from 1; found %s at position %d", s.name, i+1)
		}
	}
	return &Migrator{steps: steps}, nil
}

// Latest is the highest schema version this binary supports.
func (m *Migrator) Latest() int { return len(m.steps) }

// Apply runs every step above from, plus PRAGMA user_version, in one transaction.
// Any failure rolls back everything: the database is unchanged (FR-DATA-005 AC4).
func (m *Migrator) Apply(ctx context.Context, db *sql.DB, from int) (err error) {
	if from >= m.Latest() {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	for _, s := range m.steps[from:] {
		if _, err = tx.ExecContext(ctx, s.sql); err != nil {
			return fmt.Errorf("store: migration %s: %w", s.name, err)
		}
	}
	// user_version lives in the database header and is covered by the transaction.
	if _, err = tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", m.Latest())); err != nil {
		return err
	}
	return tx.Commit()
}
```

`internal/store/store.go`:

```go
// Package store owns the data directory and app.db (ARCHITECTURE §4): layout,
// the single-instance lock, opening SQLite with WAL, the migration runner,
// backup/restore and the settings repository.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"modernc.org/sqlite"
)

// SettingAppVersionLast is the settings key holding the version of the app that last opened app.db.
const SettingAppVersionLast = "app_version_last"

// SupportedSchema is the app.db schema version of this binary.
func SupportedSchema() int {
	m, err := NewMigrator(EmbeddedMigrations())
	if err != nil {
		panic(err) // embedded files are validated by TestEmbeddedMigrations
	}
	return m.Latest()
}

// SchemaTooNewError is returned when app.db was written by a newer app (FR-DATA-005 AC2).
type SchemaTooNewError struct {
	Found, Supported int
	WrittenBy        string
}

func (e *SchemaTooNewError) Error() string {
	by := e.WrittenBy
	if by == "" {
		by = "an unknown version"
	}
	return fmt.Sprintf("Database schema %d (written by %s) is newer than this version supports (%d)", e.Found, by, e.Supported)
}

// ErrCorrupt is matched (errors.Is) by *CorruptError.
var ErrCorrupt = errors.New("store: database file is damaged")

// CorruptError reports a damaged database and the newest pre-migration backup that could replace it.
type CorruptError struct {
	Path, Detail, LatestBackup string
}

func (e *CorruptError) Error() string {
	msg := fmt.Sprintf("The database %s is damaged (%s).", e.Path, e.Detail)
	if e.LatestBackup != "" {
		msg += " The newest automatic backup is " + e.LatestBackup + "."
	}
	return msg
}

func (e *CorruptError) Is(target error) bool { return target == ErrCorrupt }

// Options configures Open.
type Options struct {
	AppVersion string           // stored as app_version_last after a successful open
	BackupDir  string           // pre-migration copies go here (Layout.BackupsDir)
	Now        func() time.Time // clock.SystemNow in production
	Migrations fs.FS            // nil means EmbeddedMigrations(); tests inject other sets
	Log        *slog.Logger     // nil means slog.Default()
}

// DB is app.db: one single-connection writer pool and a read-only reader pool (WAL).
type DB struct {
	W      *sql.DB
	R      *sql.DB
	path   string
	schema int
	now    func() time.Time
}

func dsn(path string, readOnly bool) string {
	q := url.Values{}
	for _, p := range []string{"busy_timeout(5000)", "journal_mode(WAL)", "synchronous(NORMAL)", "foreign_keys(1)", "temp_store(MEMORY)"} {
		q.Add("_pragma", p)
	}
	if readOnly {
		q.Add("_pragma", "query_only(1)")
	}
	return "file:" + (&url.URL{Path: path}).EscapedPath() + "?" + q.Encode()
}

func isCorruption(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() & 0xff {
		case 11, 26: // SQLITE_CORRUPT, SQLITE_NOTADB
			return true
		}
	}
	return false
}

// Open opens (creating if absent) app.db at path, checks integrity, refuses a newer schema,
// writes a pre-migration backup and migrates in one transaction (ARCHITECTURE §4.4).
func Open(ctx context.Context, path string, o Options) (*DB, error) {
	if o.Now == nil || o.BackupDir == "" {
		return nil, errors.New("store: Options.Now and Options.BackupDir are required")
	}
	if o.Migrations == nil {
		o.Migrations = EmbeddedMigrations()
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	mig, err := NewMigrator(o.Migrations)
	if err != nil {
		return nil, err
	}
	w, err := sql.Open("sqlite", dsn(path, false))
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	fail := func(err error) (*DB, error) { w.Close(); return nil, err }

	if err := quickCheck(ctx, w); err != nil {
		if isCorruption(err) || errors.Is(err, ErrCorrupt) {
			return fail(&CorruptError{Path: path, Detail: err.Error(), LatestBackup: latestFile(o.BackupDir, "app-*.db")})
		}
		return fail(err)
	}
	var cur int
	if err := w.QueryRowContext(ctx, "PRAGMA user_version").Scan(&cur); err != nil {
		return fail(err)
	}
	if cur > mig.Latest() {
		return fail(&SchemaTooNewError{Found: cur, Supported: mig.Latest(), WrittenBy: readAppVersion(ctx, w)})
	}
	if cur > 0 && cur < mig.Latest() {
		dst := filepath.Join(o.BackupDir, fmt.Sprintf("app-%d-%s.db", cur, o.Now().UTC().Format("20060102T150405Z")))
		if err := os.MkdirAll(o.BackupDir, 0o700); err != nil {
			return fail(err)
		}
		if _, err := w.ExecContext(ctx, "VACUUM INTO ?", dst); err != nil {
			return fail(fmt.Errorf("store: pre-migration backup: %w", err))
		}
		o.Log.Info("store.premigration_backup", "component", "store", "path", dst, "schema", cur)
	}
	if err := mig.Apply(ctx, w, cur); err != nil {
		return fail(err)
	}
	if cur != mig.Latest() {
		o.Log.Info("store.migrated", "component", "store", "from", cur, "to", mig.Latest())
	}
	ver, _ := json.Marshal(o.AppVersion)
	if _, err := w.ExecContext(ctx,
		`INSERT INTO settings(key, value, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		SettingAppVersionLast, string(ver), o.Now().UnixMilli()); err != nil {
		return fail(err)
	}
	r, err := sql.Open("sqlite", dsn(path, true))
	if err != nil {
		return fail(err)
	}
	r.SetMaxOpenConns(4)
	return &DB{W: w, R: r, path: path, schema: mig.Latest(), now: o.Now}, nil
}

func quickCheck(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	var msgs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return err
		}
		msgs = append(msgs, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(msgs) != 1 || msgs[0] != "ok" {
		return fmt.Errorf("%w: quick_check: %v", ErrCorrupt, msgs)
	}
	return nil
}

func readAppVersion(ctx context.Context, db *sql.DB) string {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", SettingAppVersionLast).Scan(&raw); err != nil {
		return ""
	}
	var v string
	if json.Unmarshal([]byte(raw), &v) != nil {
		return ""
	}
	return v
}

func latestFile(dir, pattern string) string {
	m, _ := filepath.Glob(filepath.Join(dir, pattern))
	if len(m) == 0 {
		return ""
	}
	sort.Strings(m)
	return m[len(m)-1]
}

// Path returns the database file path.
func (db *DB) Path() string { return db.path }

// SchemaVersion returns the schema version after migration.
func (db *DB) SchemaVersion() int { return db.schema }

// Close checkpoints the WAL into the main file and closes both pools (ARCHITECTURE §3.3 step 6).
func (db *DB) Close() error {
	_, cerr := db.W.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	rerr := db.R.Close()
	werr := db.W.Close()
	return errors.Join(cerr, rerr, werr)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go mod tidy && grep -E '^go ' go.mod && go test -race -count=1 -tags noorders ./internal/store/ ./internal/archtest/`
Expected: `go 1.25.0`; `ok  tradeloom/internal/store`; `ok  tradeloom/internal/archtest` (rule 4: only `internal/store` imports `database/sql`/`modernc.org/sqlite`).

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/store
git commit -m "feat(store): open app.db with WAL, integrity check, refuse-newer, pre-migration backup and v1 schema" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
