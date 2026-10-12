// Package sqlite implements candles.Store and candles.JobStore on candles/candles.db
// (modernc.org/sqlite). It is one of the two packages allowed to import database/sql (rule 4).
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"time"

	_ "modernc.org/sqlite"

	"tradeloom/internal/calendar"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/store"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Options configures Open.
type Options struct {
	Clock clock.Clock
	// Sessions returns the continuous session of token on date d (anchors for resampling).
	// nil: the NSE session from Calendar.
	Sessions func(token uint32, d calendar.Date) (start, end time.Time, ok bool)
	Calendar *calendar.Service // used when Sessions is nil
}

// Store is the SQLite candle store.
type Store struct {
	db  *sql.DB
	clk clock.Clock
	ses func(token uint32, d calendar.Date) (time.Time, time.Time, bool)
}

var _ candles.Store = (*Store)(nil)

const dsnPragmas = "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)" +
	"&_pragma=foreign_keys(1)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(1073741824)&_txlock=immediate"

// Open opens (creating if needed) the candle DB at path and migrates it. A new file gets
// page_size 8192, which SQLite only honours before the first table exists and before WAL.
// An existing file whose header is damaged is refused before any connection opens (a leftover
// -wal would otherwise be replayed into it); the error wraps a *store.CorruptError. candles.db and
// its -wal/-shm sidecars are owner-only (0600).
func Open(ctx context.Context, path string, o Options) (*Store, error) {
	if o.Sessions == nil && o.Calendar == nil {
		return nil, errors.New("candles: Options.Sessions or Options.Calendar is required")
	}
	if err := store.CheckHeader(path); err != nil {
		return nil, fmt.Errorf("candles: %w", err)
	}
	fi, err := os.Stat(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if err != nil || fi.Size() == 0 {
		// Create owner-only first: SQLite gives -wal/-shm the main file's permissions.
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		boot, err := sql.Open("sqlite", fileURI(path))
		if err != nil {
			return nil, err
		}
		_, err = boot.ExecContext(ctx, "PRAGMA page_size=8192; VACUUM;")
		boot.Close()
		if err != nil {
			return nil, fmt.Errorf("candles: init page size: %w", err)
		}
	}
	db, err := sql.Open("sqlite", fileURI(path)+dsnPragmas)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("candles: migrate: %w", err)
	}
	// Migration opened a WAL connection, so the sidecars exist now (controller ruling 6).
	if err := store.RestrictModes(path); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db, clk: o.Clock, ses: o.Sessions}
	if s.ses == nil {
		cal := o.Calendar
		s.ses = func(_ uint32, d calendar.Date) (time.Time, time.Time, bool) { return cal.Session(calendar.NSE, d) }
	}
	return s, nil
}

// fileURI escapes path for a SQLite URI filename (a data dir may contain spaces, '?' or '#').
func fileURI(path string) string { return "file:" + (&url.URL{Path: path}).EscapedPath() }

// migrate runs P0-1's migration runner over candles.db's own embedded migrations (§4.4).
func migrate(ctx context.Context, db *sql.DB) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	m, err := store.NewMigrator(sub)
	if err != nil {
		return err
	}
	var cur int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&cur); err != nil {
		return err
	}
	if cur > m.Latest() {
		return fmt.Errorf("candles.db schema %d is newer than this version supports (%d)", cur, m.Latest())
	}
	return m.Apply(ctx, db, cur)
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// PageSize reports PRAGMA page_size (tests, S0-2).
func (s *Store) PageSize(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&n)
	return n, err
}
