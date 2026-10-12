// internal/app/api/instruments.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Instruments struct {
	svc *service.Service
	ctx func() context.Context
}

func NewInstruments(svc *service.Service, ctx func() context.Context) *Instruments {
	return &Instruments{svc: svc, ctx: ctx}
}

func (i *Instruments) Search(q string, tags []string, limit int) ([]service.InstrumentDTO, error) {
	return i.svc.SearchInstruments(i.ctx(), q, tags, limit)
}
func (i *Instruments) ByTokens(tokens []uint32) ([]service.InstrumentDTO, error) {
	return i.svc.InstrumentsByTokens(i.ctx(), tokens)
}
func (i *Instruments) ByIdentifier(id string) (service.InstrumentDTO, error) {
	return i.svc.InstrumentByIdentifier(i.ctx(), id)
}
