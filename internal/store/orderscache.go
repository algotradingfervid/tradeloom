// internal/store/orderscache.go
package store

import (
	"context"
	"database/sql"
)

type CachedOrder struct {
	KiteOrderID string
	Tag         string
	Status      string
	Body        []byte
}

type CachedPosition struct {
	K    string
	Body []byte
}

type OrdersCacheRepo struct{ db *DB }

func NewOrdersCacheRepo(db *DB) *OrdersCacheRepo { return &OrdersCacheRepo{db: db} }

func (r *OrdersCacheRepo) ReplaceOrders(ctx context.Context, rows []CachedOrder, atMs int64) error {
	tx, err := r.db.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM orders_cache`); err != nil {
		return err
	}
	for _, o := range rows {
		if _, err := tx.ExecContext(ctx, `INSERT INTO orders_cache(kite_order_id, tag, body, status, updated_at) VALUES(?, ?, ?, ?, ?)`,
			o.KiteOrderID, o.Tag, string(o.Body), o.Status, atMs); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cache_sync(name, synced_at) VALUES(?, ?)
		ON CONFLICT(name) DO UPDATE SET synced_at = excluded.synced_at`, "orders", atMs); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OrdersCacheRepo) UpsertOrder(ctx context.Context, o CachedOrder, atMs int64) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO orders_cache(kite_order_id, tag, body, status, updated_at) VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(kite_order_id) DO UPDATE SET tag = excluded.tag, body = excluded.body, status = excluded.status, updated_at = excluded.updated_at`,
		o.KiteOrderID, o.Tag, string(o.Body), o.Status, atMs)
	return err
}

func (r *OrdersCacheRepo) ListOrders(ctx context.Context) ([]CachedOrder, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT kite_order_id, COALESCE(tag, ''), body, COALESCE(status, '') FROM orders_cache ORDER BY kite_order_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CachedOrder
	for rows.Next() {
		var o CachedOrder
		var body string
		if err := rows.Scan(&o.KiteOrderID, &o.Tag, &body, &o.Status); err != nil {
			return nil, err
		}
		o.Body = []byte(body)
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *OrdersCacheRepo) ReplacePositions(ctx context.Context, rows []CachedPosition, atMs int64) error {
	tx, err := r.db.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM positions_cache`); err != nil {
		return err
	}
	for _, p := range rows {
		if _, err := tx.ExecContext(ctx, `INSERT INTO positions_cache(k, body, updated_at) VALUES(?, ?, ?)`, p.K, string(p.Body), atMs); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cache_sync(name, synced_at) VALUES(?, ?)
		ON CONFLICT(name) DO UPDATE SET synced_at = excluded.synced_at`, "positions", atMs); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *OrdersCacheRepo) ListPositions(ctx context.Context) ([]CachedPosition, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT k, body FROM positions_cache ORDER BY k`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CachedPosition
	for rows.Next() {
		var p CachedPosition
		var body string
		if err := rows.Scan(&p.K, &body); err != nil {
			return nil, err
		}
		p.Body = []byte(body)
		out = append(out, p)
	}
	return out, rows.Err()
}

// LastUpdated returns when orders and positions were last replaced (0 = never synced).
func (r *OrdersCacheRepo) LastUpdated(ctx context.Context) (int64, int64, error) {
	var o, p int64
	for name, dst := range map[string]*int64{"orders": &o, "positions": &p} {
		err := r.db.R.QueryRowContext(ctx, `SELECT synced_at FROM cache_sync WHERE name = ?`, name).Scan(dst)
		if err != nil && err != sql.ErrNoRows {
			return 0, 0, err
		}
	}
	return o, p, nil
}
