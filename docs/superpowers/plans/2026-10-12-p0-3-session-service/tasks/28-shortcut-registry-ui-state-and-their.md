> **P0-3 Session & Service Layer Implementation Plan — Task 28.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 28: Shortcut registry, UI state and their façade (FR-SHELL-019)

**Files:**
- Create: `internal/settings/shortcuts.go`, `internal/store/shortcuts.go`, `internal/store/windowstate.go`, `internal/service/shortcuts.go`
- Modify: `internal/service/deps.go` (Task 14: two `Deps` fields)
- Test: `internal/settings/shortcuts_test.go`, `internal/store/shortcuts_test.go` (package `store_test`), `internal/service/shortcuts_test.go`

**Interfaces:**
- Consumes: v1 tables `shortcuts(action_id PK, binding, scope)` and `window_state(k PK, v)` (P0-1); `(*DB).W` / `.R`; `storetest.Open` (Task 5); Task 14 `refused`, `asRefused`.
- Produces:
  ```go
  // package settings (shortcuts.go) — contract §E / P0-4 A8
  const ScopeGlobal, ScopeView, ScopeChart, ScopeBuilder = "global", "view", "chart", "builder"
  type ShortcutAction struct{ ID string `json:"id"`; Label string `json:"label"`; Scope string `json:"scope"`; Default string `json:"defaultBinding"`; Binding string `json:"binding"` } // TS settings.ShortcutAction
  func DefaultShortcuts() []ShortcutAction // a fresh copy in registry order, Binding == Default
  type Binding struct{ Ctrl, Alt, Shift, Meta bool; Key string }
  func ParseBinding(s string) (Binding, error) // "+"-joined modifiers and one key; canonical form via String()
  func (b Binding) String() string             // Ctrl+Alt+Shift+Meta order, then the key
  func Conflicts(actions []ShortcutAction, id, binding string) (ShortcutAction, bool) // another action with the same binding in the same scope or in global
  // package store
  type ShortcutRow struct{ ActionID, Binding, Scope string }
  func NewShortcutsRepo(db *DB) *ShortcutsRepo
  func (r *ShortcutsRepo) Load(ctx context.Context) ([]ShortcutRow, error)
  func (r *ShortcutsRepo) Save(ctx context.Context, actionID, binding, scope string) error // upsert
  func (r *ShortcutsRepo) Reset(ctx context.Context) error                               // delete all
  func NewWindowStateRepo(db *DB) *WindowStateRepo
  func (r *WindowStateRepo) Get(ctx context.Context, k string) (string, error) // "" when absent
  func (r *WindowStateRepo) Set(ctx context.Context, k, v string) error
  // package service — Deps gains: Shortcuts *store.ShortcutsRepo; UiState *store.WindowStateRepo
  func (s *Service) Shortcuts(ctx context.Context) ([]settings.ShortcutAction, error)
  func (s *Service) Rebind(ctx context.Context, actionID, binding string) (settings.ShortcutAction, error) // unknown_action, invalid, conflict
  func (s *Service) ResetShortcuts(ctx context.Context) ([]settings.ShortcutAction, error)
  func (s *Service) GetUiState(ctx context.Context, key string) (string, error) // invalid_key
  func (s *Service) SetUiState(ctx context.Context, key, value string) error
  ```

The registry is Go-declared so the shell, the help view and the settings view read one list (P0-4 A8). Only user rebinds are stored; `Shortcuts()` overlays them on the defaults, so a later release can add actions without a migration. `Conflicts` is applied to rebinds only: the shipped defaults deliberately use `Alt+T` for `toasts.focus` (global) and `chart.trendLine` (chart), which P0-4's shortcut provider resolves by scope. UI state is a plain key/value table for things that are state, not settings (`chart.dashboard`, `window.geometry`, the quote cache of Task 29, the scheduled-backup marker of Task 30).

- [ ] **Step 1: Write the failing tests**

```go
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
```

