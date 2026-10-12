// internal/store/windowstate.go
package store

import (
	"context"
	"database/sql"
	"errors"
)

// WindowStateRepo is the UI state key/value table (window geometry, dashboard chart, quote cache …).
type WindowStateRepo struct{ db *DB }

func NewWindowStateRepo(db *DB) *WindowStateRepo { return &WindowStateRepo{db: db} }

// Get returns the stored value, or "" when the key is absent.
func (r *WindowStateRepo) Get(ctx context.Context, k string) (string, error) {
	var v string
	err := r.db.R.QueryRowContext(ctx, `SELECT v FROM window_state WHERE k = ?`, k).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (r *WindowStateRepo) Set(ctx context.Context, k, v string) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO window_state(k, v) VALUES(?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v`, k, v)
	return err
}
