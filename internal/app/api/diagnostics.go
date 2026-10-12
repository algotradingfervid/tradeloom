// internal/app/api/diagnostics.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Diagnostics struct {
	svc  *service.Service
	ctx  func() context.Context
	open func(url string)
}

func NewDiagnostics(svc *service.Service, ctx func() context.Context, open func(string)) *Diagnostics {
	return &Diagnostics{svc: svc, ctx: ctx, open: open}
}

// OpenLogsFolder shows the logs directory in the desktop's file manager (file:// through BrowserOpenURL).
func (d *Diagnostics) OpenLogsFolder() error {
	dir, err := d.svc.OpenLogsFolder(d.ctx())
	if err != nil {
		return err
	}
	d.open("file://" + dir)
	return nil
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
	d.open("file://" + path)
	return nil
}

func (d *Diagnostics) DismissCrash() error { return d.svc.DismissCrash(d.ctx()) }