```go
// internal/store/shortcuts_test.go
package store_test

import (
	"context"
	"testing"

	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func TestShortcutsRepo(t *testing.T) {
	ctx := context.Background()
	r := store.NewShortcutsRepo(storetest.Open(t))
	if rows, err := r.Load(ctx); err != nil || len(rows) != 0 {
		t.Fatalf("fresh %+v %v", rows, err)
	}
	if err := r.Save(ctx, "search.open", "Ctrl+P", "global"); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(ctx, "search.open", "Ctrl+O", "global"); err != nil { // upsert
		t.Fatal(err)
	}
	if err := r.Save(ctx, "order.new", "Shift+O", "chart"); err != nil {
		t.Fatal(err)
	}
	rows, err := r.Load(ctx)
	if err != nil || len(rows) != 2 || rows[0].ActionID != "order.new" || rows[1].ActionID != "search.open" || rows[1].Binding != "Ctrl+O" || rows[0].Scope != "chart" {
		t.Fatalf("%+v %v", rows, err)
	}
	if err := r.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if rows, _ := r.Load(ctx); len(rows) != 0 {
		t.Fatalf("after reset %+v", rows)
	}
}

func TestWindowStateRepo(t *testing.T) {
	ctx := context.Background()
	r := store.NewWindowStateRepo(storetest.Open(t))
	if v, err := r.Get(ctx, "window.geometry"); err != nil || v != "" {
		t.Fatalf("absent %q %v", v, err)
	}
	if err := r.Set(ctx, "window.geometry", `{"w":1}`); err != nil {
		t.Fatal(err)
	}
	if err := r.Set(ctx, "window.geometry", `{"w":2}`); err != nil {
		t.Fatal(err)
	}
	if v, err := r.Get(ctx, "window.geometry"); err != nil || v != `{"w":2}` {
		t.Fatalf("%q %v", v, err)
	}
}
```

```go
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/settings/ ./internal/store/ ./internal/service/ -run 'TestDefaultShortcuts|TestParseBinding|TestConflicts|TestShortcutsRepo|TestWindowStateRepo|TestShortcuts_|TestUiState' -v`
Expected: FAIL to compile with `undefined: DefaultShortcuts`, `undefined: store.NewShortcutsRepo`, `e.svc.d.Shortcuts undefined`.

- [ ] **Step 3: Write the registry, the repositories and the façade**

```go
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
```

```go
// internal/store/shortcuts.go
package store

import "context"

// ShortcutRow is one user rebind in the shortcuts table.
type ShortcutRow struct {
	ActionID string
	Binding  string
	Scope    string
}

type ShortcutsRepo struct{ db *DB }

func NewShortcutsRepo(db *DB) *ShortcutsRepo { return &ShortcutsRepo{db: db} }

func (r *ShortcutsRepo) Load(ctx context.Context) ([]ShortcutRow, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT action_id, binding, COALESCE(scope, '') FROM shortcuts ORDER BY action_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ShortcutRow
	for rows.Next() {
		var s ShortcutRow
		if err := rows.Scan(&s.ActionID, &s.Binding, &s.Scope); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ShortcutsRepo) Save(ctx context.Context, actionID, binding, scope string) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO shortcuts(action_id, binding, scope) VALUES(?, ?, ?)
		 ON CONFLICT(action_id) DO UPDATE SET binding = excluded.binding, scope = excluded.scope`, actionID, binding, scope)
	return err
}

func (r *ShortcutsRepo) Reset(ctx context.Context) error {
	_, err := r.db.W.ExecContext(ctx, `DELETE FROM shortcuts`)
	return err
}
```

```go
// internal/store/windowstate.go
package store

import (
	"context"
	"database/sql"
	"errors"
)

// WindowStateRepo is the UI state key/value table (window geometry, dashboard chart, quote cache …).
type WindowStateRepo struct{ db *DB }

func NewWindowStateRepo(db *DB) *WindowStateRepo { return &WindowStateRepo{db: db} }

