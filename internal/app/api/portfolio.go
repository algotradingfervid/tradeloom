// internal/app/api/portfolio.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Portfolio struct {
	svc *service.Service
	ctx func() context.Context
}

func NewPortfolio(svc *service.Service, ctx func() context.Context) *Portfolio {
	return &Portfolio{svc: svc, ctx: ctx}
}

func (p *Portfolio) OrderBook() service.BookSnapshot      { return p.svc.OrderBook(p.ctx()) }
func (p *Portfolio) Positions() service.PositionsSnapshot { return p.svc.Positions(p.ctx()) }
func (p *Portfolio) Holdings() service.HoldingsSnapshot   { return p.svc.Holdings(p.ctx()) }
func (p *Portfolio) Refresh(scope string) error           { return p.svc.RefreshPortfolio(p.ctx(), scope) }
