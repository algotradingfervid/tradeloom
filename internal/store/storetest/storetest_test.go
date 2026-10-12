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
