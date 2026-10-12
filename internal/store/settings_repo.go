package store

import "context"

// SettingsRepo persists settings rows (key → JSON text). It implements settings.Backend.
type SettingsRepo struct{ db *DB }

// SettingsRepo returns the settings table repository.
func (db *DB) SettingsRepo() *SettingsRepo { return &SettingsRepo{db: db} }

// Load returns every row.
func (r *SettingsRepo) Load(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.R.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Save upserts one row.
func (r *SettingsRepo) Save(ctx context.Context, key, value string) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO settings(key, value, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, r.db.now().UnixMilli())
	return err
}
