> **P0-1 Platform Core Implementation Plan — Task 14.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 14: `internal/secrets` — backend detection and migration keyring ⇄ file

**Files:**
- Create: `internal/secrets/select.go`, `internal/secrets/select_test.go`

**Interfaces:**
- Consumes: Task 12 `ProbeKeyring`, `NewKeyring`, `Store`, `AllKeys`, errors; Task 13 `EncFile`, `CreateEncFile`, `Destroy`, test helper `fastKDF`.
- Produces (`package secrets`):
  - `type Mode string`: `ModeKeyring = "keyring"`, `ModeFile = "file"`, `ModeChoose = "choose"`
  - `type Choice string`: `ChoiceEncryptedFile = "encrypted_file"`, `ChoiceNoStorage = "no_storage"`
  - `type Detection struct { Mode Mode; KeyringAvailable bool; FileExists bool; ProbeError error; Choices []Choice }`
  - `func Detect(secretsFile string, probe func() error) Detection` — existing `secrets.enc` → `ModeFile`; else keyring answers → `ModeKeyring`; else `ModeChoose` with exactly `[ChoiceEncryptedFile, ChoiceNoStorage]`
  - `func Migrate(from, to Store) error` — copy + verify every present key, then delete from source (`*EncFile` source: file removed); on failure roll back the target and leave the source untouched
  - P0-3 uses: `ModeKeyring` → `NewKeyring()`; `ModeFile` → passphrase prompt → `OpenEncFile`; `ModeChoose` → setup shows exactly the two choices → `CreateEncFile` or `NewNone()`; Settings → Kite connection "Switch to keyring" (only when `KeyringAvailable`) → `Migrate(encFile, NewKeyring())`.

- [ ] **Step 1: Write the failing test**

`internal/secrets/select_test.go`:

```go
package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zalando/go-keyring"
)

// FR-ACCT-013 AC1 (engine side): no daemon → exactly two choices, none of them plaintext.
func TestDetect(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "secrets.enc")
	down := func() error { return ErrUnavailable }
	up := func() error { return nil }

	d := Detect(file, down)
	if d.Mode != ModeChoose || !reflect.DeepEqual(d.Choices, []Choice{ChoiceEncryptedFile, ChoiceNoStorage}) {
		t.Fatalf("no daemon: %+v", d)
	}
	if d := Detect(file, up); d.Mode != ModeKeyring || d.Choices != nil {
		t.Fatalf("daemon: %+v", d)
	}
	os.WriteFile(file, []byte("TLSEC1"), 0o600)
	if d := Detect(file, up); d.Mode != ModeFile || !d.KeyringAvailable {
		t.Fatalf("file + daemon: %+v", d)
	}
}

// FR-ACCT-013 AC3: switching to the keyring moves every secret and removes secrets.enc.
func TestMigrate_FileToKeyring(t *testing.T) {
	fastKDF(t)
	keyring.MockInit()
	path := filepath.Join(t.TempDir(), "secrets.enc")
	f, _ := CreateEncFile(path, []byte("passphrase-1"))
	f.Set(KeyAPISecret, "api-secret-1")
	f.Set(KeyAccessToken, "access-token-1")
	kr := NewKeyring()
	if err := Migrate(f, kr); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[Key]string{KeyAPISecret: "api-secret-1", KeyAccessToken: "access-token-1"} {
		if v, err := kr.Get(k); err != nil || v != want {
			t.Fatalf("%s: %q %v", k, v, err)
		}
	}
	if _, err := kr.Get(KeyTelegramToken); !errors.Is(err, ErrNotFound) {
		t.Fatal("absent key must stay absent")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("secrets.enc still exists")
	}
}

func TestMigrate_KeyringToFile(t *testing.T) {
	fastKDF(t)
	keyring.MockInit()
	kr := NewKeyring()
	kr.Set(KeyTelegramToken, "123456:telegram")
	f, _ := CreateEncFile(filepath.Join(t.TempDir(), "secrets.enc"), []byte("passphrase-1"))
	defer f.Close()
	if err := Migrate(kr, f); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.Get(KeyTelegramToken); v != "123456:telegram" {
		t.Fatal("not copied")
	}
	if _, err := kr.Get(KeyTelegramToken); !errors.Is(err, ErrNotFound) {
		t.Fatal("source not deleted")
	}
}

type failingStore struct {
	Store
	failOn Key
}

func (s failingStore) Set(k Key, v string) error {
	if k == s.failOn {
		return ErrUnavailable
	}
	return s.Store.Set(k, v)
}

func TestMigrate_FailureRollsBackTargetAndKeepsSource(t *testing.T) {
	fastKDF(t)
	keyring.MockInit()
	path := filepath.Join(t.TempDir(), "secrets.enc")
	f, _ := CreateEncFile(path, []byte("passphrase-1"))
	defer f.Close()
	f.Set(KeyAPISecret, "api-secret-1")
	f.Set(KeyAccessToken, "access-token-1")
	target := failingStore{Store: NewKeyring(), failOn: KeyAccessToken}
	if err := Migrate(f, target); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if _, err := NewKeyring().Get(KeyAPISecret); !errors.Is(err, ErrNotFound) {
		t.Fatal("partial copy not rolled back")
	}
	if v, _ := f.Get(KeyAPISecret); v != "api-secret-1" {
		t.Fatal("source changed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("source file removed after failed migration")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders ./internal/secrets/`
