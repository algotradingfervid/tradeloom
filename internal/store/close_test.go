package store

import "testing"

// P0-3 Task 23: a restore closes app.db early (CloseForRestore) and app.Boot.Shutdown closes it again;
// the second Close must succeed so the exit is still marked clean.
func TestClose_IsIdempotent(t *testing.T) {
	db := mustOpen(t, t.TempDir(), nil, "test")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
