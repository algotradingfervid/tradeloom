// internal/store/shortcuts.go
package store

import "context"

// ShortcutRow is one user rebind in the shortcuts table.
type ShortcutRow struct {
	ActionID string
	Binding  string
	Scope    string
}

type ShortcutsRepo struct{ db *DB }

func NewShortcutsRepo(db *DB) *ShortcutsRepo { return &ShortcutsRepo{db: db} }

func (r *ShortcutsRepo) Load(ctx context.Context) ([]ShortcutRow, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT action_id, binding, COALESCE(scope, '') FROM shortcuts ORDER BY action_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShortcutRow
	for rows.Next() {
		var s ShortcutRow
		if err := rows.Scan(&s.ActionID, &s.Binding, &s.Scope); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ShortcutsRepo) Save(ctx context.Context, actionID, binding, scope string) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO shortcuts(action_id, binding, scope) VALUES(?, ?, ?)
		 ON CONFLICT(action_id) DO UPDATE SET binding = excluded.binding, scope = excluded.scope`, actionID, binding, scope)
	return err
}

func (r *ShortcutsRepo) Reset(ctx context.Context) error {
	_, err := r.db.W.ExecContext(ctx, `DELETE FROM shortcuts`)
	return err
}
