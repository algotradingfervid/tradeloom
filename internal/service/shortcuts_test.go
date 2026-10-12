// internal/service/shortcuts_test.go
package service

import (
	"context"
	"testing"

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

// P0-4 A8: defaults, rebind with the conflict and invalid refusals, persistence, reset.
func TestShortcuts_RebindConflictAndReset(t *testing.T) {
	e := newSvcEnv(t)
	db := withUiStore(t, e)
	ctx := context.Background()
	acts, err := e.svc.Shortcuts(ctx)
	if err != nil || len(acts) != len(settings.DefaultShortcuts()) || acts[9].ID != "search.open" || acts[9].Binding != "Ctrl+K" {
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
	if acts[9].Binding != "Ctrl+P" {
		t.Fatalf("not persisted: %+v", acts[9])
	}
	if _, err := e2.svc.Rebind(ctx, "help.open", "Ctrl+P"); refusedCode(t, err) != "conflict" || err.Error() != "Open search" {
		t.Fatalf("%v", err)
	}
	if _, err := e2.svc.Rebind(ctx, "help.open", "Ctrl+K"); err != nil { // Ctrl+K is free again
		t.Fatal(err)
	}
	acts, err = e2.svc.ResetShortcuts(ctx)
	if err != nil || acts[9].Binding != "Ctrl+K" || acts[10].Binding != "F1" {
		t.Fatalf("%+v %v", acts, err)
	}
	if rows, _ := store.NewShortcutsRepo(db).Load(ctx); len(rows) != 0 {
		t.Fatalf("rows after reset %+v", rows)
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
