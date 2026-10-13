// internal/store/shortcuts_test.go
package store_test

import (
	"context"
	"testing"

	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func TestShortcutsRepo(t *testing.T) {
	ctx := context.Background()
	r := store.NewShortcutsRepo(storetest.Open(t))
	if rows, err := r.Load(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("fresh %+v %v", rows, err)
	}
	if err := r.Save(ctx, "search.open", "Ctrl+P", "global"); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(ctx, "search.open", "Ctrl+O", "global"); err != nil { // upsert
		t.Fatal(err)
	}
	if err := r.Save(ctx, "order.new", "Shift+O", "chart"); err != nil {
		t.Fatal(err)
	}
	rows, err := r.Load(ctx)
	if err != nil || len(rows) != 2 || rows[0].ActionID != "order.new" || rows[1].ActionID != "search.open" || rows[1].Binding != "Ctrl+O" || rows[0].Scope != "chart" {
		t.Fatalf("%+v %v", rows, err)
	}
	if err := r.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, _ := r.Load(ctx); len(rows) != 0 {
		t.Fatalf("after reset %+v", rows)
	}
}

func TestWindowStateRepo(t *testing.T) {
	ctx := context.Background()
	r := store.NewWindowStateRepo(storetest.Open(t))
	if v, err := r.Get(ctx, "window.geometry"); err != nil || v != "" {
		t.Fatalf("absent %q %v", v, err)
	}
	if err := r.Set(ctx, "window.geometry", `{"w":1}`); err != nil {
		t.Fatal(err)
	}
	if err := r.Set(ctx, "window.geometry", `{"w":2}`); err != nil {
		t.Fatal(err)
	}
	if v, err := r.Get(ctx, "window.geometry"); err != nil || v != `{"w":2}` {
		t.Fatalf("%q %v", v, err)
	}
}
