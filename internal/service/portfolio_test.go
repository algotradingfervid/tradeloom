package service

import (
	"context"
	"testing"

	"tradeloom/internal/kite"
	"tradeloom/internal/orders"
)

func TestPortfolio(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	if e.svc.OrderBook(ctx).AsOfMs != 1 || e.svc.Positions(ctx).AsOfMs != 2 || e.svc.Holdings(ctx).AsOfMs != 3 {
		t.Fatal("snapshots not passed through")
	}
	if err := e.svc.RefreshPortfolio(ctx, "holdings"); err != nil {
		t.Fatal(err)
	}
	if e.sync.refreshed[0] != orders.ScopeHoldings {
		t.Fatalf("scope %v", e.sync.refreshed)
	}
	if err := e.svc.RefreshPortfolio(ctx, "gtt"); refusedCode(t, err) != "invalid_scope" {
		t.Fatal(err)
	}
	e.session.setState(kite.SessionState{State: kite.StateExpired})
	if err := e.svc.RefreshPortfolio(ctx, "all"); refusedCode(t, err) != kite.CodeNotLoggedIn {
		t.Fatal(err)
	}
}
