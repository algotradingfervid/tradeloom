// internal/app/api/watchlists.go
package api

import (
	"context"

	"tradeloom/internal/service"
)

type Watchlists struct {
	svc *service.Service
	ctx func() context.Context
}

func NewWatchlists(svc *service.Service, ctx func() context.Context) *Watchlists {
	return &Watchlists{svc: svc, ctx: ctx}
}

func (w *Watchlists) List() (service.WatchlistsState, error) { return w.svc.Watchlists(w.ctx()) }
func (w *Watchlists) Create(name string) (service.WatchlistsState, error) {
	return w.svc.CreateWatchlist(w.ctx(), name)
}
func (w *Watchlists) Rename(id, name string) (service.WatchlistsState, error) {
	return w.svc.RenameWatchlist(w.ctx(), id, name)
}
func (w *Watchlists) Delete(id string) (service.WatchlistsState, error) {
	return w.svc.DeleteWatchlist(w.ctx(), id)
}
func (w *Watchlists) SetActive(id string) (service.WatchlistsState, error) {
	return w.svc.SetActiveWatchlist(w.ctx(), id)
}
func (w *Watchlists) Add(id string, token uint32, position int) (service.WatchlistsState, error) {
	return w.svc.AddToWatchlist(w.ctx(), id, token, position)
}
func (w *Watchlists) Remove(id string, token uint32) (service.RemovedItem, error) {
	return w.svc.RemoveFromWatchlist(w.ctx(), id, token)
}
func (w *Watchlists) Reorder(id string, tokens []uint32) (service.WatchlistsState, error) {
	return w.svc.ReorderWatchlist(w.ctx(), id, tokens)
}
func (w *Watchlists) ReorderLists(ids []string) (service.WatchlistsState, error) {
	return w.svc.ReorderWatchlists(w.ctx(), ids)
}
