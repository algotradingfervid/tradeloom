> **P0-1 Platform Core Implementation Plan — Task 10.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 10: `internal/settings` and `store.SettingsRepo`

**Files:**
- Create: `internal/store/settings_repo.go`, `internal/store/settings_repo_test.go`, `internal/settings/settings.go`, `internal/settings/keys.go`, `internal/settings/settings_test.go`

**Interfaces:**
- Consumes: Task 3 `bus.Bus`, `bus.NewTopic`, `bus.Publish`, `bus.Subscribe`, `bus.SubOpts`; Task 4 `logging.Default.IsSensitiveKey`, `logging.Default.RedactString`, `logging.RegisterSecret`; Task 7 `store.Open`, `store.Options`, `DB.W`, `DB.R`.
- Produces:
  - `package store`: `type SettingsRepo struct{…}`; `func (db *DB) SettingsRepo() *SettingsRepo`; `func (r *SettingsRepo) Load(ctx context.Context) (map[string]string, error)`; `func (r *SettingsRepo) Save(ctx context.Context, key, value string) error`
  - `package settings`:
    - `type Backend interface { Load(ctx context.Context) (map[string]string, error); Save(ctx context.Context, key, value string) error }`
    - `type Key[T any] struct { Name string; Default T; Validate func(T) error }`
    - `type Changed struct { Key string; Value json.RawMessage }` (JSON `key`, `value`); `var Topic = bus.NewTopic[Changed]("settings.changed")`
    - `var ErrSecretKey`, `var ErrSecretValue`; `type ValidationError struct { Key string; Err error }`
    - `type Settings struct{…}`; `func Load(ctx context.Context, b Backend, bs *bus.Bus) (*Settings, error)`; `func Get[T any](s *Settings, k Key[T]) T`; `func Set[T any](ctx context.Context, s *Settings, k Key[T], v T) error`; `func Reset[T any](ctx context.Context, s *Settings, k Key[T]) error`; `func (s *Settings) Snapshot() []Changed`
    - Validators: `func OneOf(allowed ...string) func(string) error`; `func IntRange(lo, hi int) func(int) error`; `func AbsPathOrEmpty(v string) error`
    - Keys: `LogLevel` (`"log.level"`, default `"info"`, one of info|debug); `BackupScheduleEnabled` (`"backup.schedule.enabled"`, false); `BackupScheduleFolder` (`"backup.schedule.folder"`, `""`, absolute or empty); `BackupScheduleKeep` (`"backup.schedule.keep"`, 7, 1…365); `BackupIncludeCandles` (`"backup.include_candles"`, false). Later sub-plans declare their keys in new files of `package settings` (e.g. `keys_kite.go`) using the same `Key[T]` pattern.

- [ ] **Step 1: Write the failing tests**

`internal/store/settings_repo_test.go`:

```go
package store

import (
	"context"
	"testing"
)

func TestSettingsRepo_RoundTrip(t *testing.T) {
	db := mustOpen(t, t.TempDir(), nil, "v0.1.0")
	defer db.Close()
	r := db.SettingsRepo()
	ctx := context.Background()
	if err := r.Save(ctx, "log.level", `"debug"`); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(ctx, "log.level", `"info"`); err != nil {
		t.Fatal(err)
	}
	m, err := r.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if m["log.level"] != `"info"` || m[SettingAppVersionLast] != `"v0.1.0"` {
		t.Fatalf("%v", m)
	}
}
```

`internal/settings/settings_test.go`:

```go
package settings

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
	"tradeloom/internal/store"
)

type memBackend struct {
	m       map[string]string
	saveErr error
}

func (b *memBackend) Load(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range b.m {
		out[k] = v
	}
	return out, nil
}

func (b *memBackend) Save(_ context.Context, k, v string) error {
	if b.saveErr != nil {
		return b.saveErr
	}
	b.m[k] = v
	return nil
}

func TestGet_DefaultsAndStoredValues(t *testing.T) {
	b := &memBackend{m: map[string]string{
		"backup.schedule.keep":   "12",
		"log.level":              `"verbose"`, // fails validation → default
		"backup.schedule.folder": `not json`,  // corrupt → default
	}}
	s, err := Load(context.Background(), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if Get(s, BackupScheduleKeep) != 12 || Get(s, LogLevel) != "info" || Get(s, BackupScheduleFolder) != "" || Get(s, BackupScheduleEnabled) {
		t.Fatal("unexpected values")
	}
}

func TestSet_ValidatesPersistsAndPublishes(t *testing.T) {
	bs := bus.New(nil)
	defer bs.Close()
	sub := bus.Subscribe(bs, Topic, bus.SubOpts[Changed]{Name: "test"})
	b := &memBackend{m: map[string]string{}}
	s, _ := Load(context.Background(), b, bs)

	cases := []struct {
		name    string
		set     func() error
		wantErr bool
	}{
		{"valid level", func() error { return Set(context.Background(), s, LogLevel, "debug") }, false},
		{"invalid level", func() error { return Set(context.Background(), s, LogLevel, "trace") }, true},
		{"keep too low", func() error { return Set(context.Background(), s, BackupScheduleKeep, 0) }, true},
		{"relative folder", func() error { return Set(context.Background(), s, BackupScheduleFolder, "backups") }, true},
		{"absolute folder", func() error { return Set(context.Background(), s, BackupScheduleFolder, "/media/usb/tl") }, false},
	}
	for _, tc := range cases {
		err := tc.set()
		if (err != nil) != tc.wantErr {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
		var ve *ValidationError
		if tc.wantErr && !errors.As(err, &ve) {
			t.Fatalf("%s: want ValidationError, got %T", tc.name, err)
		}
	}
	if Get(s, LogLevel) != "debug" || b.m["log.level"] != `"debug"` || b.m["backup.schedule.keep"] != "" {
		t.Fatalf("state %v", b.m)
	}
	for _, want := range []string{"log.level", "backup.schedule.folder"} {
		select {
		case ev := <-sub.C():
			if ev.Key != want {
				t.Fatalf("event %s, want %s", ev.Key, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no event for %s", want)
		}
	}
}

func TestSet_BackendErrorLeavesCacheUnchanged(t *testing.T) {
	b := &memBackend{m: map[string]string{}, saveErr: errors.New("disk I/O error")}
	s, _ := Load(context.Background(), b, nil)
	if err := Set(context.Background(), s, BackupScheduleKeep, 9); err == nil {
		t.Fatal("want error")
	}
	if Get(s, BackupScheduleKeep) != 7 {
		t.Fatal("cache changed on failed save")
	}
}

// NFR-04: settings never hold secrets.
func TestSet_RefusesSecrets(t *testing.T) {
	s, _ := Load(context.Background(), &memBackend{m: map[string]string{}}, nil)
	if err := Set(context.Background(), s, Key[string]{Name: "kite.api_secret"}, "x"); !errors.Is(err, ErrSecretKey) {
		t.Fatalf("want ErrSecretKey, got %v", err)
	}
	logging.RegisterSecret("registeredSecretValue42")
	if err := Set(context.Background(), s, Key[string]{Name: "note"}, "has registeredSecretValue42 inside"); !errors.Is(err, ErrSecretValue) {
		t.Fatalf("want ErrSecretValue, got %v", err)
	}
}

func TestSettings_OverStore(t *testing.T) {
	dir := t.TempDir()
	open := func() *store.DB {
		db, err := store.Open(context.Background(), filepath.Join(dir, "app.db"), store.Options{AppVersion: "v0.1.0",
			BackupDir: filepath.Join(dir, "backups"), Now: func() time.Time { return time.Unix(1, 0) }})
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	db := open()
	s, err := Load(context.Background(), db.SettingsRepo(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Set(context.Background(), s, BackupScheduleKeep, 30); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db = open()
	defer db.Close()
	s2, _ := Load(context.Background(), db.SettingsRepo(), nil)
	if Get(s2, BackupScheduleKeep) != 30 {
		t.Fatal("value not persisted")
	}
	snap := s2.Snapshot()
	var found bool
	for _, c := range snap {
		if c.Key == "backup.schedule.keep" && string(c.Value) == "30" {
			found = true
		}
	}
	if !found {
		t.Fatalf("snapshot %v", snap)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -tags noorders ./internal/store/ ./internal/settings/`
