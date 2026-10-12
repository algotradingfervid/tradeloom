> **P0-3 Session & Service Layer Implementation Plan — Task 13.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 13: `WatchlistsRepo`

**Files:**
- Create: `internal/store/watchlists.go`
- Test: `internal/store/watchlists_test.go` (package `store_test`)

**Interfaces:**
- Consumes: v1 tables `watchlists(id, name, position, created_at, updated_at, deleted_at)` and `watchlist_items(watchlist_id, token, position, PRIMARY KEY(watchlist_id, token))` (ARCH §4.2); `(*DB).W` / `.R` (P0-1); `ErrNoRow`, `ErrDuplicate`, `storetest.Open` (Task 5).
- Produces:
  ```go
  type WatchlistItemRow struct{ Token uint32; Position int }
  type WatchlistRow struct{ ID, Name string; Position int; Items []WatchlistItemRow }
  func NewWatchlistsRepo(db *DB) *WatchlistsRepo
  func (r *WatchlistsRepo) List(ctx context.Context) ([]WatchlistRow, error)           // live lists by position, items by position
  func (r *WatchlistsRepo) Create(ctx context.Context, id, name string, nowMs int64) error // appended last
  func (r *WatchlistsRepo) Rename(ctx context.Context, id, name string, nowMs int64) error // ErrNoRow
  func (r *WatchlistsRepo) SoftDelete(ctx context.Context, id string, nowMs int64) error   // ErrNoRow
  func (r *WatchlistsRepo) ReorderLists(ctx context.Context, ids []string, nowMs int64) error
  func (r *WatchlistsRepo) AddItem(ctx context.Context, id string, token uint32, at int) (int, error) // at<0 or >len: append; ErrDuplicate; ErrNoRow
  func (r *WatchlistsRepo) RemoveItem(ctx context.Context, id string, token uint32) (int, error)      // returns old position; ErrNoRow
  func (r *WatchlistsRepo) ReorderItems(ctx context.Context, id string, tokens []uint32) error
  ```

- [ ] **Step 1: Write the failing test**

```go
// internal/store/watchlists_test.go
package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func tokens(w store.WatchlistRow) []uint32 {
	var out []uint32
	for _, it := range w.Items {
		out = append(out, it.Token)
	}
	return out
}

func TestWatchlistsRepo(t *testing.T) {
	ctx := context.Background()
	r := store.NewWatchlistsRepo(storetest.Open(t))
	for i, id := range []string{"a", "b"} {
		if err := r.Create(ctx, id, "Watchlist "+string(rune('1'+i)), int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	for _, tok := range []uint32{10, 20, 30} {
		if _, err := r.AddItem(ctx, "a", tok, -1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.AddItem(ctx, "a", 20, -1); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("duplicate add = %v", err)
	}
	if _, err := r.AddItem(ctx, "zz", 1, -1); !errors.Is(err, store.ErrNoRow) {
		t.Fatalf("missing list add = %v", err)
	}
	pos, err := r.RemoveItem(ctx, "a", 20)
	if err != nil || pos != 1 {
		t.Fatalf("remove pos %d err %v", pos, err)
	}
	if got, err := r.AddItem(ctx, "a", 20, pos); err != nil || got != 1 { // undo restores the position
		t.Fatalf("re-add %d %v", got, err)
	}
	if err := r.ReorderItems(ctx, "a", []uint32{30, 10, 20}); err != nil {
		t.Fatal(err)
	}
	if err := r.Rename(ctx, "b", "Banks", 5); err != nil {
		t.Fatal(err)
	}
	if err := r.Rename(ctx, "zz", "x", 5); !errors.Is(err, store.ErrNoRow) {
		t.Fatalf("rename missing = %v", err)
	}
	if err := r.ReorderLists(ctx, []string{"b", "a"}, 6); err != nil {
		t.Fatal(err)
	}
	lists, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 2 || lists[0].ID != "b" || lists[0].Name != "Banks" || !reflect.DeepEqual(tokens(lists[1]), []uint32{30, 10, 20}) {
		t.Fatalf("lists %+v", lists)
	}
	if err := r.SoftDelete(ctx, "b", 7); err != nil {
		t.Fatal(err)
	}
	if lists, _ = r.List(ctx); len(lists) != 1 || lists[0].ID != "a" {
		t.Fatalf("after delete %+v", lists)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/store/ -run TestWatchlistsRepo -v`
Expected: FAIL to compile with `undefined: store.NewWatchlistsRepo`.

- [ ] **Step 3: Write the implementation**

