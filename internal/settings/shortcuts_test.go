// internal/settings/shortcuts_test.go
package settings

import "testing"

func TestDefaultShortcuts_MatchContract(t *testing.T) {
	acts := DefaultShortcuts()
	want := map[string][2]string{
		"nav.1": {"Alt+1", ScopeGlobal}, "nav.2": {"Alt+2", ScopeGlobal}, "nav.3": {"Alt+3", ScopeGlobal}, "nav.4": {"Alt+4", ScopeGlobal},
		"nav.5": {"Alt+5", ScopeGlobal}, "nav.6": {"Alt+6", ScopeGlobal}, "nav.7": {"Alt+7", ScopeGlobal}, "nav.8": {"Alt+8", ScopeGlobal},
		"toasts.focus": {"Alt+T", ScopeGlobal}, "search.open": {"Ctrl+K", ScopeGlobal}, "help.open": {"F1", ScopeGlobal},
		"kill.switch": {"Ctrl+Shift+K", ScopeGlobal}, "chart.fullView": {"F11", ScopeGlobal}, "chart.trendLine": {"Alt+T", ScopeChart},
		"order.new": {"Shift+T", ScopeChart}, "builder.undo": {"Ctrl+Z", ScopeBuilder}, "builder.redo": {"Ctrl+Shift+Z", ScopeBuilder},
		"watchlist.moveUp": {"Alt+ArrowUp", ScopeView}, "watchlist.moveDown": {"Alt+ArrowDown", ScopeView},
	}
	if len(acts) != len(want) {
		t.Fatalf("%d actions, want %d", len(acts), len(want))
	}
	for _, a := range acts {
		w, ok := want[a.ID]
		if !ok || a.Default != w[0] || a.Binding != w[0] || a.Scope != w[1] || a.Label == "" {
			t.Fatalf("%+v", a)
		}
		if _, err := ParseBinding(a.Default); err != nil {
			t.Fatalf("%s default %q: %v", a.ID, a.Default, err)
		}
	}
	if acts[0].ID != "nav.1" || acts[len(acts)-1].ID != "watchlist.moveDown" {
		t.Fatalf("order %s … %s", acts[0].ID, acts[len(acts)-1].ID)
	}
	acts[0].Binding = "changed"
	if DefaultShortcuts()[0].Binding != "Alt+1" {
		t.Fatal("DefaultShortcuts must return a copy")
	}
}

func TestParseBinding(t *testing.T) {
	ok := map[string]string{
		"ctrl+k": "Ctrl+K", "Shift+Ctrl+Z": "Ctrl+Shift+Z", "F11": "F11", "alt+arrowup": "Alt+ArrowUp", " Meta + Escape ": "Meta+Escape", "Ctrl+Alt+Shift+Meta+Y": "Ctrl+Alt+Shift+Meta+Y",
	}
	for in, want := range ok {
		b, err := ParseBinding(in)
		if err != nil || b.String() != want {
			t.Fatalf("ParseBinding(%q) = %q, %v; want %q", in, b.String(), err, want)
		}
	}
	for _, bad := range []string{"", "Ctrl", "Ctrl+", "+K", "Ctrl+Ctrl+K", "K+Ctrl", "Ctrl+K+L", "Ctrl + "} {
		if _, err := ParseBinding(bad); err == nil {
			t.Fatalf("ParseBinding(%q) accepted", bad)
		}
	}
}

func TestConflicts(t *testing.T) {
	acts := DefaultShortcuts()
	if other, ok := Conflicts(acts, "search.open", "Alt+1"); !ok || other.ID != "nav.1" {
		t.Fatalf("global vs global: %+v %v", other, ok)
	}
	if other, ok := Conflicts(acts, "order.new", "Alt+T"); !ok || other.ID != "toasts.focus" {
		t.Fatalf("chart vs global: %+v %v", other, ok)
	}
	if other, ok := Conflicts(acts, "builder.undo", "Shift+T"); ok {
		t.Fatalf("builder vs chart must not conflict: %+v", other)
	}
	if other, ok := Conflicts(acts, "search.open", "Ctrl+K"); ok {
		t.Fatalf("an action never conflicts with itself: %+v", other)
	}
	if _, ok := Conflicts(acts, "search.open", "Ctrl+P"); ok {
		t.Fatal("free binding reported as a conflict")
	}
}