Expected: FAIL — build errors `db.SettingsRepo undefined` (store) and `undefined: Load`, `undefined: Topic` (settings).

- [ ] **Step 3: Write the implementation**

`internal/store/settings_repo.go`:

```go
package store

import "context"

// SettingsRepo persists settings rows (key → JSON text). It implements settings.Backend.
type SettingsRepo struct{ db *DB }

// SettingsRepo returns the settings table repository.
func (db *DB) SettingsRepo() *SettingsRepo { return &SettingsRepo{db: db} }

// Load returns every row.
func (r *SettingsRepo) Load(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.R.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// Save upserts one row.
func (r *SettingsRepo) Save(ctx context.Context, key, value string) error {
	_, err := r.db.W.ExecContext(ctx,
		`INSERT INTO settings(key, value, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, r.db.now().UnixMilli())
	return err
}
```

`internal/settings/settings.go`:

```go
// Package settings provides typed, validated application settings persisted as JSON
// in app.db's settings table, with a change event on the bus (ARCHITECTURE §2.2).
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
)

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
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return k.Default
	}
	if k.Validate != nil && k.Validate(v) != nil {
		return k.Default
	}
	return v
}

// Set validates, persists and caches v, then publishes Changed. Nothing changes on error.
func Set[T any](ctx context.Context, s *Settings, k Key[T], v T) error {
	if logging.Default.IsSensitiveKey(k.Name) {
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

// Snapshot returns every stored value (for the support bundle and the settings view), sorted by key.
func (s *Settings) Snapshot() []Changed {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Changed, 0, len(s.raw))
	for k, v := range s.raw {
		out = append(out, Changed{Key: k, Value: json.RawMessage(v)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
```

`internal/settings/keys.go`:

```go
package settings

import (
	"errors"
	"fmt"
	"path/filepath"
)

// Keys owned by P0-1. Later sub-plans add their keys in their own files of this package.
var (
	// LogLevel is Settings → Diagnostics "debug logging" (FR-SHELL-018): "info" | "debug".
	LogLevel = Key[string]{Name: "log.level", Default: "info", Validate: OneOf("info", "debug")}
	// BackupScheduleEnabled turns on the daily scheduled backup (FR-DATA-005; off by default).
	BackupScheduleEnabled = Key[bool]{Name: "backup.schedule.enabled", Default: false}
	// BackupScheduleFolder is the user-chosen folder for scheduled backups ("" until chosen).
	BackupScheduleFolder = Key[string]{Name: "backup.schedule.folder", Default: "", Validate: AbsPathOrEmpty}
	// BackupScheduleKeep is N, the number of scheduled backups kept (default 7).
	BackupScheduleKeep = Key[int]{Name: "backup.schedule.keep", Default: 7, Validate: IntRange(1, 365)}
	// BackupIncludeCandles is the "include the candle store" checkbox of Backup now.
	BackupIncludeCandles = Key[bool]{Name: "backup.include_candles", Default: false}
)

// OneOf accepts only the listed strings.
func OneOf(allowed ...string) func(string) error {
	return func(v string) error {
		for _, a := range allowed {
			if v == a {
				return nil
			}
		}
		return fmt.Errorf("%q is not one of %v", v, allowed)
	}
}

// IntRange accepts lo ≤ v ≤ hi.
func IntRange(lo, hi int) func(int) error {
	return func(v int) error {
		if v < lo || v > hi {
			return fmt.Errorf("%d is outside %d…%d", v, lo, hi)
		}
		return nil
	}
}

// AbsPathOrEmpty accepts "" or an absolute path.
func AbsPathOrEmpty(v string) error {
	if v != "" && !filepath.IsAbs(v) {
		return errors.New("must be an absolute path")
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race -count=1 -tags noorders ./internal/store/ ./internal/settings/ ./internal/archtest/`
Expected: `ok` for all three.

- [ ] **Step 5: Commit**

```bash
git add internal/store/settings_repo.go internal/store/settings_repo_test.go internal/settings
git commit -m "feat(settings): typed keys with defaults, validation, secret guard and change events over app.db" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
