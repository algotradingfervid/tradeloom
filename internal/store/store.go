// Package store owns the data directory and app.db (ARCHITECTURE §4): layout,
// the single-instance lock, opening SQLite with WAL, the migration runner,
// backup/restore and the settings repository.
package store

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// Raw header check before any connection: opening in WAL mode would replay a leftover -wal
	// into a damaged app.db (Review Focus 1).
	if err := CheckHeader(path); err != nil {
		var ce *CorruptError
		if errors.As(err, &ce) {
			ce.LatestBackup = latestFile(o.BackupDir, "app-*.db")
		}
		return nil, err
	}
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600); err != nil {
		return nil, err
	} else if err := f.Close(); err != nil {
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
		if err := os.Chmod(dst, 0o600); err != nil {
			return fail(fmt.Errorf("store: chmod backup: %w", err))
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
	// Files hold session and order data: owner-only (Global Constraint, controller ruling 6).
	if err := RestrictModes(path); err != nil {
		r.Close()
		return fail(err)
	}
	return &DB{W: w, R: r, path: path, schema: mig.Latest(), now: o.Now}, nil
}

// CheckHeader validates an existing SQLite file (app.db, candles.db) without opening it with
// SQLite, so a leftover -wal is never replayed into a damaged file. A missing or empty file is a
// new database and passes. A damaged file yields a *CorruptError (errors.Is ErrCorrupt) without
// LatestBackup.
func CheckHeader(path string) error {
	if detail := rawHeaderCheck(path); detail != "" {
		return &CorruptError{Path: path, Detail: detail}
	}
	return nil
}

func rawHeaderCheck(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "" // absent (or unreadable: let SQLite report it)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.Size() == 0 {
		return ""
	}
	var h [100]byte
	n, _ := io.ReadFull(f, h[:])
	if n < 100 || string(h[:16]) != "SQLite format 3\x00" {
		return "file header is not a SQLite database"
	}
	pageSize := int64(binary.BigEndian.Uint16(h[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if pages := int64(binary.BigEndian.Uint32(h[28:32])); pages != 0 && fi.Size() < pageSize*pages {
		return fmt.Sprintf("file is truncated: %d bytes, header declares %d pages of %d", fi.Size(), pages, pageSize)
	}
	return ""
}

// RestrictModes chmods a database file and its -wal/-shm sidecars (those that exist) to 0600.
func RestrictModes(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(p, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("store: chmod %s: %w", filepath.Base(p), err)
		}
	}
	return nil
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
