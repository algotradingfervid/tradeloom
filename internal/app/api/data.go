// internal/app/api/data.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

// Dialogs are the native file pickers (runtime.OpenDirectoryDialog / OpenFileDialog); "" means cancelled.
type Dialogs struct {
	Directory func(title string) (string, error)
	File      func(title string, patterns []string) (string, error)
}

type Data struct {
	svc     *service.Service
	ctx     func() context.Context
	dialogs Dialogs
}

func NewData(svc *service.Service, ctx func() context.Context, d Dialogs) *Data {
	return &Data{svc: svc, ctx: ctx, dialogs: d}
}

func (d *Data) Stats() (service.DataStats, error) { return d.svc.DataStats(d.ctx()) }
func (d *Data) ClearCandles() error               { return d.svc.ClearCandles(d.ctx()) }
func (d *Data) RefreshInstruments() error         { return d.svc.RefreshInstruments(d.ctx()) }
func (d *Data) ChooseFolder() (string, error)     { return d.dialogs.Directory("Choose a backup folder") }
func (d *Data) ChooseBackupFile() (string, error) {
	return d.dialogs.File("Choose a backup archive", []string{"*.tar.zst"})
}
func (d *Data) BackupNow(folder string, includeCandles bool) (string, error) {
	return d.svc.BackupNow(d.ctx(), folder, includeCandles)
}
func (d *Data) InspectBackup(path string) (service.BackupInfo, error) {
	return d.svc.InspectBackup(d.ctx(), path)
}
func (d *Data) Restore(path, confirmation string) (string, error) {
	return d.svc.RestoreBackup(d.ctx(), path, confirmation)
}
