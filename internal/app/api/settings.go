// internal/app/api/settings.go
package api

import (
	"context"

	"tradeloom/internal/service"
	"tradeloom/internal/settings"
)

type Settings struct {
	svc *service.Service
	ctx func() context.Context
}

func NewSettings(svc *service.Service, ctx func() context.Context) *Settings {
	return &Settings{svc: svc, ctx: ctx}
}

func (s *Settings) All() ([]service.SettingValue, error) { return s.svc.Settings(s.ctx()) }
func (s *Settings) Set(name, valueJSON string) (service.SettingValue, error) {
	return s.svc.SetSetting(s.ctx(), name, valueJSON)
}

// Shortcuts (FR-SHELL-019 keyboard, Task 28).
func (s *Settings) Shortcuts() ([]settings.ShortcutAction, error) { return s.svc.Shortcuts(s.ctx()) }
func (s *Settings) Rebind(actionID, binding string) (settings.ShortcutAction, error) {
	return s.svc.Rebind(s.ctx(), actionID, binding)
}
func (s *Settings) ResetShortcuts() ([]settings.ShortcutAction, error) {
	return s.svc.ResetShortcuts(s.ctx())
}

// UI state (window_state table).
func (s *Settings) GetUiState(key string) (string, error) { return s.svc.GetUiState(s.ctx(), key) }
func (s *Settings) SetUiState(key, value string) error    { return s.svc.SetUiState(s.ctx(), key, value) }
