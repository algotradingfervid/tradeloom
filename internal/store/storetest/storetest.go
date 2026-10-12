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
