// internal/service/watchlists.go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"tradeloom/internal/bus"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

const maxWatchlistName = 40

var (
	SettingActiveWatchlist = settings.Register(settings.Key[string]{Name: "watchlist.active", Default: ""})
	TopicWatchlists        = bus.NewTopic[WatchlistsState]("watchlists.changed")
)

type WatchlistItem struct {
	Token       uint32 `json:"token"`
	Exchange    string `json:"exchange"`
	Symbol      string `json:"symbol"`
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
	IsIndex     bool   `json:"isIndex"`
	Missing     bool   `json:"missing"`
}

type Watchlist struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Items []WatchlistItem `json:"items"`
}

type WatchlistsState struct {
	Lists    []Watchlist `json:"lists"`
	ActiveID string      `json:"activeId"`
}

type RemovedItem struct {
	ListID   string `json:"listId"`
	Token    uint32 `json:"token"`
	Position int    `json:"position"`
}

const wlStage = "watchlists"

func (s *Service) Watchlists(ctx context.Context) (WatchlistsState, error) {
	s.wlMu.Lock()
	defer s.wlMu.Unlock()
	return s.watchlistsLocked(ctx)
}

func (s *Service) watchlistsLocked(ctx context.Context) (WatchlistsState, error) {
	rows, err := s.d.Watchlists.List(ctx)
	if err != nil {
		return WatchlistsState{}, s.refuse(wlStage, err)
	}
	if len(rows) == 0 {
		if err := s.d.Watchlists.Create(ctx, s.d.NewID(), "Watchlist 1", s.nowMs()); err != nil {
			return WatchlistsState{}, s.refuse(wlStage, err)
		}
		if rows, err = s.d.Watchlists.List(ctx); err != nil {
			return WatchlistsState{}, s.refuse(wlStage, err)
		}
	}
	active := settings.Get(s.d.Settings, SettingActiveWatchlist)
	if findList(rows, active) < 0 {
		active = rows[0].ID
		if err := settings.Set(ctx, s.d.Settings, SettingActiveWatchlist, active); err != nil {
			return WatchlistsState{}, s.refuse(wlStage, err)
		}
	}
	st := WatchlistsState{ActiveID: active}
	for _, r := range rows {
		w := Watchlist{ID: r.ID, Name: r.Name, Items: []WatchlistItem{}}
		for _, it := range r.Items {
			in, ok := s.d.Instruments.ByToken(it.Token)
			w.Items = append(w.Items, WatchlistItem{Token: it.Token, Exchange: in.Exchange, Symbol: in.Tradingsymbol,
				DisplayName: in.DisplayName, Name: in.Name, IsIndex: in.IsIndex, Missing: !ok})
		}
		st.Lists = append(st.Lists, w)
	}
	return st, nil
}

// mutate runs fn under the watchlist lock and publishes the resulting state.
func (s *Service) mutate(ctx context.Context, fn func(rows []store.WatchlistRow) error) (WatchlistsState, error) {
	s.wlMu.Lock()
	defer s.wlMu.Unlock()
	if _, err := s.watchlistsLocked(ctx); err != nil {
		return WatchlistsState{}, err
	}
	rows, err := s.d.Watchlists.List(ctx)
	if err != nil {
		return WatchlistsState{}, s.refuse(wlStage, err)
	}
	if err := fn(rows); err != nil {
		return WatchlistsState{}, s.refuse(wlStage, mapStoreErr(err))
	}
	st, err := s.watchlistsLocked(ctx)
	if err == nil {
		bus.Publish(s.d.Bus, TopicWatchlists, st)
	}
	return st, err
}

// mapStoreErr turns repository sentinels into refusals the UI can act on (Ruling 42): a conflicting
// reorder tells the client to refetch; a vanished list or row is not_found.
func mapStoreErr(err error) error {
	switch {
	case errors.Is(err, store.ErrConflict):
		return refused(wlStage, "stale_order", "The list changed; reload and try again.")
	case errors.Is(err, store.ErrNoRow):
		return notFound()
	}
	return err
}

func findList(rows []store.WatchlistRow, id string) int {
	for i, r := range rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}

func validateName(rows []store.WatchlistRow, name, selfID string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", refused(wlStage, "invalid_name", "Enter a watchlist name.")
	}
	if utf8.RuneCountInString(n) > maxWatchlistName {
		return "", refused(wlStage, "invalid_name", "Watchlist names can be up to 40 characters.")
	}
	for _, r := range rows {
		if r.ID != selfID && strings.EqualFold(r.Name, n) {
			return "", refused(wlStage, "duplicate_name", "A watchlist with this name exists")
		}
	}
	return n, nil
}

func nextDefaultName(rows []store.WatchlistRow) string {
	used := map[string]bool{}
	for _, r := range rows {
		used[strings.ToLower(r.Name)] = true
	}
	for i := 1; ; i++ {
		if n := fmt.Sprintf("Watchlist %d", i); !used[strings.ToLower(n)] {
			return n
		}
	}
}

func notFound() error { return refused(wlStage, "not_found", "That watchlist no longer exists.") }

