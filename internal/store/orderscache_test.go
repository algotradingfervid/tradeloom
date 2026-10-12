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

func TestOrdersCacheFixRound1(t *testing.T) {
	ctx := context.Background()
	r := store.NewOrdersCacheRepo(storetest.Open(t))
	if o, p, err := r.LastUpdated(ctx); err != nil || o != 0 || p != 0 {
		t.Fatalf("never synced: %d %d %v", o, p, err)
	}
	// Body and Tag round-trip; ordered by kite_order_id.
	in := []store.CachedOrder{
		{KiteOrderID: "b", Tag: "tl-2", Status: "OPEN", Body: []byte(`{"x":2}`)},
		{KiteOrderID: "a", Tag: "tl-1", Status: "OPEN", Body: []byte(`{"x":1}`)},
	}
	if err := r.ReplaceOrders(ctx, in, 100); err != nil {
		t.Fatal(err)
	}
	got, err := r.ListOrders(ctx)
	if err != nil || len(got) != 2 || got[0].KiteOrderID != "a" || got[1].KiteOrderID != "b" ||
		got[0].Tag != "tl-1" || string(got[0].Body) != `{"x":1}` || string(got[1].Body) != `{"x":2}` {
		t.Fatalf("round trip %+v %v", got, err)
	}
	// Rollback on duplicate id keeps previous cache and timestamp.
	dup := []store.CachedOrder{{KiteOrderID: "z", Body: []byte(`{}`)}, {KiteOrderID: "z", Body: []byte(`{}`)}}
	if err := r.ReplaceOrders(ctx, dup, 300); err == nil {
		t.Fatal("expected duplicate error")
	}
	got, _ = r.ListOrders(ctx)
	if o, _, _ := r.LastUpdated(ctx); len(got) != 2 || o != 100 {
		t.Fatalf("rollback: %+v at %d", got, o)
	}
	// Empty replace still advances the timestamp.
	if err := r.ReplaceOrders(ctx, nil, 400); err != nil {
		t.Fatal(err)
	}
	got, _ = r.ListOrders(ctx)
	if o, _, err := r.LastUpdated(ctx); len(got) != 0 || o != 400 || err != nil {
		t.Fatalf("empty replace: %+v at %d %v", got, o, err)
	}
	if err := r.ReplacePositions(ctx, nil, 500); err != nil {
		t.Fatal(err)
	}
	ps, err := r.ListPositions(ctx)
	if o, p, e := r.LastUpdated(ctx); err != nil || len(ps) != 0 || o != 400 || p != 500 || e != nil {
		t.Fatalf("positions: %+v %d/%d %v %v", ps, o, p, err, e)
	}
}
