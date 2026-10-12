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

func TestWatchlistsReorderValidation(t *testing.T) {
	ctx := context.Background()
	r := store.NewWatchlistsRepo(storetest.Open(t))
	for i, id := range []string{"a", "b", "c"} {
		if err := r.Create(ctx, id, id, int64(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.SoftDelete(ctx, "c", 9); err != nil {
		t.Fatal(err)
	}
	for _, tok := range []uint32{10, 20, 30} {
		if _, err := r.AddItem(ctx, "a", tok, -1); err != nil {
			t.Fatal(err)
		}
	}
	listOrder := func() []string {
		ls, err := r.List(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for i, l := range ls {
			if l.Position != i {
				t.Fatalf("list %s position %d want %d", l.ID, l.Position, i)
			}
			ids = append(ids, l.ID)
		}
		return ids
	}
	for name, ids := range map[string][]string{
		"missing":   {"a"},
		"duplicate": {"a", "a"},
		"unknown":   {"a", "zz"},
		"deleted":   {"a", "b", "c"},
		"extra":     {"a", "b", "zz"},
	} {
		if err := r.ReorderLists(ctx, ids, 10); !errors.Is(err, store.ErrConflict) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	if got := listOrder(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("lists mutated: %v", got)
	}

	for name, toks := range map[string][]uint32{
		"missing":   {10, 20},
		"duplicate": {10, 10, 20},
		"unknown":   {10, 20, 99},
		"extra":     {10, 20, 30, 40},
	} {
		if err := r.ReorderItems(ctx, "a", toks); !errors.Is(err, store.ErrConflict) {
			t.Fatalf("items %s: err = %v", name, err)
		}
	}
	for _, id := range []string{"c", "zz"} {
		if err := r.ReorderItems(ctx, id, nil); !errors.Is(err, store.ErrNoRow) {
			t.Fatalf("items on %s: err = %v", id, err)
		}
	}
	ls, _ := r.List(ctx)
	if got := tokens(ls[0]); !reflect.DeepEqual(got, []uint32{10, 20, 30}) {
		t.Fatalf("items mutated: %v", got)
	}

	// mid-list insert shifts; positions stay contiguous, and a full-set reorder still works.
	if _, err := r.AddItem(ctx, "a", 15, 1); err != nil {
		t.Fatal(err)
	}
	ls, _ = r.List(ctx)
	for i, it := range ls[0].Items {
		if it.Position != i {
			t.Fatalf("item %d position %d", it.Token, it.Position)
		}
	}
	if got := tokens(ls[0]); !reflect.DeepEqual(got, []uint32{10, 15, 20, 30}) {
		t.Fatalf("after insert: %v", got)
	}
	if err := r.ReorderItems(ctx, "a", []uint32{30, 20, 15, 10}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReorderLists(ctx, []string{"b", "a"}, 11); err != nil {
		t.Fatal(err)
	}
	if got := listOrder(); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatalf("lists: %v", got)
	}
}
