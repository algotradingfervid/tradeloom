// internal/service/watchlists_test.go
package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/clock"
	"tradeloom/internal/instruments"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
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

// Ruling 42: a store-level reorder conflict surfaces as stale_order.
func TestWatchlists_StoreConflictIsStaleOrder(t *testing.T) {
	if refusedCode(t, s2(mapStoreErr(store.ErrConflict))) != "stale_order" {
		t.Fatal("conflict not mapped")
	}
	if refusedCode(t, s2(mapStoreErr(store.ErrNoRow))) != "not_found" {
		t.Fatal("no-row not mapped")
	}
}

func s2(err error) error { return asRefused(wlStage, err) }

// hookClock runs hook once, on the first Now call, before delegating. The façade calls nowMs() while
// building the arguments of a store write, so the hook simulates another writer changing the DB between
// the façade's read and the store call.
type hookClock struct {
	clock.Clock
	hook func()
}

func (h *hookClock) Now() time.Time {
	if h.hook != nil {
		f := h.hook
		h.hook = nil
		f()
	}
	return h.Clock.Now()
}

func armHook(e *svcEnv, f func()) {
	e.svc.d.Clock = &hookClock{Clock: e.clock, hook: f}
}

// Ruling 42 wiring: mutate must route store sentinels through mapStoreErr.
func TestWatchlists_MutateMapsStoreSentinels(t *testing.T) {
	ctx := context.Background()

	t.Run("ErrNoRow from Rename is not_found", func(t *testing.T) {
		e := newSvcEnv(t)
		st, _ := e.svc.CreateWatchlist(ctx, "A")
		id := st.Lists[len(st.Lists)-1].ID
		armHook(e, func() {
			if err := e.svc.d.Watchlists.SoftDelete(ctx, id, 1); err != nil {
				t.Error(err)
			}
		})
		if _, err := e.svc.RenameWatchlist(ctx, id, "B"); refusedCode(t, err) != "not_found" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("ErrNoRow from SoftDelete is not_found", func(t *testing.T) {
		e := newSvcEnv(t)
		st, _ := e.svc.CreateWatchlist(ctx, "A")
		id := st.Lists[len(st.Lists)-1].ID
		armHook(e, func() { _ = e.svc.d.Watchlists.SoftDelete(ctx, id, 1) })
		if _, err := e.svc.DeleteWatchlist(ctx, id); refusedCode(t, err) != "not_found" {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("ErrConflict from ReorderLists is stale_order", func(t *testing.T) {
		e := newSvcEnv(t)
		e.svc.CreateWatchlist(ctx, "A")
		st, _ := e.svc.CreateWatchlist(ctx, "B")
		var ids []string
		for _, l := range st.Lists {
			ids = append(ids, l.ID)
		}
		armHook(e, func() { _ = e.svc.d.Watchlists.SoftDelete(ctx, ids[2], 1) })
		if _, err := e.svc.ReorderWatchlists(ctx, []string{ids[2], ids[1], ids[0]}); refusedCode(t, err) != "stale_order" {
			t.Fatalf("got %v", err)
		}
	})
}

// Same-name creates racing: the lock serialises them, so exactly one wins.
func TestWatchlists_ConcurrentCreateSameName(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	e.svc.Watchlists(ctx)
	const n = 16
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = e.svc.CreateWatchlist(ctx, "Banks")
		}(i)
	}
	close(start)
	wg.Wait()
	ok, dup := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case refusedCode(t, err) == "duplicate_name":
			dup++
		}
	}
	if ok != 1 || dup != n-1 {
		t.Fatalf("ok=%d dup=%d", ok, dup)
	}
	st, _ := e.svc.Watchlists(ctx)
	if len(st.Lists) != 2 {
		t.Fatalf("lists %v", names(st))
	}
}

func TestWatchlists_RemoveFromDeletedOrUnknownListIsNotFound(t *testing.T) {
	e := newSvcEnv(t)
	seedInstruments(e)
	ctx := context.Background()
	st, _ := e.svc.CreateWatchlist(ctx, "A")
	id := st.Lists[len(st.Lists)-1].ID
	if _, err := e.svc.AddToWatchlist(ctx, id, 779521, -1); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.DeleteWatchlist(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.RemoveFromWatchlist(ctx, id, 779521); refusedCode(t, err) != "not_found" {
		t.Fatalf("deleted: %v", err)
	}
	if _, err := e.svc.RemoveFromWatchlist(ctx, "nope", 779521); refusedCode(t, err) != "not_found" {
		t.Fatalf("unknown: %v", err)
	}
	// An item that is not in a live list stays not_found too.
	st, _ = e.svc.Watchlists(ctx)
	if _, err := e.svc.RemoveFromWatchlist(ctx, st.Lists[0].ID, 779521); refusedCode(t, err) != "not_found" {
		t.Fatalf("missing item: %v", err)
	}
}

func TestWatchlists_DeleteActiveActivatesAdjacent(t *testing.T) {
	ctx := context.Background()
	setup := func() (*svcEnv, []string) {
		e := newSvcEnv(t)
		e.svc.Watchlists(ctx)
		e.svc.CreateWatchlist(ctx, "B")
		st, _ := e.svc.CreateWatchlist(ctx, "C")
		var ids []string
		for _, l := range st.Lists {
			ids = append(ids, l.ID)
		}
		return e, ids
	}
	// Middle: the list that slides into its slot (the next one) becomes active.
	e, ids := setup()
	e.svc.SetActiveWatchlist(ctx, ids[1])
	st, err := e.svc.DeleteWatchlist(ctx, ids[1])
	if err != nil || st.ActiveID != ids[2] || len(st.Lists) != 2 {
		t.Fatalf("middle %+v %v", st, err)
	}
	// Last: falls back to the previous one.
	e, ids = setup()
	st, err = e.svc.DeleteWatchlist(ctx, ids[2]) // C is active after create
	if err != nil || st.ActiveID != ids[1] {
		t.Fatalf("last %+v %v", st, err)
	}
	// Non-active delete leaves the active list alone.
	e, ids = setup()
	st, err = e.svc.DeleteWatchlist(ctx, ids[0])
	if err != nil || st.ActiveID != ids[2] {
		t.Fatalf("other %+v %v", st, err)
	}
}

func TestWatchlists_ReorderLists(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	e.svc.Watchlists(ctx)
	e.svc.CreateWatchlist(ctx, "B")
	st, _ := e.svc.CreateWatchlist(ctx, "C")
	a, b, c := st.Lists[0].ID, st.Lists[1].ID, st.Lists[2].ID
	st, err := e.svc.ReorderWatchlists(ctx, []string{c, a, b})
	if err != nil || strings.Join(names(st), ",") != "C,Watchlist 1,B" {
		t.Fatalf("%v %v", names(st), err)
	}
	if st, _ = e.svc.Watchlists(ctx); strings.Join(names(st), ",") != "C,Watchlist 1,B" {
		t.Fatalf("not persisted %v", names(st))
	}
	for name, ids := range map[string][]string{
		"missing":   {c, a},
		"duplicate": {c, c, a},
		"unknown":   {c, a, "zzz"},
		"extra":     {c, a, b, "zzz"},
	} {
		if _, err := e.svc.ReorderWatchlists(ctx, ids); refusedCode(t, err) != "stale_order" {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestWatchlists_NameLengthBoundaryCountsRunes(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	e.svc.Watchlists(ctx)
	ascii40 := strings.Repeat("a", 40)
	multi40 := strings.Repeat("日", 40) // 40 runes, 120 bytes
	for _, ok := range []string{ascii40, multi40} {
		if st, err := e.svc.CreateWatchlist(ctx, ok); err != nil || names(st)[len(st.Lists)-1] != ok {
			t.Fatalf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{ascii40 + "a", multi40 + "日"} {
		if _, err := e.svc.CreateWatchlist(ctx, bad); refusedCode(t, err) != "invalid_name" {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	// Surrounding whitespace is trimmed before counting.
	if _, err := e.svc.CreateWatchlist(ctx, "  "+strings.Repeat("b", 40)+"  "); err != nil {
		t.Fatal(err)
	}
}

func TestWatchlists_SixtyItems(t *testing.T) {
	e := newSvcEnv(t)
	ctx := context.Background()
	st, _ := e.svc.Watchlists(ctx)
	id := st.Lists[0].ID
	toks := make([]uint32, 60)
	for i := range toks {
		toks[i] = uint32(1000 + i)
		e.instr.m[toks[i]] = instruments.Instrument{Token: toks[i], Exchange: "NSE", Tradingsymbol: fmt.Sprintf("S%d", i)}
		if _, err := e.svc.AddToWatchlist(ctx, id, toks[i], -1); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	st, _ = e.svc.Watchlists(ctx)
	if len(st.Lists[0].Items) != 60 {
		t.Fatalf("items %d", len(st.Lists[0].Items))
	}
	for i, it := range st.Lists[0].Items {
		if it.Token != toks[i] {
			t.Fatalf("order at %d: %d", i, it.Token)
		}
	}
	rev := make([]uint32, 60)
	for i := range toks {
		rev[i] = toks[59-i]
	}
	st, err := e.svc.ReorderWatchlist(ctx, id, rev)
	if err != nil || st.Lists[0].Items[0].Token != toks[59] || st.Lists[0].Items[59].Token != toks[0] {
		t.Fatalf("reorder %v", err)
	}
	rm, err := e.svc.RemoveFromWatchlist(ctx, id, toks[30])
	if err != nil || rm.Position != 29 {
		t.Fatalf("remove %+v %v", rm, err)
	}
}
