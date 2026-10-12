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
	{ID: "chart.horizontalLine", Label: "Draw horizontal line", Scope: ScopeChart, Default: "Alt+H"},
	{ID: "chart.horizontalRay", Label: "Draw horizontal ray", Scope: ScopeChart, Default: "Alt+J"},
	{ID: "chart.verticalLine", Label: "Draw vertical line", Scope: ScopeChart, Default: "Alt+V"},
	{ID: "chart.crossLine", Label: "Draw cross line", Scope: ScopeChart, Default: "Alt+C"},
	{ID: "order.new", Label: "New order", Scope: ScopeChart, Default: "Shift+T"},
	// Scalper actions ship unbound (FR-SCALP-013); an empty binding never conflicts.
	{ID: "scalper.buy", Label: "Scalper: buy", Scope: ScopeView},
	{ID: "scalper.sell", Label: "Scalper: sell", Scope: ScopeView},
	{ID: "scalper.exitAll", Label: "Scalper: exit all", Scope: ScopeView},
	{ID: "scalper.cancelAll", Label: "Scalper: cancel all", Scope: ScopeView},
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

// namedKeys maps the lower-cased spelling to the KeyboardEvent.key spelling. "Space" is the only alias:
// the event key itself is " ", which Binding.Key stores and String renders as "Space".
var namedKeys = func() map[string]string {
	m := map[string]string{"space": " "}
	for _, k := range []string{"Escape", "Enter", "Tab", "Backspace", "Delete", "Insert", "Home", "End", "PageUp", "PageDown", "ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"} {
		m[strings.ToLower(k)] = k
	}
	return m
}()

const punctuationKeys = "`-=[]\\;',./"

// normaliseKey returns the canonical key for p, or false when p is not on the allow-list: A-Z, 0-9, F1-F24,
// the punctuation keys and the named keys.
func normaliseKey(p string) (string, bool) {
	if n, ok := namedKeys[strings.ToLower(p)]; ok {
		return n, true
	}
	if len(p) == 1 { // ASCII only: a multi-byte or invalid rune is longer than one byte
		c := p[0]
		switch {
		case c >= 'a' && c <= 'z':
			return string(c - 'a' + 'A'), true
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9', strings.IndexByte(punctuationKeys, c) >= 0:
			return p, true
		}
		return "", false
	}
	if (p[0] == 'F' || p[0] == 'f') && len(p) <= 3 {
		n := 0
		for _, c := range p[1:] {
			if c < '0' || c > '9' {
				return "", false
			}
			n = n*10 + int(c-'0')
		}
		if n >= 1 && n <= 24 && p[1] != '0' {
			return "F" + p[1:], true
		}
	}
	return "", false
}

// ParseBinding parses "+"-joined modifiers (Ctrl, Alt, Shift, Meta; any case) followed by exactly one key
// from the allow-list (normaliseKey); keys use the KeyboardEvent.key spelling.
func ParseBinding(s string) (Binding, error) {
	var b Binding
	if strings.TrimSpace(s) == "" {
		return b, errors.New("binding is empty")
	}
	parts := strings.Split(strings.TrimSpace(s), "+")
	for i, raw := range parts {
		p := strings.TrimSpace(raw)
		if p == "" {
			return b, fmt.Errorf("binding %q has an empty part", s)
		}
		last := i == len(parts)-1
		var flag *bool
		switch strings.ToLower(p) {
		case "ctrl", "control":
			flag = &b.Ctrl
		case "alt":
			flag = &b.Alt
		case "shift":
			flag = &b.Shift
		case "meta", "super", "cmd":
			flag = &b.Meta
		}
		if flag != nil {
			if last || *flag {
				return b, fmt.Errorf("binding %q: %q is a modifier, not a key", s, p)
			}
			*flag = true
			continue
		}
		if !last {
			return b, fmt.Errorf("binding %q: %q must be the last part", s, p)
		}
		k, ok := normaliseKey(p)
		if !ok {
			return b, fmt.Errorf("binding %q: %q is not a supported key", s, p)
		}
		b.Key = k
	}
	if b.Key == "" {
		return b, fmt.Errorf("binding %q has no key", s)
	}
	return b, nil
}

var reservedBindings = map[string]bool{
	"Ctrl+C": true, "Ctrl+V": true, "Ctrl+X": true, "Ctrl+A": true, "Ctrl+Q": true, "Ctrl+W": true,
	"Alt+F4": true, "F5": true, "F12": true,
}

// ValidateBinding parses s and applies the rules of a scope: global and view bindings need a modifier
// (Shift alone does not count for a printable key) unless the key is F1-F24, and the reserved list (clipboard,
// quit, close, reload, devtools) is refused everywhere. Alt+letter stays allowed (no GTK mnemonics, FR-SHELL-019).
// It returns the canonical form.
func ValidateBinding(scope, s string) (string, error) {
	switch scope {
	case ScopeGlobal, ScopeView, ScopeChart, ScopeBuilder:
	default:
		return "", fmt.Errorf("unknown shortcut scope %q", scope)
	}
	b, err := ParseBinding(s)
	if err != nil {
		return "", err
	}
	canon := b.String()
	if reservedBindings[canon] {
		return "", fmt.Errorf("%s is reserved and cannot be used", canon)
	}
	if scope == ScopeGlobal || scope == ScopeView {
		fn := len(b.Key) >= 2 && b.Key[0] == 'F' && b.Key[1] >= '0' && b.Key[1] <= '9'
		if !fn && !b.Ctrl && !b.Alt && !b.Meta {
			return "", fmt.Errorf("%s needs Ctrl, Alt or Meta in the %s scope", canon, scope)
		}
	}
	return canon, nil
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
	key := b.Key
	if key == " " {
		key = "Space"
	}
	return strings.Join(append(parts, key), "+")
}

// Conflicts reports another action that already uses binding in the same scope as id, or in the global scope
// (a global action overlaps every scope). Unbound actions never conflict, an unknown id reports nothing, and
// restoring an action to its own default ignores a shipped default overlap (toasts.focus / chart.trendLine
// both default to Alt+T and are resolved by scope at runtime).
func Conflicts(actions []ShortcutAction, id, binding string) (ShortcutAction, bool) {
	if binding == "" {
		return ShortcutAction{}, false
	}
	var self ShortcutAction
	found := false
	for _, a := range actions {
		if a.ID == id {
			self, found = a, true
		}
	}
	if !found {
		return ShortcutAction{}, false
	}
	for _, a := range actions {
		if a.ID == id || a.Binding != binding {
			continue
		}
		if a.Scope != self.Scope && a.Scope != ScopeGlobal && self.Scope != ScopeGlobal {
			continue
		}
		if binding == self.Default && a.Binding == a.Default {
			continue // shipped overlap
		}
		return a, true
	}
	return ShortcutAction{}, false
}