```go
// internal/store/watchlists.go
package store

import (
	"context"
	"database/sql"
)

type WatchlistItemRow struct {
	Token    uint32
	Position int
}

type WatchlistRow struct {
	ID       string
	Name     string
	Position int
	Items    []WatchlistItemRow
}

type WatchlistsRepo struct{ db *DB }

func NewWatchlistsRepo(db *DB) *WatchlistsRepo { return &WatchlistsRepo{db: db} }

func (r *WatchlistsRepo) List(ctx context.Context) ([]WatchlistRow, error) {
	rows, err := r.db.R.QueryContext(ctx,
		`SELECT id, name, position FROM watchlists WHERE deleted_at IS NULL ORDER BY position, created_at`)
	if err != nil {
		return nil, err
	}
	var out []WatchlistRow
	for rows.Next() {
		var w WatchlistRow
		if err := rows.Scan(&w.ID, &w.Name, &w.Position); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, w)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		items, err := r.db.R.QueryContext(ctx,
			`SELECT token, position FROM watchlist_items WHERE watchlist_id = ? ORDER BY position`, out[i].ID)
		if err != nil {
			return nil, err
		}
		for items.Next() {
			var it WatchlistItemRow
			if err := items.Scan(&it.Token, &it.Position); err != nil {
				items.Close()
				return nil, err
			}
			out[i].Items = append(out[i].Items, it)
		}
		items.Close()
		if err := items.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *WatchlistsRepo) Create(ctx context.Context, id, name string, nowMs int64) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO watchlists(id, name, position, created_at, updated_at, deleted_at)
		 VALUES(?, ?, (SELECT COALESCE(MAX(position), -1) + 1 FROM watchlists WHERE deleted_at IS NULL), ?, ?, NULL)`,
		id, name, nowMs, nowMs)
	return err
}

func affectedOrNoRow(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNoRow
	}
	return nil
}

func (r *WatchlistsRepo) Rename(ctx context.Context, id, name string, nowMs int64) error {
	return affectedOrNoRow(r.db.W.ExecContext(ctx,
		`UPDATE watchlists SET name = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, name, nowMs, id))
}

func (r *WatchlistsRepo) SoftDelete(ctx context.Context, id string, nowMs int64) error {
	return affectedOrNoRow(r.db.W.ExecContext(ctx,
		`UPDATE watchlists SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, nowMs, nowMs, id))
}

func (r *WatchlistsRepo) ReorderLists(ctx context.Context, ids []string, nowMs int64) error {
	tx, err := r.db.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE watchlists SET position = ?, updated_at = ? WHERE id = ?`, i, nowMs, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *WatchlistsRepo) AddItem(ctx context.Context, id string, token uint32, at int) (int, error) {
	tx, err := r.db.W.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var live, exists, n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM watchlists WHERE id = ? AND deleted_at IS NULL`, id).Scan(&live); err != nil {
		return 0, err
	}
	if live == 0 {
		return 0, ErrNoRow
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM watchlist_items WHERE watchlist_id = ? AND token = ?`, id, token).Scan(&exists); err != nil {
		return 0, err
	}
	if exists > 0 {
		return 0, ErrDuplicate
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM watchlist_items WHERE watchlist_id = ?`, id).Scan(&n); err != nil {
		return 0, err
	}
	if at < 0 || at > n {
		at = n
	}
	if _, err := tx.ExecContext(ctx, `UPDATE watchlist_items SET position = position + 1 WHERE watchlist_id = ? AND position >= ?`, id, at); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO watchlist_items(watchlist_id, token, position) VALUES(?, ?, ?)`, id, token, at); err != nil {
		return 0, err
	}
	return at, tx.Commit()
}

func (r *WatchlistsRepo) RemoveItem(ctx context.Context, id string, token uint32) (int, error) {
	tx, err := r.db.W.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var pos int
	err = tx.QueryRowContext(ctx, `SELECT position FROM watchlist_items WHERE watchlist_id = ? AND token = ?`, id, token).Scan(&pos)
	if err == sql.ErrNoRows {
		return 0, ErrNoRow
	}
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM watchlist_items WHERE watchlist_id = ? AND token = ?`, id, token); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE watchlist_items SET position = position - 1 WHERE watchlist_id = ? AND position > ?`, id, pos); err != nil {
		return 0, err
	}
	return pos, tx.Commit()
}

func (r *WatchlistsRepo) ReorderItems(ctx context.Context, id string, tokens []uint32) error {
	tx, err := r.db.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, tok := range tokens {
		if _, err := tx.ExecContext(ctx, `UPDATE watchlist_items SET position = ? WHERE watchlist_id = ? AND token = ?`, i, id, tok); err != nil {
			return err
		}
	}
	return tx.Commit()
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test -tags webkit2_41,noorders ./internal/store/ -run TestWatchlistsRepo -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/watchlists.go internal/store/watchlists_test.go
git commit -m "feat(store): watchlists repository with positional items

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
