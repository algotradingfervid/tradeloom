// internal/service/shortcuts_test.go
package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func withUiStore(t *testing.T, e *svcEnv) *store.DB {
	t.Helper()
	db := storetest.Open(t)
	e.svc.d.Shortcuts = store.NewShortcutsRepo(db)
	e.svc.d.UiState = store.NewWindowStateRepo(db)
	return db
}

func actByID(t *testing.T, acts []settings.ShortcutAction, id string) settings.ShortcutAction {
	t.Helper()
	for _, a := range acts {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("action %s not in %+v", id, acts)
	return settings.ShortcutAction{}
}

// P0-4 A8: defaults, rebind with the conflict and invalid refusals, persistence, reset.
func TestShortcuts_RebindConflictAndReset(t *testing.T) {
	e := newSvcEnv(t)
	db := withUiStore(t, e)
	ctx := context.Background()
	acts, err := e.svc.Shortcuts(ctx)
	if err != nil || len(acts) != len(settings.DefaultShortcuts()) || actByID(t, acts, "search.open").Binding != "Ctrl+K" {
		t.Fatalf("%+v %v", acts, err)
	}
	if _, err := e.svc.Rebind(ctx, "search.open", "Alt+1"); refusedCode(t, err) != "conflict" || err.Error() != "Go to Home" {
		t.Fatalf("%v", err)
	}
	if _, err := e.svc.Rebind(ctx, "search.open", "Ctrl+"); refusedCode(t, err) != "invalid" {
		t.Fatal(err)
	}
	if _, err := e.svc.Rebind(ctx, "nope", "Ctrl+P"); refusedCode(t, err) != "unknown_action" {
		t.Fatal(err)
	}
	a, err := e.svc.Rebind(ctx, "search.open", "ctrl+p")
	if err != nil || a.ID != "search.open" || a.Binding != "Ctrl+P" || a.Default != "Ctrl+K" {
		t.Fatalf("%+v %v", a, err)
	}
	// a second service over the same database sees the rebind; the conflict check uses the stored binding
	e2 := newSvcEnv(t)
	e2.svc.d.Shortcuts = store.NewShortcutsRepo(db)
	acts, _ = e2.svc.Shortcuts(ctx)
	if actByID(t, acts, "search.open").Binding != "Ctrl+P" {
		t.Fatalf("not persisted")
	}
	if _, err := e2.svc.Rebind(ctx, "help.open", "Ctrl+P"); refusedCode(t, err) != "conflict" || err.Error() != "Open search" {
		t.Fatalf("%v", err)
	}
	if _, err := e2.svc.Rebind(ctx, "help.open", "Ctrl+K"); err != nil { // Ctrl+K is free again
		t.Fatal(err)
	}
	acts, err = e2.svc.ResetShortcuts(ctx)
	if err != nil || actByID(t, acts, "search.open").Binding != "Ctrl+K" || actByID(t, acts, "help.open").Binding != "F1" {
		t.Fatalf("%+v %v", acts, err)
	}
	if rows, _ := store.NewShortcutsRepo(db).Load(ctx); len(rows) != 0 {
		t.Fatalf("rows after reset %+v", rows)
	}
}

// Ruling 64 (1): the shipped Alt+T overlap never blocks restoring a default; (11) rebind to the current binding.
func TestShortcuts_RestoreOwnDefaultAndNoOpRebind(t *testing.T) {
	e := newSvcEnv(t)
	withUiStore(t, e)
	ctx := context.Background()
	if a, err := e.svc.Rebind(ctx, "chart.trendLine", "Alt+T"); err != nil || a.Binding != "Alt+T" {
		t.Fatalf("default overlap refused: %+v %v", a, err)
	}
	if _, err := e.svc.Rebind(ctx, "chart.trendLine", "Alt+Y"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Rebind(ctx, "order.new", "Alt+T"); refusedCode(t, err) != "conflict" || err.Error() != "Focus toasts" { // global toasts.focus still holds Alt+T
		t.Fatal(err)
	}
	if a, err := e.svc.Rebind(ctx, "chart.trendLine", "Alt+T"); err != nil || a.Binding != "Alt+T" {
		t.Fatalf("round trip: %+v %v", a, err)
	}
	if a, err := e.svc.Rebind(ctx, "search.open", "Ctrl+K"); err != nil || a.Binding != "Ctrl+K" {
		t.Fatalf("rebind to current: %+v %v", a, err)
	}
	if a, err := e.svc.Rebind(ctx, "toasts.focus", "Alt+T"); err != nil || a.Binding != "Alt+T" {
		t.Fatalf("toasts default: %+v %v", a, err)
	}
}

func TestShortcuts_ViewScopeAndReservedAndKeys(t *testing.T) {
	e := newSvcEnv(t)
	withUiStore(t, e)
	ctx := context.Background()
	if _, err := e.svc.Rebind(ctx, "watchlist.moveUp", "Alt+1"); refusedCode(t, err) != "conflict" {
		t.Fatal(err)
	}
	if _, err := e.svc.Rebind(ctx, "watchlist.moveUp", "Alt+H"); err != nil { // view vs chart: no overlap
		t.Fatal(err)
	}
	for _, bad := range []string{"K", "Ctrl+C", "Alt+F4", "F5", "F12", "Ctrl+Ö", "Ctrl+Foo"} {
		if _, err := e.svc.Rebind(ctx, "help.open", bad); refusedCode(t, err) != "invalid" {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	if _, err := e.svc.Rebind(ctx, "help.open", "Alt+Q"); err != nil { // Alt+letter is free on GTK
		t.Fatal(err)
	}
	if _, err := e.svc.Rebind(ctx, strings.Repeat("a", 65), "Alt+W"); refusedCode(t, err) != "invalid" {
		t.Fatal(err)
	}
	if _, err := e.svc.Rebind(ctx, "help.open", strings.Repeat("a", 65)); refusedCode(t, err) != "invalid" {
		t.Fatal(err)
	}
}

// FR-SHELL-019: Scalper actions ship unbound, can be bound, conflicts skip unbound actions.
func TestShortcuts_UnboundScalperActions(t *testing.T) {
	e := newSvcEnv(t)
	withUiStore(t, e)
	ctx := context.Background()
	acts, _ := e.svc.Shortcuts(ctx)
	for _, id := range []string{"scalper.buy", "scalper.sell", "scalper.exitAll", "scalper.cancelAll"} {
		if a := actByID(t, acts, id); a.Binding != "" || a.Default != "" {
			t.Fatalf("%+v", a)
		}
	}
	for _, id := range []string{"chart.horizontalLine", "chart.horizontalRay", "chart.verticalLine", "chart.crossLine"} {
		if a := actByID(t, acts, id); a.Scope != settings.ScopeChart || a.Binding == "" {
			t.Fatalf("%+v", a)
		}
	}
	if a, err := e.svc.Rebind(ctx, "scalper.buy", "Ctrl+B"); err != nil || a.Binding != "Ctrl+B" {
		t.Fatalf("%+v %v", a, err)
	}
	if a, err := e.svc.Rebind(ctx, "scalper.sell", "Ctrl+S"); err != nil || a.Binding != "Ctrl+S" { // unbound others never conflict
		t.Fatalf("%+v %v", a, err)
	}
}

func TestShortcuts_PublishesChangeEvents(t *testing.T) {
	e := newSvcEnv(t)
	withUiStore(t, e)
	ctx := context.Background()
	sub := bus.Subscribe(e.bus, settings.Topic, bus.SubOpts[settings.Changed]{Name: "t28", Buffer: 8})
	defer bus.Unsubscribe(e.bus, sub)
	next := func() settings.Changed {
		t.Helper()
		select {
		case c := <-sub.C():
			return c
		case <-time.After(3 * time.Second):
			t.Fatal("no change event")
		}
		return settings.Changed{}
	}
	if _, err := e.svc.Rebind(ctx, "search.open", "Ctrl+P"); err != nil {
		t.Fatal(err)
	}
	c := next()
	var acts []settings.ShortcutAction
	if c.Key != "shortcuts" || json.Unmarshal(c.Value, &acts) != nil || actByID(t, acts, "search.open").Binding != "Ctrl+P" {
		t.Fatalf("%+v", c)
	}
	if _, err := e.svc.Rebind(ctx, "search.open", "Alt+1"); err == nil { // refusals publish nothing
		t.Fatal("expected conflict")
	}
	if _, err := e.svc.ResetShortcuts(ctx); err != nil {
		t.Fatal(err)
	}
	c = next()
	if c.Key != "shortcuts" || json.Unmarshal(c.Value, &acts) != nil || actByID(t, acts, "search.open").Binding != "Ctrl+K" {
		t.Fatalf("%+v", c)
	}
	select {
	case extra := <-sub.C():
		t.Fatalf("unexpected extra event %+v", extra)
	default:
	}
}

func TestShortcuts_DropsInvalidStoredRows(t *testing.T) {
	e := newSvcEnv(t)
	db := withUiStore(t, e)
	ctx := context.Background()
	r := store.NewShortcutsRepo(db)
	for _, row := range [][2]string{{"ghost.action", "Ctrl+P"}, {"help.open", "Ctrl+"}, {"kill.switch", "Ctrl+C"}, {"search.open", "ctrl+p"}} {
		if err := r.Save(ctx, row[0], row[1], "global"); err != nil {
			t.Fatal(err)
		}
	}
	acts, err := e.svc.Shortcuts(ctx)
	if err != nil || len(acts) != len(settings.DefaultShortcuts()) {
		t.Fatalf("%d %v", len(acts), err)
	}
	if actByID(t, acts, "help.open").Binding != "F1" || actByID(t, acts, "kill.switch").Binding != "Ctrl+Shift+K" || actByID(t, acts, "search.open").Binding != "Ctrl+P" {
		t.Fatalf("%+v", acts)
	}
}

func TestShortcuts_ConcurrentRebindsAreSerialised(t *testing.T) {
	for round := 0; round < 5; round++ {
		e := newSvcEnv(t)
		withUiStore(t, e)
		ctx := context.Background()
		ids := []string{"nav.1", "nav.2", "nav.3", "nav.4", "nav.5", "nav.6", "nav.7", "nav.8"}
		var wg sync.WaitGroup
		var mu sync.Mutex
		ok := 0
		for _, id := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := e.svc.Rebind(ctx, id, "Alt+Z")
				mu.Lock()
				defer mu.Unlock()
				if err == nil {
					ok++
				} else if refusedCode(t, err) != "conflict" {
					t.Errorf("%v", err)
				}
			}()
		}
		wg.Wait()
		if ok != 1 {
			t.Fatalf("round %d: %d rebinds to the same binding succeeded, want exactly 1", round, ok)
		}
	}
}

func TestShortcuts_StorageFailureAndUnavailable(t *testing.T) {
	e := newSvcEnv(t)
	db := withUiStore(t, e)
	ctx := context.Background()
	if _, err := db.W.ExecContext(ctx, `CREATE TRIGGER fail_save BEFORE INSERT ON shortcuts BEGIN SELECT RAISE(ABORT, 'disk full'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Rebind(ctx, "search.open", "Ctrl+P"); refusedCode(t, err) != "internal" {
		t.Fatal(err)
	}
	acts, _ := e.svc.Shortcuts(ctx)
	if actByID(t, acts, "search.open").Binding != "Ctrl+K" {
		t.Fatal("failed save must not change the binding")
	}

	e2 := newSvcEnv(t) // no repositories wired
	if _, err := e2.svc.Shortcuts(ctx); refusedCode(t, err) != "unavailable" {
		t.Fatal(err)
	}
	if _, err := e2.svc.Rebind(ctx, "search.open", "Ctrl+P"); refusedCode(t, err) != "unavailable" {
		t.Fatal(err)
	}
	if _, err := e2.svc.ResetShortcuts(ctx); refusedCode(t, err) != "unavailable" {
		t.Fatal(err)
	}
	if _, err := e2.svc.GetUiState(ctx, "chart.dashboard"); refusedCode(t, err) != "unavailable" {
		t.Fatal(err)
	}
	if err := e2.svc.SetUiState(ctx, "chart.dashboard", "x"); refusedCode(t, err) != "unavailable" {
		t.Fatal(err)
	}
}

func TestUiState_GetSet(t *testing.T) {
	e := newSvcEnv(t)
	withUiStore(t, e)
	ctx := context.Background()
	if v, err := e.svc.GetUiState(ctx, "chart.dashboard"); err != nil || v != "" {
		t.Fatalf("%q %v", v, err)
	}
	if err := e.svc.SetUiState(ctx, "chart.dashboard", `{"identifier":"NSE:NIFTY 50","timeframe":"60minute"}`); err != nil {
		t.Fatal(err)
	}
	if v, _ := e.svc.GetUiState(ctx, "chart.dashboard"); v != `{"identifier":"NSE:NIFTY 50","timeframe":"60minute"}` {
		t.Fatalf("%q", v)
	}
	if _, err := e.svc.GetUiState(ctx, " "); refusedCode(t, err) != "invalid_key" {
		t.Fatal(err)
	}
	if err := e.svc.SetUiState(ctx, "", "x"); refusedCode(t, err) != "invalid_key" {
		t.Fatal(err)
	}
}

func TestUiState_LimitsAndProtectedKeys(t *testing.T) {
	e := newSvcEnv(t)
	db := withUiStore(t, e)
	ctx := context.Background()
	for _, k := range []string{strings.Repeat("a", 65), "Upper", "has space", "a/b", "ünï"} {
		if err := e.svc.SetUiState(ctx, k, "x"); refusedCode(t, err) != "invalid_key" {
			t.Fatalf("%q: %v", k, err)
		}
		if _, err := e.svc.GetUiState(ctx, k); refusedCode(t, err) != "invalid_key" {
			t.Fatalf("get %q: %v", k, err)
		}
	}
	if err := e.svc.SetUiState(ctx, strings.Repeat("a", 64), "x"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"quotes.last", "window.geometry", "backup.scheduled.last"} {
		if err := e.svc.SetUiState(ctx, k, "x"); refusedCode(t, err) != "invalid_key" {
			t.Fatalf("%s writable: %v", k, err)
		}
	}
	// protected keys stay readable (the shell restores geometry through the backend, the UI may read it)
	if err := store.NewWindowStateRepo(db).Set(ctx, "window.geometry", "{}"); err != nil {
		t.Fatal(err)
	}
	if v, err := e.svc.GetUiState(ctx, "window.geometry"); err != nil || v != "{}" {
		t.Fatalf("%q %v", v, err)
	}
	if err := e.svc.SetUiState(ctx, "chart.big", strings.Repeat("x", 64*1024+1)); refusedCode(t, err) != "too_large" {
		t.Fatal(err)
	}
	if err := e.svc.SetUiState(ctx, "chart.big", strings.Repeat("x", 64*1024)); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SetUiState(ctx, "chart.bad", "\xff\xfe"); refusedCode(t, err) != "invalid_value" {
		t.Fatal(err)
	}
}

// Task 22: window.geometry is backend-owned (Ruling 64) — internal/app saves it before close through
// SaveWindowGeometry (never bound); the value obeys the same size and UTF-8 limits.
func TestSaveWindowGeometry(t *testing.T) {
	e := newSvcEnv(t)
	withUiStore(t, e)
	ctx := context.Background()
	g := `{"w":1600,"h":1000,"x":5,"y":6,"maximised":true}`
	if err := e.svc.SaveWindowGeometry(ctx, g); err != nil {
		t.Fatal(err)
	}
	if v, err := e.svc.GetUiState(ctx, UiStateWindowGeometry); err != nil || v != g {
		t.Fatalf("%q %v", v, err)
	}
	if err := e.svc.SaveWindowGeometry(ctx, strings.Repeat("x", 64*1024+1)); refusedCode(t, err) != "too_large" {
		t.Fatal(err)
	}
	if err := e.svc.SaveWindowGeometry(ctx, "\xff"); refusedCode(t, err) != "invalid_value" {
		t.Fatal(err)
	}
	e2 := newSvcEnv(t)
	if err := e2.svc.SaveWindowGeometry(ctx, g); refusedCode(t, err) != "unavailable" {
		t.Fatal(err)
	}
}

// Ruling 82 (M2): SaveWindowGeometry accepts only the geometry JSON shape.
func TestSaveWindowGeometry_Shape(t *testing.T) {
	e := newSvcEnv(t)
	withUiStore(t, e)
	ctx := context.Background()
	for _, bad := range []string{"", "x", "[]", "null", `{"foo":1}`, `{"w":0,"h":10,"x":0,"y":0,"maximised":false}`,
		`{"w":10,"h":-1,"x":0,"y":0,"maximised":false}`, `{"w":"10","h":10}`, `{"w":10,"h":10} {}`} {
		if err := e.svc.SaveWindowGeometry(ctx, bad); refusedCode(t, err) != "invalid_value" {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if err := e.svc.SaveWindowGeometry(ctx, `{"w":1280,"h":720,"x":-5,"y":0,"maximised":true}`); err != nil {
		t.Fatal(err)
	}
}
