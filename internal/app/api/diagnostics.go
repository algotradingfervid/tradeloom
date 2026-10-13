// internal/app/api/diagnostics.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Diagnostics struct {
	svc  *service.Service
	ctx  func() context.Context
	open func(path string) error
}

// NewDiagnostics takes the app's file opener (checked open / xdg-open of the logs folder or a file in it).
func NewDiagnostics(svc *service.Service, ctx func() context.Context, open func(string) error) *Diagnostics {
	return &Diagnostics{svc: svc, ctx: ctx, open: open}
}

// OpenLogsFolder shows the logs directory in the desktop's file manager; an opener failure is returned.
func (d *Diagnostics) OpenLogsFolder() error {
	dir, err := d.svc.OpenLogsFolder(d.ctx())
	if err != nil {
		return err
	}
	return d.open(dir)
}

func (d *Diagnostics) CreateSupportBundle() (string, error) {
	return d.svc.CreateSupportBundle(d.ctx())
}
func (d *Diagnostics) LastCrash() (service.CrashInfo, error) { return d.svc.LastCrash(d.ctx()) }

func (d *Diagnostics) OpenCrashReport() error {
	path, err := d.svc.OpenCrashReport(d.ctx())
	if err != nil {
		return err
	}
	return d.open(path)
}

func (d *Diagnostics) DismissCrash() error { return d.svc.DismissCrash(d.ctx()) }
