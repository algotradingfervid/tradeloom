> **P0-1 Platform Core Implementation Plan — Task 12.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 12: `internal/secrets` — Store interface, keyring backend, probe, `none` backend

**Files:**
- Create: `internal/secrets/secrets.go`, `internal/secrets/keyring.go`, `internal/secrets/keyring_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: Task 4 `logging.RegisterSecret`; Task 5 `logging.Go`.
- Produces (`package secrets`):
  - `type Key string`; `const KeyAPISecret Key = "kite.api_secret"`, `KeyAccessToken Key = "kite.access_token"`, `KeyTelegramToken Key = "telegram.bot_token"`; `var AllKeys = []Key{KeyAPISecret, KeyAccessToken, KeyTelegramToken}`
  - `type Store interface { Get(Key) (string, error); Set(Key, string) error; Delete(Key) error; Backend() string }`; `const BackendKeyring = "keyring"`, `BackendFile = "file"`, `BackendNone = "none"`
  - Errors: `ErrNotFound`, `ErrUnavailable`, `ErrStorageRefused`, `ErrWrongPassphrase`, `ErrPassphraseTooShort`, `ErrCorruptFile`, `ErrFileExists`
  - `const KeyringService = "tradeloom"`; `func ProbeKeyring() error`; `const ProbeTimeout = 3 * time.Second`; `func ProbeKeyringWithin(d time.Duration) error`; `func NewKeyring() Store`; `func NewNone() Store`
  - Every value read or written is passed to `logging.RegisterSecret`, so the log redactor removes it from every later log line (FR-SHELL-018 "any value read from the keyring").

- [ ] **Step 1: Write the failing test**

`internal/secrets/keyring_test.go` (uses go-keyring's in-memory `MockInit`/`MockInitWithError`, verified in v0.2.8 `keyring_mock.go`):

```go
package secrets

import (
	"errors"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestProbeKeyring(t *testing.T) {
	keyring.MockInit()
	if err := ProbeKeyring(); err != nil {
		t.Fatalf("daemon present: %v", err)
	}
	keyring.MockInitWithError(errors.New("dbus: The name org.freedesktop.secrets was not provided by any .service files"))
	if err := ProbeKeyring(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no daemon: want ErrUnavailable, got %v", err)
	}
}

func TestKeyringStore_RoundTrip(t *testing.T) {
	keyring.MockInit()
	s := NewKeyring()
	if s.Backend() != BackendKeyring {
		t.Fatal(s.Backend())
	}
	if _, err := s.Get(KeyAPISecret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.Set(KeyAPISecret, "s3cret-value"); err != nil {
		t.Fatal(err)
	}
	if v, err := s.Get(KeyAPISecret); err != nil || v != "s3cret-value" {
		t.Fatalf("%q %v", v, err)
	}
	if err := s.Delete(KeyAPISecret); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(KeyAPISecret); err != nil {
		t.Fatalf("deleting a missing key must be a no-op: %v", err)
	}
}

// Review Focus 2: the daemon disappears mid-session → every call fails with ErrUnavailable;
// there is no silent fallback.
func TestKeyringStore_DaemonGoneMidSession(t *testing.T) {
	keyring.MockInit()
	s := NewKeyring()
	if err := s.Set(KeyAccessToken, "tok-123456"); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("dbus: connection closed by user"))
	if err := s.Set(KeyAccessToken, "tok-654321"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Set: want ErrUnavailable, got %v", err)
	}
	if _, err := s.Get(KeyAccessToken); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get: want ErrUnavailable, got %v", err)
	}
	if err := s.Delete(KeyAccessToken); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Delete: want ErrUnavailable, got %v", err)
	}
	if s.Backend() != BackendKeyring {
		t.Fatal("backend changed silently")
	}
}

func TestNone(t *testing.T) {
	s := NewNone()
	if err := s.Set(KeyAPISecret, "x"); !errors.Is(err, ErrStorageRefused) {
		t.Fatal(err)
	}
	if _, err := s.Get(KeyAPISecret); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if s.Backend() != BackendNone || s.Delete(KeyAPISecret) != nil {
		t.Fatal("none backend")
	}
}