// Get returns the stored value, or "" when the key is absent.
func (r *WindowStateRepo) Get(ctx context.Context, k string) (string, error) {
	var v string
	err := r.db.R.QueryRowContext(ctx, `SELECT v FROM window_state WHERE k = ?`, k).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (r *WindowStateRepo) Set(ctx context.Context, k, v string) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO window_state(k, v) VALUES(?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v`, k, v)
	return err
}
```

In `deps.go` add to `Deps` (after `NewKeyring`):

```go
	Shortcuts *store.ShortcutsRepo   // user rebinds (Task 28)
	UiState   *store.WindowStateRepo // window_state key/value (Task 28)
```

```go
// internal/service/shortcuts.go
package service

import (
	"context"
	"strings"

	"tradeloom/internal/settings"
)

const (
	shortcutsStage = "shortcuts"
	uiStateStage   = "ui_state"
)

// Shortcuts returns the registry with the user's rebinds applied (FR-SHELL-019 keyboard).
func (s *Service) Shortcuts(ctx context.Context) ([]settings.ShortcutAction, error) {
	acts := settings.DefaultShortcuts()
	if s.d.Shortcuts == nil {
		return acts, nil
	}
	rows, err := s.d.Shortcuts.Load(ctx)
	if err != nil {
		return nil, asRefused(shortcutsStage, err)
	}
	byID := make(map[string]string, len(rows))
	for _, r := range rows {
		byID[r.ActionID] = r.Binding
	}
	for i := range acts {
		if b, ok := byID[acts[i].ID]; ok {
			acts[i].Binding = b
		}
	}
	return acts, nil
}

// Rebind stores a new binding for an action; conflicts name the other action (P0-4 A1).
func (s *Service) Rebind(ctx context.Context, actionID, binding string) (settings.ShortcutAction, error) {
	acts, err := s.Shortcuts(ctx)
	if err != nil {
		return settings.ShortcutAction{}, err
	}
	idx := -1
	for i, a := range acts {
		if a.ID == actionID {
			idx = i
		}
	}
	if idx < 0 {
		return settings.ShortcutAction{}, refused(shortcutsStage, "unknown_action", "Unknown shortcut action "+actionID+".")
	}
	b, err := settings.ParseBinding(binding)
	if err != nil {
		return settings.ShortcutAction{}, refused(shortcutsStage, "invalid", err.Error())
	}
	canon := b.String()
	if other, ok := settings.Conflicts(acts, actionID, canon); ok {
		return settings.ShortcutAction{}, refused(shortcutsStage, "conflict", other.Label)
	}
	if s.d.Shortcuts == nil {
		return settings.ShortcutAction{}, refused(shortcutsStage, "unavailable", "Shortcut storage is not configured.")
	}
	if err := s.d.Shortcuts.Save(ctx, actionID, canon, acts[idx].Scope); err != nil {
		return settings.ShortcutAction{}, asRefused(shortcutsStage, err)
	}
	acts[idx].Binding = canon
	return acts[idx], nil
}

// ResetShortcuts forgets every rebind.
func (s *Service) ResetShortcuts(ctx context.Context) ([]settings.ShortcutAction, error) {
	if s.d.Shortcuts != nil {
		if err := s.d.Shortcuts.Reset(ctx); err != nil {
			return nil, asRefused(shortcutsStage, err)
		}
	}
	return settings.DefaultShortcuts(), nil
}

func uiKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", refused(uiStateStage, "invalid_key", "UI state keys cannot be empty.")
	}
	return key, nil
}

// GetUiState reads a window_state value ("" when absent).
func (s *Service) GetUiState(ctx context.Context, key string) (string, error) {
	k, err := uiKey(key)
	if err != nil {
		return "", err
	}
	if s.d.UiState == nil {
		return "", nil
	}
	v, err := s.d.UiState.Get(ctx, k)
	return v, asRefused(uiStateStage, err)
}

// SetUiState writes a window_state value.
func (s *Service) SetUiState(ctx context.Context, key, value string) error {
	k, err := uiKey(key)
	if err != nil {
		return err
	}
	if s.d.UiState == nil {
		return refused(uiStateStage, "unavailable", "UI state storage is not configured.")
	}
	return asRefused(uiStateStage, s.d.UiState.Set(ctx, k, value))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/settings/ ./internal/store/ ./internal/service/ -run 'TestDefaultShortcuts|TestParseBinding|TestConflicts|TestShortcutsRepo|TestWindowStateRepo|TestShortcuts_|TestUiState' -v`
Expected: PASS (7 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/settings/shortcuts.go internal/settings/shortcuts_test.go internal/store/shortcuts.go internal/store/windowstate.go internal/store/shortcuts_test.go \
  internal/service/shortcuts.go internal/service/shortcuts_test.go internal/service/deps.go
git commit -m "feat(settings,store,service): shortcut registry with rebind conflicts, and UI state storage

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
