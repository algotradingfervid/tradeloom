> **P0-3 Session & Service Layer Implementation Plan — Task 2.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 2: Settings key registry, credential validation, settings keys and redirect URL guidance

**Files:**
- Create: `internal/settings/registry.go`, `internal/kite/credentials.go`
- Test: `internal/settings/registry_test.go`, `internal/kite/credentials_test.go`

**Interfaces:**
- Consumes: P0-1 `settings.Key[T]`, `settings.Load`, `settings.Get`, `settings.Set`, `settings.ValidationError`, the five P0-1 keys (`LogLevel`, `BackupScheduleEnabled`, `BackupScheduleFolder`, `BackupScheduleKeep`, `BackupIncludeCandles`), `settings.Backend`.
- Produces:
  ```go
  // package settings (registry.go) — P0-1 declares keys as plain struct literals; this registry lets the generic
  // settings façade (Task 17) list, read and write them by name without a type switch.
  func Register[T any](k Key[T]) Key[T]                                              // panics on a duplicate name
  func Names() []string                                                              // sorted
  func GetJSON(s *Settings, name string) (json.RawMessage, error)                    // effective value (stored or default)
  func SetJSON(ctx context.Context, s *Settings, name string, raw json.RawMessage) error // unmarshal into T, then Set
  var ErrUnknownKey error
  // package kite (credentials.go)
  const CallbackPath = "/kite/callback"; const DefaultRedirectPort = 7420
  var SettingAPIKey = settings.Register(settings.Key[string]{Name: "kite.api_key", Default: ""})          // validated when non-empty
  var SettingRedirectPort = settings.Register(settings.Key[int]{Name: "kite.redirect_port", Default: 7420, Validate: ValidatePort})
  func NormalizeCredential(s string) string
  func ValidateAPIKey(k string) error    // *LoginError{Code: CodeInvalidCredentials}
  func ValidateAPISecret(s string) error // *LoginError{Code: CodeInvalidCredentials}
  func ValidatePort(p int) error
  func RedirectURL(port int) string // "http://127.0.0.1:<port>/kite/callback"
  func MaskKey(k string) string     // "••••" + last 4
  type CredentialsInfo struct{ Configured bool; APIKeyMasked string; Port int; RedirectURL, SecretBackend string } // TS kite.CredentialsInfo
  ```

The registry lives in this task (not Task 17) because `SettingAPIKey` is the first key declared through `settings.Register`; P0-1's own keys are registered in `registry.go`'s `init()`. `kite.api_key` passes P0-1's secret-key guard (`logging.Default.IsSensitiveKey` matches `api_secret`, `access_token`, … but not `api_key`).

Kite does not document the key or secret format (A07 §A2). Validation is therefore deliberately loose: 6–64 ASCII letters or digits after trimming whitespace and stray quotes. S0-1 (Task 25) records real lengths. Tighten only if they prove uniform.

- [ ] **Step 1: Write the failing registry test**