Expected: FAIL — build errors `undefined: Detect`, `undefined: ModeChoose`, `undefined: Migrate`.

- [ ] **Step 3: Write the implementation**

`internal/secrets/select.go`:

```go
package secrets

import (
	"errors"
	"fmt"
	"os"
)

// Mode is what the app must do at start to get a Store.
type Mode string

const (
	ModeKeyring Mode = "keyring" // use NewKeyring()
	ModeFile    Mode = "file"    // ask the passphrase, then OpenEncFile
	ModeChoose  Mode = "choose"  // no keyring and no file: offer exactly Choices
)

// Choice is one option of the no-daemon flow (FR-ACCT-013).
type Choice string

const (
	ChoiceEncryptedFile Choice = "encrypted_file"
	ChoiceNoStorage     Choice = "no_storage"
)

// Detection is the result of Detect.
type Detection struct {
	Mode             Mode
	KeyringAvailable bool
	FileExists       bool
	ProbeError       error    // why the keyring is unavailable (for the S0-4 log line and Settings)
	Choices          []Choice // non-empty only in ModeChoose; never contains a plaintext option
}

// Detect chooses the backend: an existing secrets.enc wins (the user chose it; switching to the
// keyring is an explicit action), else the keyring if probe succeeds, else the two-choice flow.
func Detect(secretsFile string, probe func() error) Detection {
	d := Detection{}
	if _, err := os.Stat(secretsFile); err == nil {
		d.FileExists = true
	}
	d.ProbeError = probe()
	d.KeyringAvailable = d.ProbeError == nil
	switch {
	case d.FileExists:
		d.Mode = ModeFile
	case d.KeyringAvailable:
		d.Mode = ModeKeyring
	default:
		d.Mode = ModeChoose
		d.Choices = []Choice{ChoiceEncryptedFile, ChoiceNoStorage}
	}
	return d
}

// Migrate copies every key of AllKeys present in from into to, verifies each by reading it back,
// then deletes it from from; when from is an *EncFile its file is removed (FR-ACCT-013 AC3).
// If any copy fails, keys already copied are removed from to and from is left untouched.
func Migrate(from, to Store) error {
	var copied []Key
	rollback := func(cause error) error {
		var errs []error
		for _, k := range copied {
			errs = append(errs, to.Delete(k))
		}
		return errors.Join(append([]error{cause}, errs...)...)
	}
	for _, k := range AllKeys {
		v, err := from.Get(k)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return rollback(fmt.Errorf("secrets: migrate read %s from %s: %w", k, from.Backend(), err))
		}
		if err := to.Set(k, v); err != nil {
			return rollback(fmt.Errorf("secrets: migrate write %s to %s: %w", k, to.Backend(), err))
		}
		copied = append(copied, k)
		if got, err := to.Get(k); err != nil || got != v {
			return rollback(fmt.Errorf("secrets: migrate verify %s in %s failed: %v", k, to.Backend(), err))
		}
	}
	if f, ok := from.(*EncFile); ok {
		return f.Destroy()
	}
	for _, k := range copied {
		if err := from.Delete(k); err != nil {
			return fmt.Errorf("secrets: migrated, but deleting %s from %s failed: %w", k, from.Backend(), err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -race -count=1 -tags noorders ./internal/secrets/`
Expected: `ok  tradeloom/internal/secrets`.

- [ ] **Step 5: Commit**

```bash
git add internal/secrets/select.go internal/secrets/select_test.go
git commit -m "feat(secrets): backend detection with the two no-daemon choices and keyring/file migration" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
