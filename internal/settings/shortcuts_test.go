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
		"chart.horizontalLine": {"Alt+H", ScopeChart}, "chart.horizontalRay": {"Alt+J", ScopeChart},
		"chart.verticalLine": {"Alt+V", ScopeChart}, "chart.crossLine": {"Alt+C", ScopeChart},
		"scalper.buy": {"", ScopeView}, "scalper.sell": {"", ScopeView}, "scalper.exitAll": {"", ScopeView}, "scalper.cancelAll": {"", ScopeView},
	}
	if len(acts) != len(want) {
		t.Fatalf("%d actions, want %d", len(acts), len(want))
	}
	for _, a := range acts {
		w, ok := want[a.ID]
		if !ok || a.Default != w[0] || a.Binding != w[0] || a.Scope != w[1] || a.Label == "" {
			t.Fatalf("%+v", a)
		}
		if a.Default == "" {
			continue // unbound by default (FR-SCALP-013)
		}
		if got, err := ValidateBinding(a.Scope, a.Default); err != nil || got != a.Default {
			t.Fatalf("%s default %q: %q %v", a.ID, a.Default, got, err)
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
	for _, bad := range []string{"", "Ctrl", "Ctrl+", "+K", "Ctrl+Ctrl+K", "K+Ctrl", "Ctrl+K+L", "Ctrl + ", "Ctrl+Ö", "Ctrl+\xff", "Ctrl+F25", "Ctrl+F0", "Ctrl+Foo", "Ctrl+ab", "Ctrl+Ctrl"} {
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

func TestValidateBinding_ScopeAndReserved(t *testing.T) {
	ok := []struct{ scope, in, want string }{
		{ScopeGlobal, "alt+x", "Alt+X"}, {ScopeGlobal, "F9", "F9"}, {ScopeView, "Ctrl+Shift+Y", "Ctrl+Shift+Y"},
		{ScopeChart, "T", "T"}, {ScopeChart, "Shift+T", "Shift+T"}, {ScopeBuilder, "Delete", "Delete"},
	}
	for _, c := range ok {
		if got, err := ValidateBinding(c.scope, c.in); err != nil || got != c.want {
			t.Fatalf("%s %q: %q %v", c.scope, c.in, got, err)
		}
	}
	bad := []struct{ scope, in string }{
		{ScopeGlobal, "K"}, {ScopeGlobal, "Shift+K"}, {ScopeView, "ArrowUp"}, {ScopeGlobal, "Space"},
		{ScopeChart, "Ctrl+C"}, {ScopeGlobal, "ctrl+v"}, {ScopeBuilder, "Ctrl+X"}, {ScopeGlobal, "Ctrl+A"}, {ScopeGlobal, "Ctrl+Q"},
		{ScopeGlobal, "Ctrl+W"}, {ScopeGlobal, "Alt+F4"}, {ScopeChart, "F5"}, {ScopeGlobal, "F12"}, {"nowhere", "Ctrl+K"},
	}
	for _, c := range bad {
		if got, err := ValidateBinding(c.scope, c.in); err == nil {
			t.Fatalf("%s %q accepted as %q", c.scope, c.in, got)
		}
	}
}

func TestConflicts_ViewScopeAndShippedOverlap(t *testing.T) {
	acts := DefaultShortcuts()
	if other, ok := Conflicts(acts, "watchlist.moveUp", "Alt+1"); !ok || other.ID != "nav.1" {
		t.Fatalf("view vs global: %+v %v", other, ok)
	}
	if other, ok := Conflicts(acts, "watchlist.moveUp", "Shift+T"); ok {
		t.Fatalf("view vs chart must not conflict: %+v", other)
	}
	if other, ok := Conflicts(acts, "scalper.buy", "Alt+ArrowDown"); !ok || other.ID != "watchlist.moveDown" {
		t.Fatalf("view vs view: %+v %v", other, ok)
	}
	// restoring an action to its own default never trips over a shipped default overlap
	if other, ok := Conflicts(acts, "chart.trendLine", "Alt+T"); ok {
		t.Fatalf("shipped overlap reported: %+v", other)
	}
	if other, ok := Conflicts(acts, "toasts.focus", "Alt+T"); ok {
		t.Fatalf("shipped overlap reported: %+v", other)
	}
	// but a real conflict with a rebound action is still reported
	for i := range acts {
		if acts[i].ID == "order.new" {
			acts[i].Binding = "Alt+T"
		}
		if acts[i].ID == "chart.trendLine" {
			acts[i].Binding = "Alt+Y"
		}
	}
	if other, ok := Conflicts(acts, "chart.trendLine", "Alt+T"); !ok || other.ID != "order.new" {
		t.Fatalf("real conflict missed: %+v %v", other, ok)
	}
	if _, ok := Conflicts(acts, "nope", "Ctrl+P"); ok {
		t.Fatal("unknown action reported a conflict")
	}
	if _, ok := Conflicts(DefaultShortcuts(), "scalper.sell", ""); ok {
		t.Fatal("empty binding must never conflict (unbound actions)")
	}
}
