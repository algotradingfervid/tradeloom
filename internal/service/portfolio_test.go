package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
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

// gatedSync wraps the fake: Refresh counts calls, optionally blocks on gate, and returns err.
type gatedSync struct {
	*fakeSync
	mu      sync.Mutex
	calls   int
	gate    chan struct{} // nil = do not block
	started chan struct{} // closed on the first call
	err     error
	panicV  any // when set, Refresh panics with it after the gate
	book    orders.BookSnapshot
	pos     orders.PositionsSnapshot
	hold    orders.HoldingsSnapshot
}

func (g *gatedSync) Refresh(_ context.Context, _ orders.Scope) error {
	g.mu.Lock()
	g.calls++
	if g.calls == 1 && g.started != nil {
		close(g.started)
	}
	gate, err, pv := g.gate, g.err, g.panicV
	g.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if pv != nil {
		panic(pv)
	}
	return err
}
func (g *gatedSync) Book() orders.BookSnapshot           { return g.book }
func (g *gatedSync) Positions() orders.PositionsSnapshot { return g.pos }
func (g *gatedSync) Holdings() orders.HoldingsSnapshot   { return g.hold }
func (g *gatedSync) count() int                          { g.mu.Lock(); defer g.mu.Unlock(); return g.calls }

func gated(e *svcEnv) *gatedSync {
	g := &gatedSync{fakeSync: e.sync, started: make(chan struct{})}
	e.svc.d.Orders = g
	return g
}

func TestRefreshPortfolio_ConcurrentCallsShareOneRefresh(t *testing.T) {
	e := newSvcEnv(t)
	g := gated(e)
	g.gate = make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range errs {
		wg.Add(1)
		go func() { defer wg.Done(); errs[i] = e.svc.RefreshPortfolio(context.Background(), "all") }()
	}
	<-g.started
	time.Sleep(20 * time.Millisecond)
	close(g.gate)
	wg.Wait()
	if g.count() != 1 {
		t.Fatalf("Refresh called %d times, want 1", g.count())
	}
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRefreshPortfolio_JoinersGetTheLeadersResult(t *testing.T) {
	e := newSvcEnv(t)
	g := gated(e)
	g.gate = make(chan struct{})
	g.err = errors.New("boom")
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() { defer wg.Done(); errs[i] = e.svc.RefreshPortfolio(context.Background(), "orders") }()
	}
	eventually(t, "3 joiners", func() bool {
		e.svc.pf.mu.Lock()
		defer e.svc.pf.mu.Unlock()
		f := e.svc.pf.flights[orders.ScopeOrders]
		return f != nil && f.waiters == 3
	})
	close(g.gate)
	wg.Wait()
	if g.count() != 1 {
		t.Fatalf("Refresh called %d times, want 1", g.count())
	}
	for _, err := range errs {
		if refusedCode(t, err) != "refresh_failed" {
			t.Fatal(err)
		}
	}
	// A failed refresh does not start the minimum interval: the next call refreshes again.
	g.gate = nil
	_ = e.svc.RefreshPortfolio(context.Background(), "orders")
	if g.count() != 2 {
		t.Fatalf("calls %d, want 2", g.count())
	}
}

func TestRefreshPortfolio_MinIntervalPerScope(t *testing.T) {
	e := newSvcEnv(t)
	g := gated(e)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := e.svc.RefreshPortfolio(ctx, "holdings"); err != nil {
			t.Fatal(err)
		}
	}
	if g.count() != 1 {
		t.Fatalf("second call inside the interval refreshed: %d", g.count())
	}
	if err := e.svc.RefreshPortfolio(ctx, "positions"); err != nil || g.count() != 2 {
		t.Fatalf("other scope must not be throttled: %v %d", err, g.count())
	}
	e.clock.Advance(1999 * time.Millisecond)
	_ = e.svc.RefreshPortfolio(ctx, "holdings")
	if g.count() != 2 {
		t.Fatal("refreshed before 2 s")
	}
	e.clock.Advance(time.Millisecond)
	_ = e.svc.RefreshPortfolio(ctx, "holdings")
	if g.count() != 3 {
		t.Fatalf("no refresh after 2 s: %d", g.count())
	}
}

func TestRefreshPortfolio_EmptyAndAllScopes(t *testing.T) {
	for _, sc := range []string{"", "all"} {
		e := newSvcEnv(t)
		if err := e.svc.RefreshPortfolio(context.Background(), sc); err != nil {
			t.Fatal(err)
		}
		if len(e.sync.refreshed) != 1 || e.sync.refreshed[0] != orders.ScopeAll {
			t.Fatalf("scope %q -> %v", sc, e.sync.refreshed)
		}
	}
}

func TestRefreshPortfolio_RefusalsDoNotCallSync(t *testing.T) {
	for _, st := range []kite.State{kite.StateLoggedOut, kite.StateLoggingIn, kite.StateExpired, kite.StateNoCredentials} {
		e := newSvcEnv(t)
		e.session.setState(kite.SessionState{State: st})
		if err := e.svc.RefreshPortfolio(context.Background(), "all"); refusedCode(t, err) != kite.CodeNotLoggedIn {
			t.Fatalf("%s: %v", st, err)
		}
		if err := e.svc.RefreshPortfolio(context.Background(), "gtt"); refusedCode(t, err) != "invalid_scope" {
			t.Fatalf("%s: %v", st, err)
		}
		if len(e.sync.refreshed) != 0 {
			t.Fatalf("%s: Refresh called on a refusal", st)
		}
	}
}

