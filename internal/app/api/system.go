// internal/app/api/system.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type System struct {
	svc *service.Service
	ctx func() context.Context
}

func NewSystem(svc *service.Service, ctx func() context.Context) *System {
	return &System{svc: svc, ctx: ctx}
}

func (s *System) Info() service.AppInfo          { return s.svc.AppInfo(s.ctx()) }
func (s *System) Status() service.StatusSnapshot { return s.svc.Status(s.ctx()) }