```go
// internal/settings/registry_test.go
package settings

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

type regTestBackend struct{ m map[string]string }

func (b *regTestBackend) Load(context.Context) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range b.m {
		out[k] = v
	}
	return out, nil
}

func (b *regTestBackend) Save(_ context.Context, k, v string) error { b.m[k] = v; return nil }

func TestRegistry_NamesAreSortedAndIncludeP01Keys(t *testing.T) {
	names := Names()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("not sorted: %v", names)
	}
	for _, want := range []string{LogLevel.Name, BackupScheduleEnabled.Name, BackupScheduleFolder.Name, BackupScheduleKeep.Name, BackupIncludeCandles.Name} {
		if i := sort.SearchStrings(names, want); i >= len(names) || names[i] != want {
			t.Fatalf("%s not registered: %v", want, names)
		}
	}
}

func TestRegistry_GetSetJSON(t *testing.T) {
	ctx := context.Background()
	s, err := Load(ctx, &regTestBackend{m: map[string]string{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := GetJSON(s, "log.level")
	if err != nil || string(raw) != `"info"` {
		t.Fatalf("default: %s %v", raw, err)
	}
	if err := SetJSON(ctx, s, "log.level", json.RawMessage(`"debug"`)); err != nil {
		t.Fatal(err)
	}
	if raw, _ = GetJSON(s, "log.level"); string(raw) != `"debug"` {
		t.Fatalf("after set: %s", raw)
	}
	var ve *ValidationError
	if err := SetJSON(ctx, s, "log.level", json.RawMessage(`"trace"`)); !errors.As(err, &ve) {
		t.Fatalf("validator not applied: %v", err)
	}
	if err := SetJSON(ctx, s, "backup.schedule.keep", json.RawMessage(`"seven"`)); !errors.As(err, &ve) {
		t.Fatalf("type mismatch not reported as a validation error: %v", err)
	}
	if raw, _ = GetJSON(s, "backup.schedule.keep"); string(raw) != `7` {
		t.Fatalf("value changed on a rejected set: %s", raw)
	}
	if _, err := GetJSON(s, "no.such.key"); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("GetJSON unknown: %v", err)
	}
	if err := SetJSON(ctx, s, "no.such.key", json.RawMessage(`1`)); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("SetJSON unknown: %v", err)
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil || !strings.Contains(r.(string), "registered twice") {
			t.Fatalf("recover = %v", r)
		}
	}()
	Register(Key[string]{Name: LogLevel.Name})
}
```

- [ ] **Step 2: Run the registry test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/settings/ -run TestRegistry -v`
Expected: FAIL to compile with `undefined: Names`, `undefined: GetJSON`, `undefined: SetJSON`, `undefined: ErrUnknownKey`, `undefined: Register`.

- [ ] **Step 3: Write the registry**

```go
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
```

- [ ] **Step 4: Run the registry tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/settings/ -v`
Expected: PASS (every P0-1 settings test plus `TestRegistry_NamesAreSortedAndIncludeP01Keys`, `TestRegistry_GetSetJSON`, `TestRegistry_DuplicatePanics`).

- [ ] **Step 5: Write the failing credentials test**

```go
// internal/kite/credentials_test.go
package kite

import (
	"errors"
	"testing"
)

func TestNormalizeCredential(t *testing.T) {
	for in, want := range map[string]string{
		"  abc123def  ":     "abc123def",
		"\"abc123def\"\n":   "abc123def",
		"'abc123def'":       "abc123def",
		"\tabc123def\r\n":   "abc123def",
	} {
		if got := NormalizeCredential(in); got != want {
			t.Fatalf("NormalizeCredential(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateCredentials(t *testing.T) {
	ok := []string{"abcd12", "kitekey1234abcd", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz01"}
	bad := []string{"", "abc", "has space1", "semi;colon1", "ünïcode123", "x123456789012345678901234567890123456789012345678901234567890123456"}
	for _, k := range ok {
		if err := ValidateAPIKey(k); err != nil {
			t.Fatalf("ValidateAPIKey(%q) = %v, want nil", k, err)
		}
		if err := ValidateAPISecret(k); err != nil {
			t.Fatalf("ValidateAPISecret(%q) = %v, want nil", k, err)
		}
	}
	for _, k := range bad {
		var le *LoginError
		if err := ValidateAPIKey(k); !errors.As(err, &le) || le.Code != CodeInvalidCredentials {
			t.Fatalf("ValidateAPIKey(%q) = %v, want invalid_credentials", k, err)
		}
		if err := ValidateAPISecret(k); !errors.As(err, &le) || le.Code != CodeInvalidCredentials {
			t.Fatalf("ValidateAPISecret(%q) = %v, want invalid_credentials", k, err)
		}
	}
}

func TestValidatePort(t *testing.T) {
	for _, p := range []int{1024, 7420, 65535} {
		if err := ValidatePort(p); err != nil {
			t.Fatalf("port %d: %v", p, err)
		}
	}
	for _, p := range []int{0, 80, 1023, 65536} {
		if ValidatePort(p) == nil {
			t.Fatalf("port %d accepted", p)
		}
	}
}

func TestRedirectURLAndMask(t *testing.T) {
	if got := RedirectURL(DefaultRedirectPort); got != "http://127.0.0.1:7420/kite/callback" {
		t.Fatalf("RedirectURL = %q", got)
	}
	if got := MaskKey("kitekey1234abcd"); got != "••••abcd" {
		t.Fatalf("MaskKey = %q", got)
	}
	if got := MaskKey(""); got != "" {
		t.Fatalf("MaskKey(empty) = %q", got)
	}
}
```

