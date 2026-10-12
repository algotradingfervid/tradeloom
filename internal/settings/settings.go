// Package settings provides typed, validated application settings persisted as JSON
// in app.db's settings table, with a change event on the bus (ARCHITECTURE §2.2).
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"sync"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
)

// secretKeyName is the settings-specific secret-key guard, applied in addition to
// logging.Default.IsSensitiveKey. It deliberately omits a bare "api_key": the Kite API key is
// not secret and lives in the setting kite.api_key (P0-3); only api_secret is refused.
var secretKeyName = regexp.MustCompile(`(?i)(secret|token|password|passwd|passphrase|api_?secret|totp|authorization|checksum)`)

func isSecretKey(name string) bool {
	return logging.Default.IsSensitiveKey(name) || secretKeyName.MatchString(name)
}

// Backend persists raw JSON values by key. *store.SettingsRepo implements it.
type Backend interface {
	Load(ctx context.Context) (map[string]string, error)
	Save(ctx context.Context, key, value string) error
}

// Key is a typed setting. Declare keys as package-level vars (see keys.go).
type Key[T any] struct {
	Name     string
	Default  T
	Validate func(T) error // nil accepts any value
}

// Changed is published on Topic after every successful Set.
type Changed struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// Topic carries every settings change; internal/app mirrors it to the UI as "settings.changed".
var Topic = bus.NewTopic[Changed]("settings.changed")

var (
	// ErrSecretKey: the key name looks like a secret; secrets belong in internal/secrets (NFR-04).
	ErrSecretKey = errors.New("settings: secret-like keys are not allowed in settings")
	// ErrSecretValue: the value contains a registered secret (NFR-04).
	ErrSecretValue = errors.New("settings: value contains a secret")
)

// ValidationError is returned by Set when Key.Validate rejects a value.
type ValidationError struct {
	Key string
	Err error
}

func (e *ValidationError) Error() string { return fmt.Sprintf("settings: %s: %v", e.Key, e.Err) }
func (e *ValidationError) Unwrap() error { return e.Err }

// Settings is an in-memory cache over a Backend. Safe for concurrent use.
type Settings struct {
	mu  sync.RWMutex
	b   Backend
	bus *bus.Bus
	raw map[string]string
}

// Load reads every stored value. bs may be nil (no change events, used by tools and tests).
func Load(ctx context.Context, b Backend, bs *bus.Bus) (*Settings, error) {
	raw, err := b.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("settings: load: %w", err)
	}
	// Drop rows that must never have been stored (NFR-04): secret-like names or registered secret values.
	for k, v := range raw {
		if isSecretKey(k) || logging.Default.RedactString(v) != v {
			slog.Warn("settings.secret_row_dropped", "component", "settings", "setting", k)
			delete(raw, k)
		}
	}
	return &Settings{b: b, bus: bs, raw: raw}, nil
}

// Get returns the stored value, or k.Default when it is missing, not valid JSON for T, or fails Validate.
func Get[T any](s *Settings, k Key[T]) T {
	s.mu.RLock()
	raw, ok := s.raw[k.Name]
	s.mu.RUnlock()
	if !ok {
		return k.Default
	}
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil || strings.TrimSpace(raw) == "null" {
		return k.Default
	}
	if k.Validate != nil && k.Validate(v) != nil {
		return k.Default
	}
	return v
}

// Set validates, persists and caches v, then publishes Changed. Nothing changes on error.
func Set[T any](ctx context.Context, s *Settings, k Key[T], v T) error {
	if isSecretKey(k.Name) {
		return fmt.Errorf("%w: %s", ErrSecretKey, k.Name)
	}
	if k.Validate != nil {
		if err := k.Validate(v); err != nil {
			return &ValidationError{Key: k.Name, Err: err}
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if logging.Default.RedactString(string(b)) != string(b) {
		return fmt.Errorf("%w: %s", ErrSecretValue, k.Name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.b.Save(ctx, k.Name, string(b)); err != nil {
		return fmt.Errorf("settings: save %s: %w", k.Name, err)
	}
	s.raw[k.Name] = string(b)
	if s.bus != nil {
		bus.Publish(s.bus, Topic, Changed{Key: k.Name, Value: json.RawMessage(b)})
	}
	return nil
}

// Reset stores k.Default.
func Reset[T any](ctx context.Context, s *Settings, k Key[T]) error { return Set(ctx, s, k, k.Default) }

// Snapshot returns every stored value, passed through the log redactor (for the support bundle and the settings view), sorted by key.
func (s *Settings) Snapshot() []Changed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Changed, 0, len(s.raw))
	for k, v := range s.raw {
		out = append(out, Changed{Key: k, Value: json.RawMessage(logging.Default.RedactString(v))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
