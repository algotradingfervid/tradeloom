package service

import (
	"fmt"
	"runtime/debug"

	"context"
	"sync"
	"time"
	"tradeloom/internal/logging"

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

// minRefreshInterval is the minimum gap after a successful refresh of a scope (Ruling 65).
const minRefreshInterval = 2 * time.Second

// RefreshPortfolio is the views' Refresh button (FR-ORD-001/002/003) and FR-ORD-014's "on demand" reconciliation.
// It is single-flight per scope (concurrent callers join the in-flight refresh and get its result) and a call
// within 2 s of a successful refresh of the same scope is a no-op that returns nil.
func (s *Service) RefreshPortfolio(ctx context.Context, scope string) (err error) {
	sc, perr := orders.ParseScope(scope)
	if perr != nil {
		return s.refuse(portfolioStage, refused(portfolioStage, "invalid_scope", perr.Error()))
	}
	if s.d.Session.State().State != kite.StateReady {
		return s.refuse(portfolioStage, refused(portfolioStage, kite.CodeNotLoggedIn, "Log in to Kite to refresh."))
	}

	g := &s.pf
	g.mu.Lock()
	if f, ok := g.flights[sc]; ok {
		f.waiters++
		g.mu.Unlock()
		select {
		case <-f.done:
			return f.err
		case <-ctx.Done():
			return s.refuse(portfolioStage, ctx.Err())
		}
	}
	if last, ok := g.lastOK[sc]; ok && s.d.Clock.Now().Sub(last) < minRefreshInterval {
		g.mu.Unlock()
		return nil
	}
	f := &refreshFlight{done: make(chan struct{})}
	if g.flights == nil {
		g.flights = map[orders.Scope]*refreshFlight{}
		g.lastOK = map[orders.Scope]time.Time{}
	}
	g.flights[sc] = f
	g.mu.Unlock()

	// The cleanup is deferred so a panicking Orders.Refresh can never leave the flight (and its joiners) stuck.
	// A panic is converted into an "internal" refusal returned to the leader and the joiners (not re-panicked).
	defer func() {
		if r := recover(); r != nil {
			msg := logging.Default.RedactString(fmt.Sprint(r))
			s.d.Log.Error("service.portfolio_refresh_panic", "scope", scope, "panic", msg, "stack", logging.Default.RedactString(string(debug.Stack())))
			f.err = refused(portfolioStage, "internal", "Could not refresh: unexpected internal error.")
			err = f.err
		}
		g.mu.Lock()
		delete(g.flights, sc)
		if f.err == nil {
			g.lastOK[sc] = s.d.Clock.Now()
		}
		g.mu.Unlock()
		close(f.done)
	}()
	f.err = s.doRefresh(ctx, scope, sc)
	return f.err
}

func (s *Service) doRefresh(ctx context.Context, scope string, sc orders.Scope) error {
	err := s.d.Orders.Refresh(ctx, sc)
	if err == nil {
		return nil
	}
	if kite.Classify(err) == kite.KindSession {
		return s.refuse(portfolioStage, refused(portfolioStage, kite.CodeNotLoggedIn, "Your Kite session has ended. Log in again."))
	}
	s.d.Log.Warn("service.portfolio_refresh_failed", "scope", scope, "err", err)
	return s.refuse(portfolioStage, refused(portfolioStage, "refresh_failed", "Could not refresh from Kite: "+err.Error()))
}

type refreshFlight struct {
	done    chan struct{}
	err     error
	waiters int
}

// portfolioGate is the zero-value-usable state behind RefreshPortfolio.
type portfolioGate struct {
	mu      sync.Mutex
	flights map[orders.Scope]*refreshFlight
	lastOK  map[orders.Scope]time.Time
}
