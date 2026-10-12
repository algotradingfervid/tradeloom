> **P0-3 Session & Service Layer Implementation Plan — Task 5.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 5: `SessionsRepo` over the `sessions` table

**Files:**
- Create: `internal/store/storetest/storetest.go`, `internal/store/errors_p03.go`, `internal/store/sessions.go`
- Test: `internal/store/storetest/storetest_test.go`, `internal/store/sessions_test.go` (external test package `store_test`, so it can import `storetest`)

**Interfaces:**
- Consumes: P0-1 `store.Open(ctx, path, store.Options{AppVersion, BackupDir, Now, Migrations})`, `store.EmbeddedMigrations()`, `(*store.DB).W` / `.R` (`*sql.DB`), `(*store.DB).SchemaVersion()`, `store.SupportedSchema()`, the v1 `sessions` table (ARCH §4.2: `id, user_id, logged_in_at, expires_at, risk_ack_at, data_plan, ended_at, end_reason`).
- Produces:
  ```go
  // package storetest (internal/store/storetest) — P0-1 has no exported temp-DB helper; its openTest lives in package store's tests.
  func Open(t testing.TB) *store.DB // migrated v1 app.db in t.TempDir(); closed by t.Cleanup
  // package store
  var ErrNoRow, ErrDuplicate error
  type SessionRow struct{ ID, UserID string; LoggedInAtMs, ExpiresAtMs, RiskAckAtMs int64; DataPlan string; EndedAtMs int64; EndReason string } // 0/"" = NULL
  func NewSessionsRepo(db *DB) *SessionsRepo
  func (r *SessionsRepo) Insert(ctx context.Context, row SessionRow) error
  func (r *SessionsRepo) SetDataPlan(ctx context.Context, id, plan string) error
  func (r *SessionsRepo) AckRisk(ctx context.Context, id string, atMs int64) error // first ack wins
  func (r *SessionsRepo) End(ctx context.Context, id string, atMs int64, reason string) error // first end wins
  func (r *SessionsRepo) Get(ctx context.Context, id string) (SessionRow, error) // ErrNoRow
  ```

- [ ] **Step 1: Write the `storetest` helper and its test**

```go
// internal/store/storetest/storetest.go
// Package storetest opens a migrated, temporary app.db for tests outside package store
// (kite, service, orders, app). Package store's own tests keep P0-1's in-package helper.
package storetest

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/store"
)

// Open returns a migrated v1 app.db in t.TempDir(). It is closed by t.Cleanup before the directory is removed.
func Open(t testing.TB) *store.DB {
	t.Helper()
	dir := t.TempDir()
	fixed := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	db, err := store.Open(context.Background(), filepath.Join(dir, "app.db"), store.Options{
		AppVersion: "test",
		BackupDir:  filepath.Join(dir, "backups"),
		Now:        func() time.Time { return fixed },
		Migrations: store.EmbeddedMigrations(),
	})
	if err != nil {
		t.Fatalf("storetest: open app.db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
```

```go
// internal/store/storetest/storetest_test.go
package storetest

import (
	"context"
	"testing"

	"tradeloom/internal/store"
)

func TestOpen_MigratedAndUsable(t *testing.T) {
	db := Open(t)
	if db.SchemaVersion() != store.SupportedSchema() {
		t.Fatalf("schema %d, want %d", db.SchemaVersion(), store.SupportedSchema())
	}
	var n int
	if err := db.R.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("sessions table: n=%d err=%v", n, err)
	}
	if _, err := db.W.ExecContext(context.Background(), `INSERT INTO watchlists(id, name, position, created_at, updated_at, deleted_at) VALUES('w', 'W', 0, 1, 1, NULL)`); err != nil {
		t.Fatalf("writer: %v", err)
	}
}
```

Run: `go test -tags webkit2_41,noorders ./internal/store/storetest/ -v`
Expected: PASS (`TestOpen_MigratedAndUsable`).

- [ ] **Step 2: Write the failing test**

