> **P0-3 Session & Service Layer Implementation Plan — Task 15.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 15: Watchlist façade methods

**Files:**
- Create: `internal/service/watchlists.go`
- Test: `internal/service/watchlists_test.go`

**Interfaces:**
- Consumes: Task 13; `InstrumentFinder.ByToken`; `settings.Register` (Task 2), `settings.Get`, `settings.Set(ctx, …)` (P0-1).
- Produces:
  ```go
  var SettingActiveWatchlist = settings.Register(settings.Key[string]{Name: "watchlist.active", Default: ""})
  var TopicWatchlists bus.Topic[WatchlistsState]  // "watchlists.changed"
  type WatchlistItem struct{ Token uint32; Exchange, Symbol, DisplayName, Name string; IsIndex, Missing bool }
  type Watchlist struct{ ID, Name string; Items []WatchlistItem }
  type WatchlistsState struct{ Lists []Watchlist; ActiveID string }
  type RemovedItem struct{ ListID string; Token uint32; Position int }
  // methods: Watchlists, CreateWatchlist, RenameWatchlist, DeleteWatchlist, SetActiveWatchlist, AddToWatchlist,
  //          RemoveFromWatchlist, ReorderWatchlist, ReorderWatchlists (signatures in "Produces (for P0-4 / P0-5)")
  ```

Rules from FR-WATCH-001…005:
- A fresh install has one empty list, "Watchlist 1".
- Names are trimmed, 1–40 characters, and unique case-insensitively ("A watchlist with this name exists").
- A blank name on create becomes "Watchlist N" with the lowest unused N.
- Deleting the last list leaves a new empty "Watchlist 1". Deleting the active list activates the adjacent one.
- There is no item cap. A duplicate add returns the code `already_in_list` with "Already in <name>".
- Remove returns the old position so "Undo" can re-add at it.
- The active list persists in settings.

- [ ] **Step 1: Write the failing tests**

