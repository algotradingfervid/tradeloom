> **P0-3 Session & Service Layer Implementation Plan — Task 18.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 18: `OrdersCacheRepo`

**Files:**
- Create: `internal/store/orderscache.go`
- Test: `internal/store/orderscache_test.go` (package `store_test`)

**Interfaces:**
- Consumes: v1 tables `orders_cache(kite_order_id PK, tag, body, status, updated_at)` and `positions_cache(k PK, body, updated_at)`; `(*DB).W` / `.R` (P0-1); `storetest.Open` (Task 5).
- Produces:
  ```go
  type CachedOrder struct{ KiteOrderID, Tag, Status string; Body []byte }
  type CachedPosition struct{ K string; Body []byte }
  func NewOrdersCacheRepo(db *DB) *OrdersCacheRepo
  func (r *OrdersCacheRepo) ReplaceOrders(ctx context.Context, rows []CachedOrder, atMs int64) error
  func (r *OrdersCacheRepo) UpsertOrder(ctx context.Context, row CachedOrder, atMs int64) error
  func (r *OrdersCacheRepo) ListOrders(ctx context.Context) ([]CachedOrder, error)
  func (r *OrdersCacheRepo) ReplacePositions(ctx context.Context, rows []CachedPosition, atMs int64) error
  func (r *OrdersCacheRepo) ListPositions(ctx context.Context) ([]CachedPosition, error)
  func (r *OrdersCacheRepo) LastUpdated(ctx context.Context) (ordersAtMs, positionsAtMs int64, err error)
  ```

The v1 schema has no holdings cache. Holdings stay in memory and are re-fetched on login (5-minute refresh plus order-update triggers).

- [ ] **Step 1: Write the failing test**

```go
// internal/store/orderscache_test.go
package store_test

import (
	"context"
	"testing"

	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func TestOrdersCacheRepo(t *testing.T) {
	ctx := context.Background()
	r := store.NewOrdersCacheRepo(storetest.Open(t))
	if err := r.ReplaceOrders(ctx, []store.CachedOrder{{KiteOrderID: "1", Status: "OPEN", Body: []byte(`{"order_id":"1"}`)}, {KiteOrderID: "2", Status: "OPEN", Body: []byte(`{}`)}}, 100); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertOrder(ctx, store.CachedOrder{KiteOrderID: "1", Status: "COMPLETE", Body: []byte(`{"order_id":"1","status":"COMPLETE"}`)}, 150); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceOrders(ctx, []store.CachedOrder{{KiteOrderID: "1", Status: "COMPLETE", Body: []byte(`{"order_id":"1","status":"COMPLETE"}`)}}, 200); err != nil {
		t.Fatal(err)
	}
	os, err := r.ListOrders(ctx)
	if err != nil || len(os) != 1 || os[0].Status != "COMPLETE" {
		t.Fatalf("%+v %v", os, err)
	}
	if err := r.ReplacePositions(ctx, []store.CachedPosition{{K: "net|NSE|INFY|MIS", Body: []byte(`{}`)}}, 300); err != nil {
		t.Fatal(err)
	}
	ps, _ := r.ListPositions(ctx)
	oAt, pAt, err := r.LastUpdated(ctx)
	if len(ps) != 1 || oAt != 200 || pAt != 300 || err != nil {
		t.Fatalf("ps %+v at %d/%d %v", ps, oAt, pAt, err)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/store/ -run TestOrdersCacheRepo -v`
Expected: FAIL to compile with `undefined: store.NewOrdersCacheRepo`.

- [ ] **Step 3: Write the implementation**

```go
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
	rows, err := r.db.R.QueryContext(ctx, `SELECT kite_order_id, COALESCE(tag, ''), body, COALESCE(status, '') FROM orders_cache`)
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

func (r *OrdersCacheRepo) LastUpdated(ctx context.Context) (int64, int64, error) {
	var o, p sql.NullInt64
	if err := r.db.R.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM orders_cache`).Scan(&o); err != nil {
		return 0, 0, err
	}
	if err := r.db.R.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM positions_cache`).Scan(&p); err != nil {
		return 0, 0, err
	}
	return o.Int64, p.Int64, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test -tags webkit2_41,noorders ./internal/store/ -run TestOrdersCacheRepo -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/orderscache.go internal/store/orderscache_test.go
git commit -m "feat(store): orders and positions cache repository

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
