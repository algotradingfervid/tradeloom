// internal/settings/registry.go
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ErrUnknownKey is returned by GetJSON/SetJSON for a name no package registered.
var ErrUnknownKey = errors.New("settings: unknown key")

type entry struct {
	get func(s *Settings) (json.RawMessage, error)
	set func(ctx context.Context, s *Settings, raw json.RawMessage) error
}

var (
	regMu    sync.RWMutex
	registry = map[string]entry{}
)

// Register records k so the generic settings façade can list, read and write it by name.
// Declare keys as `var X = settings.Register(settings.Key[T]{…})`. It panics on a duplicate name.
func Register[T any](k Key[T]) Key[T] {
	if isSecretKey(k.Name) {
		panic(fmt.Sprintf("settings: key %q looks like a secret; secrets belong in internal/secrets", k.Name))
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := registry[k.Name]; dup {
		panic(fmt.Sprintf("settings: key %q registered twice", k.Name))
	}
	registry[k.Name] = entry{
		get: func(s *Settings) (json.RawMessage, error) { return json.Marshal(Get(s, k)) },
		set: func(ctx context.Context, s *Settings, raw json.RawMessage) error {
			var v T
			if err := json.Unmarshal(raw, &v); err != nil {
				return &ValidationError{Key: k.Name, Err: err}
			}
			return Set(ctx, s, k, v)
		},
	}
	return k
}

func lookup(name string) (entry, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	e, ok := registry[name]
	return e, ok
}

// Names returns every registered key name, sorted.
func Names() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// GetJSON returns the effective value of a registered key (stored, or the default) as JSON.
func GetJSON(s *Settings, name string) (json.RawMessage, error) {
	e, ok := lookup(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownKey, name)
	}
	return e.get(s)
}

// SetJSON parses raw as the key's type and stores it through Set (validation, secret guard, change event).
func SetJSON(ctx context.Context, s *Settings, name string, raw json.RawMessage) error {
	e, ok := lookup(name)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownKey, name)
	}
	return e.set(ctx, s, raw)
}

// P0-1's keys are plain struct literals in keys.go; register them here so the façade sees them.
func init() {
	Register(LogLevel)
	Register(BackupScheduleEnabled)
	Register(BackupScheduleFolder)
	Register(BackupScheduleKeep)
	Register(BackupIncludeCandles)
}
