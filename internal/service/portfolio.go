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
		return s.refuse(portfolioStage, refused(portfolioStage, "invalid_scope", err.Error()))
	}
	if s.d.Session.State().State != kite.StateReady {
		return refused(portfolioStage, kite.CodeNotLoggedIn, "Log in to Kite to refresh.")
	}
	if err := s.d.Orders.Refresh(ctx, sc); err != nil {
		s.d.Log.Warn("service.portfolio_refresh_failed", "scope", scope, "err", err)
		return s.refuse(portfolioStage, refused(portfolioStage, "refresh_failed", "Could not refresh from Kite: "+err.Error()))
	}
	return nil
}
