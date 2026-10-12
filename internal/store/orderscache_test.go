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
