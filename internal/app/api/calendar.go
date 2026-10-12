// internal/app/api/calendar.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Calendar struct {
	svc     *service.Service
	ctx     func() context.Context
	dialogs Dialogs
}

func NewCalendar(svc *service.Service, ctx func() context.Context, d Dialogs) *Calendar {
	return &Calendar{svc: svc, ctx: ctx, dialogs: d}
}

func (c *Calendar) Info() (service.CalendarInfo, error) { return c.svc.CalendarInfo(c.ctx()) }
func (c *Calendar) ChooseImportFile() (string, error) {
	return c.dialogs.File("Choose a calendar CSV", []string{"*.csv"})
}
func (c *Calendar) Import(path string) (service.CalendarInfo, error) {
	return c.svc.ImportCalendar(c.ctx(), path)
}
func (c *Calendar) Upsert(in service.CalendarEntryInput) (service.CalendarInfo, error) {
	return c.svc.UpsertCalendarEntry(c.ctx(), in)
}
func (c *Calendar) Remove(exchange, date string) (service.CalendarInfo, error) {
	return c.svc.RemoveCalendarEntry(c.ctx(), exchange, date)
}