func TestRefreshPortfolio_SessionErrorMapsToNotLoggedIn(t *testing.T) {
	e := newSvcEnv(t)
	e.sync.refreshErr = kite.APIError("TokenException", 403, "Incorrect api_key or access_token.")
	if err := e.svc.RefreshPortfolio(context.Background(), "all"); refusedCode(t, err) != kite.CodeNotLoggedIn {
		t.Fatal(err)
	}
}

func TestRefreshPortfolio_FailureRedactsSecret(t *testing.T) {
	const secret = "s3cr3t-access-token-xyz"
	logging.RegisterSecret(secret)
	e := newSvcEnv(t)
	e.sync.refreshErr = errors.New("GET failed access_token=" + secret)
	err := e.svc.RefreshPortfolio(context.Background(), "all")
	if refusedCode(t, err) != "refresh_failed" {
		t.Fatal(err)
	}
	if strings.Contains(err.(*ErrRefused).Reason, secret) {
		t.Fatalf("secret leaked: %q", err.(*ErrRefused).Reason)
	}
}

func TestPortfolio_FlagsPassThrough(t *testing.T) {
	e := newSvcEnv(t)
	g := gated(e)
	g.book = orders.BookSnapshot{Stale: true, OutOfScopeCount: 1, Orders: []orders.OrderRow{{OrderID: "1", OutOfScope: true}}}
	g.pos = orders.PositionsSnapshot{Stale: true, Net: []orders.PositionRow{{OutOfScope: true}}}
	g.hold = orders.HoldingsSnapshot{Stale: true, Rows: []orders.HoldingRow{{OutOfScope: true}}}
	ctx := context.Background()
	b, p, h := e.svc.OrderBook(ctx), e.svc.Positions(ctx), e.svc.Holdings(ctx)
	if !b.Stale || !b.Orders[0].OutOfScope || b.OutOfScopeCount != 1 ||
		!p.Stale || !p.Net[0].OutOfScope || !h.Stale || !h.Rows[0].OutOfScope {
		t.Fatalf("flags changed: %+v %+v %+v", b, p, h)
	}
}

func waitWaiters(t *testing.T, e *svcEnv, sc orders.Scope, n int) {
	t.Helper()
	eventually(t, "joiners", func() bool {
		e.svc.pf.mu.Lock()
		defer e.svc.pf.mu.Unlock()
		f := e.svc.pf.flights[sc]
		return f != nil && f.waiters == n
	})
}

func TestRefreshPortfolio_PanicDoesNotStickTheFlight(t *testing.T) {
	const secret = "panic-secret-token-123"
	logging.RegisterSecret(secret)
	e := newSvcEnv(t)
	g := gated(e)
	g.gate = make(chan struct{})
	g.panicV = "kaboom " + secret
	ctx := context.Background()
	leader, joiner := make(chan error, 1), make(chan error, 1)
	go func() { leader <- e.svc.RefreshPortfolio(ctx, "orders") }()
	<-g.started
	go func() { joiner <- e.svc.RefreshPortfolio(ctx, "orders") }()
	waitWaiters(t, e, orders.ScopeOrders, 1)
	close(g.gate)
	for name, ch := range map[string]chan error{"leader": leader, "joiner": joiner} {
		select {
		case err := <-ch:
			if refusedCode(t, err) != "internal" {
				t.Fatalf("%s: %v", name, err)
			}
			if strings.Contains(err.(*ErrRefused).Reason, secret) {
				t.Fatalf("%s: secret leaked", name)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s stuck after a panicking Refresh", name)
		}
	}
	g.mu.Lock()
	g.panicV, g.gate = nil, nil
	g.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- e.svc.RefreshPortfolio(ctx, "orders") }()
	select {
	case err := <-done:
		if err != nil || g.count() != 2 {
			t.Fatalf("later call: %v, calls %d", err, g.count())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("later call stuck")
	}
}

func TestRefreshPortfolio_JoinerContextCancelReturnsPromptly(t *testing.T) {
	e := newSvcEnv(t)
	g := gated(e)
	g.gate = make(chan struct{})
	leader := make(chan error, 1)
	go func() { leader <- e.svc.RefreshPortfolio(context.Background(), "all") }()
	<-g.started
	jctx, cancel := context.WithCancel(context.Background())
	joiner := make(chan error, 1)
	go func() { joiner <- e.svc.RefreshPortfolio(jctx, "all") }()
	waitWaiters(t, e, orders.ScopeAll, 1)
	cancel()
	select {
	case err := <-joiner:
		if err == nil {
			t.Fatal("cancelled joiner returned nil")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled joiner did not return")
	}
	close(g.gate)
	if err := <-leader; err != nil {
		t.Fatal(err)
	}
}
