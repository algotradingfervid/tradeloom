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
		return nil, s.refuse(shortcutsStage, err)
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
		return settings.ShortcutAction{}, s.refuse(shortcutsStage, err)
	}
	acts[idx].Binding = canon
	return acts[idx], nil
}

// ResetShortcuts forgets every rebind.
func (s *Service) ResetShortcuts(ctx context.Context) ([]settings.ShortcutAction, error) {
	if s.d.Shortcuts != nil {
		if err := s.d.Shortcuts.Reset(ctx); err != nil {
			return nil, s.refuse(shortcutsStage, err)
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
	return v, s.refuse(uiStateStage, err)
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
	return s.refuse(uiStateStage, s.d.UiState.Set(ctx, k, value))
}
