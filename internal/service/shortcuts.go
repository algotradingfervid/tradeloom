// internal/service/shortcuts.go
package service

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"tradeloom/internal/bus"
	"tradeloom/internal/settings"
)

const (
	shortcutsStage = "shortcuts"
	uiStateStage   = "ui_state"

	// shortcutsChangeKey is the key of the settings.changed event published after a rebind or reset;
	// its value is the full action list.
	shortcutsChangeKey = "shortcuts"

	maxIDBytes      = 64
	maxUiKeyBytes   = 64
	maxUiValueBytes = 64 * 1024
)

// loadShortcuts overlays the stored rebinds on the defaults. Stored rows for unknown actions or with a binding
// that no longer validates for the action's scope are ignored (a later release may drop an action or tighten
// the rules). The caller holds shortcutsMu when it goes on to modify.
func (s *Service) loadShortcuts(ctx context.Context) ([]settings.ShortcutAction, error) {
	if s.d.Shortcuts == nil {
		return nil, refused(shortcutsStage, "unavailable", "Shortcut storage is not configured.")
	}
	acts := settings.DefaultShortcuts()
	rows, err := s.d.Shortcuts.Load(ctx)
	if err != nil {
		return nil, s.refuse(shortcutsStage, err)
	}
	byID := make(map[string]string, len(rows))
	for _, r := range rows {
		byID[r.ActionID] = r.Binding
	}
	for i := range acts {
		b, ok := byID[acts[i].ID]
		if !ok {
			continue
		}
		if canon, err := settings.ValidateBinding(acts[i].Scope, b); err == nil {
			acts[i].Binding = canon
		}
	}
	return acts, nil
}

// Shortcuts returns the registry with the user's rebinds applied (FR-SHELL-019 keyboard).
func (s *Service) Shortcuts(ctx context.Context) ([]settings.ShortcutAction, error) {
	s.shortcutsMu.Lock()
	defer s.shortcutsMu.Unlock()
	return s.loadShortcuts(ctx)
}

func (s *Service) publishShortcuts(acts []settings.ShortcutAction) {
	b, err := json.Marshal(acts)
	if err != nil {
		s.d.Log.Warn("service.shortcuts_event", "err", err)
		return
	}
	bus.Publish(s.d.Bus, settings.Topic, settings.Changed{Key: shortcutsChangeKey, Value: b})
}

// Rebind stores a new binding for an action; conflicts name the other action (P0-4 A1).
func (s *Service) Rebind(ctx context.Context, actionID, binding string) (settings.ShortcutAction, error) {
	if len(actionID) > maxIDBytes || len(binding) > maxIDBytes {
		return settings.ShortcutAction{}, refused(shortcutsStage, "invalid", "Shortcut action or binding is too long.")
	}
	s.shortcutsMu.Lock()
	defer s.shortcutsMu.Unlock()
	acts, err := s.loadShortcuts(ctx)
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
		return settings.ShortcutAction{}, refused(shortcutsStage, "unknown_action", "Unknown shortcut action.")
	}
	canon, err := settings.ValidateBinding(acts[idx].Scope, binding)
	if err != nil {
		return settings.ShortcutAction{}, refused(shortcutsStage, "invalid", err.Error())
	}
	if other, ok := settings.Conflicts(acts, actionID, canon); ok {
		return settings.ShortcutAction{}, refused(shortcutsStage, "conflict", other.Label)
	}
	if err := s.d.Shortcuts.Save(ctx, actionID, canon, acts[idx].Scope); err != nil {
		return settings.ShortcutAction{}, s.refuse(shortcutsStage, err)
	}
	acts[idx].Binding = canon
	s.publishShortcuts(acts)
	return acts[idx], nil
}

// ResetShortcuts forgets every rebind.
func (s *Service) ResetShortcuts(ctx context.Context) ([]settings.ShortcutAction, error) {
	if s.d.Shortcuts == nil {
		return nil, refused(shortcutsStage, "unavailable", "Shortcut storage is not configured.")
	}
	s.shortcutsMu.Lock()
	defer s.shortcutsMu.Unlock()
	if err := s.d.Shortcuts.Reset(ctx); err != nil {
		return nil, s.refuse(shortcutsStage, err)
	}
	acts := settings.DefaultShortcuts()
	s.publishShortcuts(acts)
	return acts, nil
}

// backendUiKey reports keys owned by backend code (window geometry, last-quote cache, scheduled-backup marker);
// the UI may read them but not write them.
func backendUiKey(k string) bool {
	return k == "quotes.last" || k == "window.geometry" || strings.HasPrefix(k, "backup.")
}

func validUiKey(k string) bool {
	if k == "" || len(k) > maxUiKeyBytes {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func uiKey(key string) (string, error) {
	if !validUiKey(key) {
		return "", refused(uiStateStage, "invalid_key", "UI state keys are 1-64 characters of a-z, 0-9, '.', '_' and '-'.")
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
		return "", refused(uiStateStage, "unavailable", "UI state storage is not configured.")
	}
	v, err := s.d.UiState.Get(ctx, k)
	if err != nil {
		return "", s.refuse(uiStateStage, err)
	}
	return v, nil
}

// SetUiState writes a window_state value (at most 64 KiB of UTF-8); backend-owned keys are refused.
func (s *Service) SetUiState(ctx context.Context, key, value string) error {
	k, err := uiKey(key)
	if err != nil {
		return err
	}
	if backendUiKey(k) {
		return refused(uiStateStage, "invalid_key", "This UI state key is managed by the application.")
	}
	if len(value) > maxUiValueBytes {
		return refused(uiStateStage, "too_large", "UI state values are limited to 64 KiB.")
	}
	if !utf8.ValidString(value) {
		return refused(uiStateStage, "invalid_value", "UI state values must be valid UTF-8.")
	}
	if s.d.UiState == nil {
		return refused(uiStateStage, "unavailable", "UI state storage is not configured.")
	}
	return s.refuse(uiStateStage, s.d.UiState.Set(ctx, k, value))
}