```go
// internal/store/sessions_test.go
package store_test

import (
	"context"
	"errors"
	"testing"

	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func TestSessionsRepo_Lifecycle(t *testing.T) {
	ctx := context.Background()
	r := store.NewSessionsRepo(storetest.Open(t))
	if _, err := r.Get(ctx, "missing"); !errors.Is(err, store.ErrNoRow) {
		t.Fatalf("Get missing = %v, want ErrNoRow", err)
	}
	row := store.SessionRow{ID: "s1", UserID: "AB1234", LoggedInAtMs: 1000, ExpiresAtMs: 2000, DataPlan: "unknown"}
	if err := r.Insert(ctx, row); err != nil {
		t.Fatal(err)
	}
	if err := r.SetDataPlan(ctx, "s1", "connect"); err != nil {
		t.Fatal(err)
	}
	if err := r.AckRisk(ctx, "s1", 1500); err != nil {
		t.Fatal(err)
	}
	if err := r.AckRisk(ctx, "s1", 1600); err != nil { // second ack keeps the first time
		t.Fatal(err)
	}
	if err := r.End(ctx, "s1", 1900, "logout"); err != nil {
		t.Fatal(err)
	}
	if err := r.End(ctx, "s1", 1950, "session_expired"); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	want := store.SessionRow{ID: "s1", UserID: "AB1234", LoggedInAtMs: 1000, ExpiresAtMs: 2000, RiskAckAtMs: 1500, DataPlan: "connect", EndedAtMs: 1900, EndReason: "logout"}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/store/ -run TestSessionsRepo -v`
Expected: FAIL to compile with `undefined: store.NewSessionsRepo`.

- [ ] **Step 4: Write the implementation**

```go
// internal/store/errors_p03.go
package store

import "errors"

// ErrNoRow is returned by P0-3 repositories when a row does not exist (callers never see database/sql errors).
var ErrNoRow = errors.New("store: no such row")

// ErrDuplicate is returned when an insert would duplicate a unique key.
var ErrDuplicate = errors.New("store: duplicate")
```

```go
// internal/store/sessions.go
package store

import (
	"context"
	"database/sql"
	"errors"
)

// SessionRow mirrors the sessions table; zero values stand for NULL.
type SessionRow struct {
	ID           string
	UserID       string
	LoggedInAtMs int64
	ExpiresAtMs  int64
	RiskAckAtMs  int64
	DataPlan     string
	EndedAtMs    int64
	EndReason    string
}

type SessionsRepo struct{ db *DB }

func NewSessionsRepo(db *DB) *SessionsRepo { return &SessionsRepo{db: db} }

func (r *SessionsRepo) Insert(ctx context.Context, row SessionRow) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, logged_in_at, expires_at, risk_ack_at, data_plan, ended_at, end_reason)
		 VALUES(?, ?, ?, ?, NULL, ?, NULL, NULL)`,
		row.ID, row.UserID, row.LoggedInAtMs, row.ExpiresAtMs, row.DataPlan)
	return err
}

func (r *SessionsRepo) SetDataPlan(ctx context.Context, id, plan string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE sessions SET data_plan = ? WHERE id = ?`, plan, id)
	return err
}

func (r *SessionsRepo) AckRisk(ctx context.Context, id string, atMs int64) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE sessions SET risk_ack_at = ? WHERE id = ? AND risk_ack_at IS NULL`, atMs, id)
	return err
}

func (r *SessionsRepo) End(ctx context.Context, id string, atMs int64, reason string) error {
	_, err := r.db.W.ExecContext(ctx,
		`UPDATE sessions SET ended_at = ?, end_reason = ? WHERE id = ? AND ended_at IS NULL`, atMs, reason, id)
	return err
}

func (r *SessionsRepo) Get(ctx context.Context, id string) (SessionRow, error) {
	var (
		row                           SessionRow
		user, plan, reason            sql.NullString
		loggedIn, expires, ack, ended sql.NullInt64
	)
	err := r.db.R.QueryRowContext(ctx,
		`SELECT id, user_id, logged_in_at, expires_at, risk_ack_at, data_plan, ended_at, end_reason FROM sessions WHERE id = ?`, id).
		Scan(&row.ID, &user, &loggedIn, &expires, &ack, &plan, &ended, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionRow{}, ErrNoRow
	}
	if err != nil {
		return SessionRow{}, err
	}
	row.UserID, row.DataPlan, row.EndReason = user.String, plan.String, reason.String
	row.LoggedInAtMs, row.ExpiresAtMs, row.RiskAckAtMs, row.EndedAtMs = loggedIn.Int64, expires.Int64, ack.Int64, ended.Int64
	return row, nil
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test -tags webkit2_41,noorders ./internal/store/ ./internal/store/storetest/ -run 'TestSessionsRepo|TestOpen_MigratedAndUsable' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store/storetest internal/store/errors_p03.go internal/store/sessions.go internal/store/sessions_test.go
git commit -m "feat(store): storetest helper and sessions repository for login sessions and risk acks

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---