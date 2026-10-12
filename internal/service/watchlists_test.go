// internal/service/watchlists_test.go
package service

import (
	"context"
	"testing"

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
