// internal/service/settings.go
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"tradeloom/internal/logging"
	"tradeloom/internal/settings"
)

type SettingValue struct {
	Name      string `json:"name"`
	ValueJSON string `json:"valueJson"`
}

const settingsStage = "settings"

func credentialKey(name string) bool { return strings.HasPrefix(name, "kite.") }

// hiddenKey is fail-closed: credential keys and any secret-like name are never listed.
func hiddenKey(name string) bool { return credentialKey(name) || logging.IsSecretLikeKey(name) }

// Settings lists every registered key (sorted by settings.Names) except the credential keys.
func (s *Service) Settings(_ context.Context) ([]SettingValue, error) {
	names := settings.Names()
	sort.Strings(names)
	out := make([]SettingValue, 0, len(names))
	for _, n := range names {
		if hiddenKey(n) {
			continue
		}
		raw, err := settings.GetJSON(s.d.Settings, n)
		if err != nil {
			return nil, asRefused(settingsStage, err)
		}
		out = append(out, SettingValue{Name: n, ValueJSON: string(raw)})
	}
	return out, nil
}

func (s *Service) SetSetting(ctx context.Context, name, valueJSON string) (SettingValue, error) {
	if credentialKey(name) {
		return SettingValue{}, refused(settingsStage, "use_credentials_flow", "Change Kite settings in Settings → Kite connection.")
	}
	if !json.Valid([]byte(valueJSON)) {
		return SettingValue{}, refused(settingsStage, "invalid_setting", "The value is not valid JSON.")
	}
	if err := settings.SetJSON(ctx, s.d.Settings, name, json.RawMessage(valueJSON)); err != nil {
		if errors.Is(err, settings.ErrUnknownKey) {
			return SettingValue{}, refused(settingsStage, "unknown_setting", "Unknown setting "+name+".")
		}
		var ve *settings.ValidationError
		if errors.As(err, &ve) {
			return SettingValue{}, refused(settingsStage, "invalid_setting", err.Error())
		}
		return SettingValue{}, asRefused(settingsStage, err)
	}
	raw, err := settings.GetJSON(s.d.Settings, name)
	if err != nil {
		return SettingValue{}, asRefused(settingsStage, err)
	}
	return SettingValue{Name: name, ValueJSON: string(raw)}, nil
}