func (s *Service) CreateWatchlist(ctx context.Context, name string) (WatchlistsState, error) {
	return s.mutate(ctx, func(rows []store.WatchlistRow) error {
		if strings.TrimSpace(name) == "" {
			name = nextDefaultName(rows)
		}
		n, err := validateName(rows, name, "")
		if err != nil {
			return err
		}
		id := s.d.NewID()
		if err := s.d.Watchlists.Create(ctx, id, n, s.nowMs()); err != nil {
			return err
		}
		return settings.Set(ctx, s.d.Settings, SettingActiveWatchlist, id)
	})
}

func (s *Service) RenameWatchlist(ctx context.Context, id, name string) (WatchlistsState, error) {
	return s.mutate(ctx, func(rows []store.WatchlistRow) error {
		if findList(rows, id) < 0 {
			return notFound()
		}
		n, err := validateName(rows, name, id)
		if err != nil {
			return err
		}
		return s.d.Watchlists.Rename(ctx, id, n, s.nowMs())
	})
}

func (s *Service) DeleteWatchlist(ctx context.Context, id string) (WatchlistsState, error) {
	return s.mutate(ctx, func(rows []store.WatchlistRow) error {
		idx := findList(rows, id)
		if idx < 0 {
			return notFound()
		}
		if err := s.d.Watchlists.SoftDelete(ctx, id, s.nowMs()); err != nil {
			return err
		}
		remaining := append(append([]store.WatchlistRow{}, rows[:idx]...), rows[idx+1:]...)
		if len(remaining) == 0 {
			nid := s.d.NewID()
			if err := s.d.Watchlists.Create(ctx, nid, "Watchlist 1", s.nowMs()); err != nil {
				return err
			}
			return settings.Set(ctx, s.d.Settings, SettingActiveWatchlist, nid)
		}
		if settings.Get(s.d.Settings, SettingActiveWatchlist) == id {
			if idx >= len(remaining) {
				idx = len(remaining) - 1
			}
			return settings.Set(ctx, s.d.Settings, SettingActiveWatchlist, remaining[idx].ID)
		}
		return nil
	})
}

func (s *Service) SetActiveWatchlist(ctx context.Context, id string) (WatchlistsState, error) {
	return s.mutate(ctx, func(rows []store.WatchlistRow) error {
		if findList(rows, id) < 0 {
			return notFound()
		}
		return settings.Set(ctx, s.d.Settings, SettingActiveWatchlist, id)
	})
}

func (s *Service) AddToWatchlist(ctx context.Context, id string, token uint32, position int) (WatchlistsState, error) {
	return s.mutate(ctx, func(rows []store.WatchlistRow) error {
		idx := findList(rows, id)
		if idx < 0 {
			return notFound()
		}
		if _, ok := s.d.Instruments.ByToken(token); !ok {
			return refused(wlStage, "unknown_instrument", "That instrument is not in the instrument list.")
		}
		_, err := s.d.Watchlists.AddItem(ctx, id, token, position)
		if errors.Is(err, store.ErrDuplicate) {
			return refused(wlStage, "already_in_list", "Already in "+rows[idx].Name)
		}
		return err
	})
}

func (s *Service) RemoveFromWatchlist(ctx context.Context, id string, token uint32) (RemovedItem, error) {
	var removed RemovedItem
	_, err := s.mutate(ctx, func(rows []store.WatchlistRow) error {
		if findList(rows, id) < 0 {
			return notFound()
		}
		pos, err := s.d.Watchlists.RemoveItem(ctx, id, token)
		if errors.Is(err, store.ErrNoRow) {
			return refused(wlStage, "not_found", "That instrument is not in the list.")
		}
		removed = RemovedItem{ListID: id, Token: token, Position: pos}
		return err
	})
	return removed, err
}

func (s *Service) ReorderWatchlist(ctx context.Context, id string, tokens []uint32) (WatchlistsState, error) {
	return s.mutate(ctx, func(rows []store.WatchlistRow) error {
		idx := findList(rows, id)
		if idx < 0 {
			return notFound()
		}
		cur := map[uint32]bool{}
		for _, it := range rows[idx].Items {
			cur[it.Token] = true
		}
		seen := map[uint32]bool{}
		for _, t := range tokens {
			if !cur[t] || seen[t] {
				return refused(wlStage, "stale_order", "The list changed; reload and try again.")
			}
			seen[t] = true
		}
		if len(seen) != len(cur) {
			return refused(wlStage, "stale_order", "The list changed; reload and try again.")
		}
		return s.d.Watchlists.ReorderItems(ctx, id, tokens)
	})
}

func (s *Service) ReorderWatchlists(ctx context.Context, ids []string) (WatchlistsState, error) {
	return s.mutate(ctx, func(rows []store.WatchlistRow) error {
		seen := map[string]bool{}
		for _, id := range ids {
			if findList(rows, id) < 0 || seen[id] {
				return refused(wlStage, "stale_order", "The lists changed; reload and try again.")
			}
			seen[id] = true
		}
		if len(seen) != len(rows) {
			return refused(wlStage, "stale_order", "The lists changed; reload and try again.")
		}
		return s.d.Watchlists.ReorderLists(ctx, ids, s.nowMs())
	})
}
