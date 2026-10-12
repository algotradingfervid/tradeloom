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
