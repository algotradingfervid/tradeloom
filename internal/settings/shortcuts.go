// internal/settings/shortcuts.go
package settings

import (
	"errors"
	"fmt"
	"strings"
)

// Shortcut scopes (P0-4 A8): global everywhere; view while a list view has focus; chart / builder inside those containers.
const (
	ScopeGlobal  = "global"
	ScopeView    = "view"
	ScopeChart   = "chart"
	ScopeBuilder = "builder"
)

// ShortcutAction is one rebindable action (TS settings.ShortcutAction).
type ShortcutAction struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Scope   string `json:"scope"`
	Default string `json:"defaultBinding"`
	Binding string `json:"binding"`
}

var defaultShortcuts = []ShortcutAction{
	{ID: "nav.1", Label: "Go to Home", Scope: ScopeGlobal, Default: "Alt+1"},
	{ID: "nav.2", Label: "Go to Scalper", Scope: ScopeGlobal, Default: "Alt+2"},
	{ID: "nav.3", Label: "Go to Strategies", Scope: ScopeGlobal, Default: "Alt+3"},
	{ID: "nav.4", Label: "Go to Equity Builder", Scope: ScopeGlobal, Default: "Alt+4"},
	{ID: "nav.5", Label: "Go to Live Strategies", Scope: ScopeGlobal, Default: "Alt+5"},
	{ID: "nav.6", Label: "Go to Scanners", Scope: ScopeGlobal, Default: "Alt+6"},
	{ID: "nav.7", Label: "Go to New Scanner", Scope: ScopeGlobal, Default: "Alt+7"},
	{ID: "nav.8", Label: "Go to Scanner Alerts", Scope: ScopeGlobal, Default: "Alt+8"},
	{ID: "toasts.focus", Label: "Focus toasts", Scope: ScopeGlobal, Default: "Alt+T"},
	{ID: "search.open", Label: "Open search", Scope: ScopeGlobal, Default: "Ctrl+K"},
	{ID: "help.open", Label: "Open help", Scope: ScopeGlobal, Default: "F1"},
	{ID: "kill.switch", Label: "Kill switch", Scope: ScopeGlobal, Default: "Ctrl+Shift+K"},
	{ID: "chart.fullView", Label: "Full-view chart", Scope: ScopeGlobal, Default: "F11"},
	{ID: "chart.trendLine", Label: "Draw trend line", Scope: ScopeChart, Default: "Alt+T"},
	{ID: "order.new", Label: "New order", Scope: ScopeChart, Default: "Shift+T"},
	{ID: "builder.undo", Label: "Undo", Scope: ScopeBuilder, Default: "Ctrl+Z"},
	{ID: "builder.redo", Label: "Redo", Scope: ScopeBuilder, Default: "Ctrl+Shift+Z"},
	{ID: "watchlist.moveUp", Label: "Move item up", Scope: ScopeView, Default: "Alt+ArrowUp"},
	{ID: "watchlist.moveDown", Label: "Move item down", Scope: ScopeView, Default: "Alt+ArrowDown"},
}

// DefaultShortcuts returns a fresh copy of the registry with Binding == Default.
func DefaultShortcuts() []ShortcutAction {
	out := make([]ShortcutAction, len(defaultShortcuts))
	for i, a := range defaultShortcuts {
		a.Binding = a.Default
		out[i] = a
	}
	return out
}

// Binding is a parsed key combination.
type Binding struct {
	Ctrl, Alt, Shift, Meta bool
	Key                    string
}

// ParseBinding parses "+"-joined modifiers (Ctrl, Alt, Shift, Meta; any case) followed by exactly one key.
// Single letters are upper-cased; other keys keep their KeyboardEvent.key spelling (F11, ArrowUp, Escape).
func ParseBinding(s string) (Binding, error) {
	var b Binding
	parts := strings.Split(strings.TrimSpace(s), "+")
	if len(parts) == 0 || strings.TrimSpace(s) == "" {
		return b, errors.New("binding is empty")
	}
	for i, raw := range parts {
		p := strings.TrimSpace(raw)
		if p == "" {
			return b, fmt.Errorf("binding %q has an empty part", s)
		}
		last := i == len(parts)-1
		switch strings.ToLower(p) {
		case "ctrl", "control":
			if last || b.Ctrl {
				return b, fmt.Errorf("binding %q: Ctrl is a modifier, not a key", s)
			}
			b.Ctrl = true
		case "alt":
			if last || b.Alt {
				return b, fmt.Errorf("binding %q: Alt is a modifier, not a key", s)
			}
			b.Alt = true
		case "shift":
			if last || b.Shift {
				return b, fmt.Errorf("binding %q: Shift is a modifier, not a key", s)
			}
			b.Shift = true
		case "meta", "super", "cmd":
			if last || b.Meta {
				return b, fmt.Errorf("binding %q: Meta is a modifier, not a key", s)
			}
			b.Meta = true
		default:
			if !last {
				return b, fmt.Errorf("binding %q: %q must be the last part", s, p)
			}
			if len([]rune(p)) == 1 {
				p = strings.ToUpper(p)
			} else {
				p = strings.ToUpper(p[:1]) + p[1:]
				if strings.HasPrefix(strings.ToLower(p), "arrow") && len(p) > 5 {
					p = "Arrow" + strings.ToUpper(p[5:6]) + p[6:]
				}
			}
			b.Key = p
		}
	}
	if b.Key == "" {
		return b, fmt.Errorf("binding %q has no key", s)
	}
	return b, nil
}

// String renders the canonical form: Ctrl+Alt+Shift+Meta order, then the key.
func (b Binding) String() string {
	var parts []string
	if b.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if b.Alt {
		parts = append(parts, "Alt")
	}
	if b.Shift {
		parts = append(parts, "Shift")
	}
	if b.Meta {
		parts = append(parts, "Meta")
	}
	return strings.Join(append(parts, b.Key), "+")
}

// Conflicts reports another action that already uses binding in the same scope as id or in the global scope.
func Conflicts(actions []ShortcutAction, id, binding string) (ShortcutAction, bool) {
	scope := ""
	for _, a := range actions {
		if a.ID == id {
			scope = a.Scope
		}
	}
	for _, a := range actions {
		if a.ID == id || a.Binding != binding {
			continue
		}
		if a.Scope == scope || a.Scope == ScopeGlobal || scope == ScopeGlobal {
			return a, true
		}
	}
	return ShortcutAction{}, false
}
