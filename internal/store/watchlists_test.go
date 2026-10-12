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
