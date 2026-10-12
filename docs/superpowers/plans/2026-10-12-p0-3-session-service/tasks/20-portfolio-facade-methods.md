> **P0-3 Session & Service Layer Implementation Plan — Task 20.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 20: Portfolio façade methods

**Files:**
- Create: `internal/service/portfolio.go`
- Test: `internal/service/portfolio_test.go`

**Interfaces:**
- Consumes: `PortfolioSync` (Task 14), `orders.ParseScope` (Task 12).
- Produces: `type BookSnapshot = orders.BookSnapshot; type PositionsSnapshot = orders.PositionsSnapshot; type HoldingsSnapshot = orders.HoldingsSnapshot`; `OrderBook`, `Positions`, `Holdings`, `RefreshPortfolio` (signatures in "Produces").

- [ ] **Step 1: Write the failing test**

```go
// internal/service/portfolio_test.go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run TestPortfolio -v`
Expected: FAIL to compile with `undefined: (*Service).OrderBook`.

- [ ] **Step 3: Write the implementation**

```go
// internal/service/portfolio.go
package service

import (
	"context"

	"tradeloom/internal/kite"
	"tradeloom/internal/orders"
)

type (
	BookSnapshot      = orders.BookSnapshot
	PositionsSnapshot = orders.PositionsSnapshot
	HoldingsSnapshot  = orders.HoldingsSnapshot
)

const portfolioStage = "portfolio"

func (s *Service) OrderBook(_ context.Context) BookSnapshot      { return s.d.Orders.Book() }
func (s *Service) Positions(_ context.Context) PositionsSnapshot { return s.d.Orders.Positions() }
func (s *Service) Holdings(_ context.Context) HoldingsSnapshot   { return s.d.Orders.Holdings() }

// RefreshPortfolio is the views' Refresh button (FR-ORD-001/002/003) and FR-ORD-014's "on demand" reconciliation.
func (s *Service) RefreshPortfolio(ctx context.Context, scope string) error {
	sc, err := orders.ParseScope(scope)
	if err != nil {
		return refused(portfolioStage, "invalid_scope", err.Error())
	}
	if s.d.Session.State().State != kite.StateReady {
		return refused(portfolioStage, kite.CodeNotLoggedIn, "Log in to Kite to refresh.")
	}
	if err := s.d.Orders.Refresh(ctx, sc); err != nil {
		return refused(portfolioStage, "refresh_failed", "Could not refresh from Kite: "+err.Error())
	}
	return nil
}
```

- [ ] **Step 4: Run all service tests**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -v`
Expected: PASS (all tests from Tasks 14, 15, 16, 17, 20).

- [ ] **Step 5: Commit**

```bash
git add internal/service/portfolio.go internal/service/portfolio_test.go
git commit -m "feat(service): portfolio read façade and on-demand refresh

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---