// internal/app/api/chart.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Chart struct {
	svc *service.Service
	ctx func() context.Context
}

func NewChart(svc *service.Service, ctx func() context.Context) *Chart {
	return &Chart{svc: svc, ctx: ctx}
}

func (c *Chart) Candles(req service.CandlesRequest) (service.CandlesResult, error) {
	return c.svc.Candles(c.ctx(), req)
}
func (c *Chart) Refresh(req service.RefreshRequest) (service.CandlesResult, error) {
	return c.svc.RefreshCandles(c.ctx(), req)
}
func (c *Chart) OpenLive(token uint32, interval string) (string, error) {
	return c.svc.OpenLive(c.ctx(), token, interval)
}
func (c *Chart) CloseLive(id string) error { return c.svc.CloseLive(c.ctx(), id) }