```go
// internal/service/watchlists_test.go
package service

import (
	"context"
	"testing"

	"tradeloom/internal/instruments"
	"tradeloom/internal/settings"
)

func names(st WatchlistsState) []string {
	var out []string
	for _, l := range st.Lists {
		out = append(out, l.Name)
	}
	return out
}

func seedInstruments(e *svcEnv) {
	e.instr.m[779521] = instruments.Instrument{Token: 779521, Exchange: "NSE", Tradingsymbol: "SBIN", Name: "STATE BANK OF INDIA", DisplayName: "SBIN"}
	e.instr.m[2953217] = instruments.Instrument{Token: 2953217, Exchange: "NSE", Tradingsymbol: "TCS", Name: "TATA CONSULTANCY SERV", DisplayName: "TCS"}
	e.instr.m[256265] = instruments.Instrument{Token: 256265, Exchange: "NSE", Tradingsymbol: "NIFTY 50", DisplayName: "NIFTY 50", IsIndex: true}
}

// FR-WATCH-001 AC1 and AC2 (data).
func TestWatchlists_DefaultListAndActivePersists(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	st, err := e.svc.Watchlists(ctx)
	if err != nil || len(st.Lists) != 1 || st.Lists[0].Name != "Watchlist 1" || len(st.Lists[0].Items) != 0 || st.ActiveID != st.Lists[0].ID {
		t.Fatalf("fresh %+v %v", st, err)
	}
	e.svc.CreateWatchlist(ctx, "")
	st, _ = e.svc.CreateWatchlist(ctx, "")
	second := st.Lists[1].ID
	if _, err := e.svc.SetActiveWatchlist(ctx, second); err != nil {
		t.Fatal(err)
	}
	if settings.Get(e.settings, SettingActiveWatchlist) != second {
		t.Fatal("active list not persisted")
	}
	if st, _ = e.svc.Watchlists(ctx); st.ActiveID != second {
		t.Fatalf("active after reload %s", st.ActiveID)
	}
}

// FR-WATCH-002 AC1–3.
func TestWatchlists_CreateAndRenameRules(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	e.svc.Watchlists(ctx)
	e.svc.CreateWatchlist(ctx, "")
	st, err := e.svc.CreateWatchlist(ctx, "   ")
	if err != nil || names(st)[2] != "Watchlist 3" || st.ActiveID != st.Lists[2].ID {
		t.Fatalf("%v %v", names(st), err)
	}
	if _, err := e.svc.RenameWatchlist(ctx, st.Lists[2].ID, "watchlist 1"); refusedCode(t, err) != "duplicate_name" {
		t.Fatal(err)
	}
	if _, err := e.svc.RenameWatchlist(ctx, st.Lists[2].ID, "  "); refusedCode(t, err) != "invalid_name" {
		t.Fatal(err)
	}
	if _, err := e.svc.RenameWatchlist(ctx, st.Lists[2].ID, "12345678901234567890123456789012345678901"); refusedCode(t, err) != "invalid_name" {
		t.Fatal(err)
	}
	if st, err = e.svc.RenameWatchlist(ctx, st.Lists[2].ID, "  Banks  "); err != nil || names(st)[2] != "Banks" {
		t.Fatalf("%v %v", names(st), err)
	}
}

// FR-WATCH-003 AC2.
func TestWatchlists_DeleteLastLeavesFreshList(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	st, _ := e.svc.Watchlists(ctx)
	st, err := e.svc.DeleteWatchlist(ctx, st.Lists[0].ID)
	if err != nil || len(st.Lists) != 1 || st.Lists[0].Name != "Watchlist 1" || st.ActiveID != st.Lists[0].ID {
		t.Fatalf("%+v %v", st, err)
	}
}

// FR-WATCH-004 AC1/AC2 and FR-WATCH-005 AC3 (data).
func TestWatchlists_AddDuplicateRemoveUndo(t *testing.T) {
	e := newSvcEnv(t)
	seedInstruments(e)
	ctx := context.Background()
	st, _ := e.svc.Watchlists(ctx)
	id := st.Lists[0].ID
	for _, tok := range []uint32{779521, 2953217, 256265} {
		if _, err := e.svc.AddToWatchlist(ctx, id, tok, -1); err != nil {
			t.Fatal(err)
		}
	}
	_, err := e.svc.AddToWatchlist(ctx, id, 779521, -1)
	if r, ok := err.(*ErrRefused); !ok || r.Code != "already_in_list" || r.Reason != "Already in Watchlist 1" {
		t.Fatalf("dup %v", err)
	}
	if _, err := e.svc.AddToWatchlist(ctx, id, 999, -1); refusedCode(t, err) != "unknown_instrument" {
		t.Fatal(err)
	}
	rm, err := e.svc.RemoveFromWatchlist(ctx, id, 2953217)
	if err != nil || rm.Position != 1 {
		t.Fatalf("%+v %v", rm, err)
	}
	st, err = e.svc.AddToWatchlist(ctx, rm.ListID, rm.Token, rm.Position)
	if err != nil || st.Lists[0].Items[1].Symbol != "TCS" || !st.Lists[0].Items[2].IsIndex {
		t.Fatalf("undo %+v %v", st.Lists[0].Items, err)
	}
	if _, err := e.svc.ReorderWatchlist(ctx, id, []uint32{256265, 779521}); refusedCode(t, err) != "stale_order" {
		t.Fatal(err)
	}
	if st, err = e.svc.ReorderWatchlist(ctx, id, []uint32{256265, 779521, 2953217}); err != nil || st.Lists[0].Items[0].Token != 256265 {
		t.Fatalf("%+v %v", st, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run TestWatchlists -v`
Expected: FAIL to compile with `undefined: (*Service).Watchlists`.

- [ ] **Step 3: Write the implementation**

```go
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
		return WatchlistsState{}, asRefused(wlStage, err)
	}
	if len(rows) == 0 {
		if err := s.d.Watchlists.Create(ctx, s.d.NewID(), "Watchlist 1", s.nowMs()); err != nil {
			return WatchlistsState{}, asRefused(wlStage, err)
		}
		if rows, err = s.d.Watchlists.List(ctx); err != nil {
			return WatchlistsState{}, asRefused(wlStage, err)
		}
	}
	active := settings.Get(s.d.Settings, SettingActiveWatchlist)
	if findList(rows, active) < 0 {
		active = rows[0].ID
		if err := settings.Set(ctx, s.d.Settings, SettingActiveWatchlist, active); err != nil {
			return WatchlistsState{}, asRefused(wlStage, err)
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
		return WatchlistsState{}, asRefused(wlStage, err)
	}
	if err := fn(rows); err != nil {
		return WatchlistsState{}, asRefused(wlStage, err)
	}
	st, err := s.watchlistsLocked(ctx)
	if err == nil {
		bus.Publish(s.d.Bus, TopicWatchlists, st)
	}
	return st, err
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
	_, err := s.mutate(ctx, func([]store.WatchlistRow) error {
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -run TestWatchlists -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/service/watchlists.go internal/service/watchlists_test.go
git commit -m "feat(service): watchlist rules, default list, active list and undo position

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