- [ ] **Step 6: Run the credentials test to verify it fails**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestNormalizeCredential|TestValidate|TestRedirectURLAndMask' -v`
Expected: FAIL to compile with `undefined: NormalizeCredential`.

- [ ] **Step 7: Write the implementation**

```go
// internal/kite/credentials.go
package kite

import (
	"fmt"
	"regexp"
	"strings"

	"tradeloom/internal/settings"
)

// CallbackPath is the loopback redirect path registered in the Kite developer console (ARCH §5.2).
const CallbackPath = "/kite/callback"

// DefaultRedirectPort is the default loopback port (Q-A3 decided: 7420, user-configurable).
const DefaultRedirectPort = 7420

var credRe = regexp.MustCompile(`^[A-Za-z0-9]{6,64}$`)

// Settings keys of this package (P0-1 Key[T] literals, registered for the generic façade of Task 17).
var (
	SettingAPIKey = settings.Register(settings.Key[string]{Name: "kite.api_key", Default: "", Validate: func(v string) error {
		if v == "" {
			return nil
		}
		return ValidateAPIKey(v)
	}})
	SettingRedirectPort = settings.Register(settings.Key[int]{Name: "kite.redirect_port", Default: DefaultRedirectPort, Validate: ValidatePort})
)

// NormalizeCredential removes whitespace, newlines and quotes picked up when copying from a web page.
func NormalizeCredential(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	return strings.TrimSpace(s)
}

func ValidateAPIKey(k string) error {
	if !credRe.MatchString(k) {
		return &LoginError{Code: CodeInvalidCredentials, Message: "The API key should be 6–64 letters and digits. Copy it from your app in the Kite developer console."}
	}
	return nil
}

func ValidateAPISecret(s string) error {
	if !credRe.MatchString(s) {
		return &LoginError{Code: CodeInvalidCredentials, Message: "The API secret should be 6–64 letters and digits. Copy it from your app in the Kite developer console."}
	}
	return nil
}

func ValidatePort(p int) error {
	if p < 1024 || p > 65535 {
		return fmt.Errorf("redirect port must be between 1024 and 65535, got %d", p)
	}
	return nil
}

// RedirectURL is the exact URL the user registers as the app's redirect URL (FR-ACCT-005).
func RedirectURL(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", port, CallbackPath)
}

// MaskKey shows only the last four characters of the API key (FR-ACCT-005).
func MaskKey(k string) string {
	if len(k) <= 4 {
		return strings.Repeat("•", len(k))
	}
	return "••••" + k[len(k)-4:]
}

// CredentialsInfo is what Settings → Kite connection shows. It never carries the secret.
type CredentialsInfo struct {
	Configured    bool   `json:"configured"`
	APIKeyMasked  string `json:"apiKeyMasked"`
	Port          int    `json:"port"`
	RedirectURL   string `json:"redirectUrl"`
	SecretBackend string `json:"secretBackend"`
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders ./internal/kite/ -run 'TestNormalizeCredential|TestValidate|TestRedirectURLAndMask' -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/settings/registry.go internal/settings/registry_test.go internal/kite/credentials.go internal/kite/credentials_test.go
git commit -m "feat(settings,kite): key registry, credential validation, redirect URL and settings keys

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