// S0-4 / Review Focus 2: a probe that never answers (locked collection, prompt with no display)
// must not hang startup.
func TestProbeWithin_Timeout(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	start := time.Now()
	err := probeWithin(func() error { <-block; return nil }, 50*time.Millisecond)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("probe waited %v", d)
	}
	if err := probeWithin(func() error { return nil }, time.Second); err != nil {
		t.Fatalf("fast probe: %v", err)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders ./internal/secrets/`
Expected: FAIL — `no required module provides package github.com/zalando/go-keyring`; after Step 3's `go get` and before the source files exist: `undefined: ProbeKeyring`, `undefined: ErrUnavailable`, `undefined: NewKeyring`.

- [ ] **Step 3: Write the implementation**

```bash
go get github.com/zalando/go-keyring@v0.2.8
```

`internal/secrets/secrets.go`:

```go
// Package secrets stores the Kite API secret, the Kite access token and the Telegram bot token
// (NFR-04, FR-ACCT-013, ARCHITECTURE §9.1). Backends: the OS keyring (Secret Service via
// zalando/go-keyring), an encrypted file (Argon2id + XChaCha20-Poly1305), or none.
// There is no plaintext backend.
package secrets

import "errors"

// Key names one secret.
type Key string

const (
	KeyAPISecret     Key = "kite.api_secret"
	KeyAccessToken   Key = "kite.access_token"
	KeyTelegramToken Key = "telegram.bot_token"
)

// AllKeys lists every secret the app stores; migration copies exactly these.
var AllKeys = []Key{KeyAPISecret, KeyAccessToken, KeyTelegramToken}

// Backend names returned by Store.Backend (shown in Settings → Kite connection).
const (
	BackendKeyring = "keyring"
	BackendFile    = "file"
	BackendNone    = "none"
)

// Store is the secrets swap point.
type Store interface {
	Get(Key) (string, error)
	Set(Key, string) error
	Delete(Key) error
	Backend() string
}

var (
	// ErrNotFound: the key has no value in this backend.
	ErrNotFound = errors.New("secrets: not found")
	// ErrUnavailable: the keyring daemon (Secret Service) cannot be reached.
	ErrUnavailable = errors.New("secrets: keyring unavailable")
	// ErrStorageRefused: the user chose not to store credentials.
	ErrStorageRefused = errors.New("secrets: credential storage refused by the user")
	// ErrWrongPassphrase: secrets.enc does not decrypt (wrong passphrase or modified file).
	ErrWrongPassphrase = errors.New("secrets: wrong passphrase or damaged secrets file")
	// ErrPassphraseTooShort: fewer than MinPassphraseRunes characters.
	ErrPassphraseTooShort = errors.New("secrets: passphrase must have at least 8 characters")
	// ErrCorruptFile: secrets.enc is not a Tradeloom secrets file (bad header or truncated).
	ErrCorruptFile = errors.New("secrets: secrets file is not readable")
	// ErrFileExists: CreateEncFile refuses to overwrite an existing secrets.enc.
	ErrFileExists = errors.New("secrets: secrets file already exists")
)

// none is the "refuse to store credentials" backend (FR-ACCT-013 choice b).
type none struct{}

// NewNone returns the backend used when the user refuses credential storage.
func NewNone() Store { return none{} }

func (none) Get(Key) (string, error) { return "", ErrNotFound }
func (none) Set(Key, string) error   { return ErrStorageRefused }
func (none) Delete(Key) error        { return nil }
func (none) Backend() string         { return BackendNone }
```

`internal/secrets/keyring.go`:

```go
package secrets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"

	"tradeloom/internal/logging"
)

// KeyringService is the Secret Service "service" attribute; the user attribute is the Key.
const KeyringService = "tradeloom"

const probeUser = "probe"

// ProbeKeyring returns nil when the Secret Service answers (a missing probe entry is an answer),
// or an error wrapping ErrUnavailable (no daemon, no session bus, locked/denied collection).
func ProbeKeyring() error {
	_, err := keyring.Get(KeyringService, probeUser)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

// ProbeTimeout bounds ProbeKeyringWithin: a locked collection can block on an unlock prompt that
// never appears in a headless session (S0-4).
const ProbeTimeout = 3 * time.Second

// ProbeKeyringWithin runs ProbeKeyring and reports ErrUnavailable if it has not answered within d.
func ProbeKeyringWithin(d time.Duration) error { return probeWithin(ProbeKeyring, d) }

func probeWithin(probe func() error, d time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	done := make(chan error, 1)
	logging.Go("secrets.probe", func() { done <- probe() })
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%w: no answer from the Secret Service within %v", ErrUnavailable, d)
	}
}

type keyringStore struct{}

// NewKeyring returns the OS keyring backend.
func NewKeyring() Store { return keyringStore{} }

func (keyringStore) Backend() string { return BackendKeyring }

func wrapKeyring(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, keyring.ErrNotFound):
		return ErrNotFound
	default:
		// No fallback of any kind: the caller surfaces the error (Review Focus 2).
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}

func (keyringStore) Get(k Key) (string, error) {
	v, err := keyring.Get(KeyringService, string(k))
	if err != nil {
		return "", wrapKeyring(err)
	}
	logging.RegisterSecret(v)
	return v, nil
}

func (keyringStore) Set(k Key, v string) error {
	logging.RegisterSecret(v)
	return wrapKeyring(keyring.Set(KeyringService, string(k), v))
}

func (keyringStore) Delete(k Key) error {
	err := wrapKeyring(keyring.Delete(KeyringService, string(k)))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go mod tidy && go test -race -count=1 -tags noorders ./internal/secrets/ ./internal/archtest/`
Expected: `ok` for both (rule 6: only `internal/secrets` imports go-keyring).

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/secrets
git commit -m "feat(secrets): Store interface, Secret Service keyring backend with bounded probe, no-storage backend" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
