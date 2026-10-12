> **P0-3 Session & Service Layer Implementation Plan — Task 27.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 27: Secret storage setup, unlock and migration (FR-ACCT-013)

**Execution order:** Tasks 27–32 run after Task 21 and before Task 22 (see Global Constraints).

**Files:**
- Create: `internal/service/secrets.go`
- Modify: `internal/service/deps.go` (Task 14: five `Deps` fields), `internal/service/session.go` (Task 14: `SessionStatus.Secrets`, the `locked` state, deferred `Restore`), `internal/app/eventsgen/main_test.go` (Task 21: one more expected line)
- Test: `internal/service/secrets_test.go`

**Interfaces:**
- Consumes: P0-1 `secrets.Store`, `secrets.Key`, `KeyAPISecret`, `ErrNotFound`, `ErrStorageRefused`, `ErrWrongPassphrase`, `ErrPassphraseTooShort`, `ErrCorruptFile`, `ErrFileExists`, `MinPassphraseRunes`, `DefaultKDF` (≈1 s per `CreateEncFile`/`OpenEncFile`), `secrets.Mode`/`ModeKeyring`/`ModeFile`/`ModeChoose`, `secrets.Choice`/`ChoiceEncryptedFile`/`ChoiceNoStorage`, `secrets.Detection`, `secrets.NewNone`, `secrets.NewKeyring`, `secrets.CreateEncFile`, `secrets.OpenEncFile`, `(*EncFile).Close`, `secrets.Migrate`, `BackendKeyring`/`BackendFile`/`BackendNone`; `logging.RegisterSecret`; Task 6 `SessionDeps.Secrets` (the switch is the Session's store); Task 14 `statusFrom`, `Restore`, `fakeSession`.
- Produces:
  ```go
  var ErrLocked error // Get/Set/Delete while the encrypted file is not unlocked
  type SecretSwitch struct{…} // implements secrets.Store; the Session holds it for the whole process
  func NewSecretSwitch(initial secrets.Store, locked bool) *SecretSwitch
  func (w *SecretSwitch) Get(k secrets.Key) (string, error); Set(k secrets.Key, v string) error; Delete(k secrets.Key) error; Backend() string // "locked" while locked
  func (w *SecretSwitch) Locked() bool; func (w *SecretSwitch) Refused() bool; func (w *SecretSwitch) Current() secrets.Store
  func (w *SecretSwitch) Swap(st secrets.Store) // installs st, clears locked/refused, closes a previous *EncFile
  func (w *SecretSwitch) Refuse()               // NewNone + refused (the user chose "no storage")
  type SecretStorageInfo struct{ Mode, Backend string; KeyringAvailable, FileExists bool; Choices []string } // TS service.SecretStorageInfo
  // Deps gains: Secrets *SecretSwitch; SecretsFile string; Detection secrets.Detection; KeyringProbe func() error; NewKeyring func() secrets.Store
  // SessionStatus gains: Secrets SecretStorageInfo `json:"secrets"`; State is "locked" while the switch is locked; Restore keeps its Task 14 signature and returns the locked status without touching the Session
  func (s *Service) SecretStorage(ctx context.Context) SecretStorageInfo
  func (s *Service) UseEncryptedFile(ctx context.Context, passphrase string) (SessionStatus, error) // passphrase_too_short, file_exists, already_configured
  func (s *Service) RefuseSecretStorage(ctx context.Context) (SessionStatus, error)                 // already_configured
  func (s *Service) Unlock(ctx context.Context, passphrase string) (SessionStatus, error)           // bad_passphrase, not_locked
  func (s *Service) MigrateToKeyring(ctx context.Context) (SecretStorageInfo, error)                // not_file_backend, keyring_unavailable, migrate_failed
  func (s *Service) Restore(ctx context.Context) SessionStatus
  ```

P0-1 detects the backend before the window exists (`Boot.Secrets`): a usable keyring, an existing `secrets.enc`, or a choice to make. The `SecretSwitch` turns that detection into something the running app can change: the Session always talks to the switch, and the setup/unlock/migration bindings swap the backend behind it. While an encrypted file exists but has not been opened this start, every secret access returns `ErrLocked`, `SessionStatus.State` is `locked`, and the startup `Restore` is deferred until `Unlock` succeeds (otherwise the Session would read "no credentials" from a store it cannot open). `Mode` reports what the user sees in setup: `keyring`, `file`, `choose` (nothing chosen yet; `Choices` = `encrypted_file`, `no_storage`) or `none` (the user refused storage). Passphrases are registered with the redactor before use so they never appear in a log line.

- [ ] **Step 1: Write the failing tests**

```go
// internal/service/secrets_test.go
package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"tradeloom/internal/kite"
	"tradeloom/internal/secrets"
)

type memStore struct {
	mu      sync.Mutex
	m       map[secrets.Key]string
	backend string
}

func (s *memStore) Get(k secrets.Key) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.m[k]; ok {
		return v, nil
	}
	return "", secrets.ErrNotFound
}
func (s *memStore) Set(k secrets.Key, v string) error { s.mu.Lock(); defer s.mu.Unlock(); s.m[k] = v; return nil }
func (s *memStore) Delete(k secrets.Key) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.m[k]; !ok {
		return secrets.ErrNotFound
	}
	delete(s.m, k)
	return nil
}
func (s *memStore) Backend() string { return s.backend }

// withSecrets installs a switch, a secrets file path in a temp dir and the detection on an env.
func withSecrets(t *testing.T, e *svcEnv, initial secrets.Store, locked bool, mode secrets.Mode) (*SecretSwitch, string) {
	t.Helper()
	sw := NewSecretSwitch(initial, locked)
	file := filepath.Join(t.TempDir(), "secrets.enc")
	det := secrets.Detection{Mode: mode}
	if mode == secrets.ModeChoose {
		det.Choices = []secrets.Choice{secrets.ChoiceEncryptedFile, secrets.ChoiceNoStorage}
	}
	e.svc.d.Secrets, e.svc.d.SecretsFile, e.svc.d.Detection = sw, file, det
	return sw, file
}

const passphrase = "correct horse battery staple"

// FR-ACCT-013: choose → encrypted file; after a restart the file is locked until the passphrase unlocks it.
// Each CreateEncFile/OpenEncFile runs argon2id with DefaultKDF (≈1 s); this test makes four such calls.
func TestSecretStorage_EncryptedFileSetupAndUnlock(t *testing.T) {
	e := newSvcEnv(t)
	e.session.setState(kite.SessionState{State: kite.StateNoCredentials})
	sw, file := withSecrets(t, e, secrets.NewNone(), false, secrets.ModeChoose)
	ctx := context.Background()
	info := e.svc.SecretStorage(ctx)
	if info.Mode != "choose" || info.Backend != "none" || info.FileExists || len(info.Choices) != 2 || info.Choices[0] != "encrypted_file" || info.Choices[1] != "no_storage" {
		t.Fatalf("%+v", info)
	}
	if st := e.svc.SessionStatus(ctx); st.State != "no_credentials" || st.Secrets.Mode != "choose" {
		t.Fatalf("%+v", st)
	}
	if _, err := e.svc.UseEncryptedFile(ctx, "short"); refusedCode(t, err) != "passphrase_too_short" {
		t.Fatal(err)
	}
	st, err := e.svc.UseEncryptedFile(ctx, passphrase)
	if err != nil || st.State != "no_credentials" || st.Secrets.Mode != "file" || st.Secrets.Backend != "file" || !st.Secrets.FileExists {
		t.Fatalf("%+v %v", st, err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("secrets file not created:", err)
	}
	if err := sw.Set(secrets.KeyAPISecret, "s3cr3t"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UseEncryptedFile(ctx, passphrase); refusedCode(t, err) != "already_configured" {
		t.Fatal(err)
	}
	if _, err := e.svc.Unlock(ctx, passphrase); refusedCode(t, err) != "not_locked" {
		t.Fatal(err)
	}

	// Restart: P0-1 detects the file and the app starts locked.
	e2 := newSvcEnv(t)
	e2.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	sw2 := NewSecretSwitch(secrets.NewNone(), true)
	e2.svc.d.Secrets, e2.svc.d.SecretsFile, e2.svc.d.Detection = sw2, file, secrets.Detection{Mode: secrets.ModeFile, FileExists: true}
	if st := e2.svc.Restore(ctx); st.State != "locked" || st.Secrets.Backend != "locked" || st.Secrets.Mode != "file" {
		t.Fatalf("locked start %+v", st)
	}
	if _, err := sw2.Get(secrets.KeyAPISecret); !errors.Is(err, ErrLocked) {
		t.Fatalf("locked get = %v", err)
	}
	if _, err := e2.svc.Unlock(ctx, "wrong passphrase"); refusedCode(t, err) != "bad_passphrase" {
		t.Fatal(err)
	}
	st, err = e2.svc.Unlock(ctx, passphrase)
	if err != nil || st.State != "logged_out" || st.Secrets.Backend != "file" {
		t.Fatalf("%+v %v", st, err)
	}
	if v, err := sw2.Get(secrets.KeyAPISecret); err != nil || v != "s3cr3t" {
		t.Fatalf("after unlock %q %v", v, err)
	}
}

func TestSecretStorage_RefuseStorage(t *testing.T) {
	e := newSvcEnv(t)
	sw, _ := withSecrets(t, e, secrets.NewNone(), false, secrets.ModeChoose)
	ctx := context.Background()
	st, err := e.svc.RefuseSecretStorage(ctx)
	if err != nil || st.Secrets.Mode != "none" || st.Secrets.Backend != "none" || len(st.Secrets.Choices) != 0 {
		t.Fatalf("%+v %v", st, err)
	}
	if err := sw.Set(secrets.KeyAPISecret, "x"); !errors.Is(err, secrets.ErrStorageRefused) {
		t.Fatalf("set after refusal = %v", err)
	}
	if _, err := e.svc.RefuseSecretStorage(ctx); err != nil {
		t.Fatalf("refusing twice is idempotent: %v", err)
	}
	e.svc.d.Secrets = NewSecretSwitch(&memStore{m: map[secrets.Key]string{}, backend: secrets.BackendKeyring}, false)
	if _, err := e.svc.RefuseSecretStorage(ctx); refusedCode(t, err) != "already_configured" {
		t.Fatal(err)
	}
	if info := e.svc.SecretStorage(ctx); info.Mode != "keyring" || !info.KeyringAvailable {
		t.Fatalf("%+v", info)
	}
}

// FR-ACCT-013 "move to keyring": the file's secrets are copied, verified, and the file is removed.
func TestMigrateToKeyring(t *testing.T) {
	e := newSvcEnv(t)
	sw, file := withSecrets(t, e, secrets.NewNone(), false, secrets.ModeChoose)
	ctx := context.Background()
	if _, err := e.svc.MigrateToKeyring(ctx); refusedCode(t, err) != "not_file_backend" {
		t.Fatal(err)
	}
	if _, err := e.svc.UseEncryptedFile(ctx, passphrase); err != nil {
		t.Fatal(err)
	}
	if err := sw.Set(secrets.KeyAPISecret, "s3cr3t"); err != nil {
		t.Fatal(err)
	}
	e.svc.d.KeyringProbe = func() error { return errors.New("no secret service on the session bus") }
	if _, err := e.svc.MigrateToKeyring(ctx); refusedCode(t, err) != "keyring_unavailable" {
		t.Fatal(err)
	}
	kr := &memStore{m: map[secrets.Key]string{}, backend: secrets.BackendKeyring}
	e.svc.d.KeyringProbe = func() error { return nil }
	e.svc.d.NewKeyring = func() secrets.Store { return kr }
	info, err := e.svc.MigrateToKeyring(ctx)
	if err != nil || info.Mode != "keyring" || info.Backend != "keyring" || !info.KeyringAvailable || info.FileExists {
		t.Fatalf("%+v %v", info, err)
	}
	if kr.m[secrets.KeyAPISecret] != "s3cr3t" {
		t.Fatalf("keyring %v", kr.m)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("secrets file still present: %v", err)
	}
	if v, err := sw.Get(secrets.KeyAPISecret); err != nil || v != "s3cr3t" {
		t.Fatalf("switch after migration %q %v", v, err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run 'TestSecretStorage|TestMigrateToKeyring' -v`
Expected: FAIL to compile with `undefined: NewSecretSwitch`, `e.svc.d.Secrets undefined`.

- [ ] **Step 3: Extend `deps.go`, `session.go` and the generator test**

In `deps.go` add to `Deps` (after `Tracker`) and the import `"tradeloom/internal/secrets"`:

```go
	Secrets      *SecretSwitch      // the Session's secrets.Store; swapped by setup, unlock and migration (Task 27)
	SecretsFile  string             // Layout.SecretsFile
	Detection    secrets.Detection  // Boot.Secrets
	KeyringProbe func() error       // secrets.ProbeKeyringWithin(secrets.ProbeTimeout); nil = available
	NewKeyring   func() secrets.Store // secrets.NewKeyring; tests inject a memory store
```

In `session.go` replace `SessionStatus`, `statusFrom` and `Restore` with:

```go
// SessionStatus is the payload of Session.Status() and of the "session.state" event.
type SessionStatus struct {
	State           string            `json:"state"`
	SessionID       string            `json:"sessionId"`
	UserID          string            `json:"userId"`
	LoggedInAtMs    int64             `json:"loggedInAtMs"`
	ExpiresAtMs     int64             `json:"expiresAtMs"`
	DataPlan        string            `json:"dataPlan"`
	Offline         bool              `json:"offline"`
	RiskAckRequired bool              `json:"riskAckRequired"`
	ErrorCode       string            `json:"errorCode"`
	ErrorMessage    string            `json:"errorMessage"`
	Credentials     CredentialsInfo   `json:"credentials"`
	Secrets         SecretStorageInfo `json:"secrets"`
}

// StateLocked is reported while an encrypted secrets file exists but has not been unlocked this start.
const StateLocked = "locked"

func (s *Service) statusFrom(st kite.SessionState) SessionStatus {
	out := SessionStatus{
		State: string(st.State), SessionID: st.SessionID, UserID: st.UserID, LoggedInAtMs: st.LoggedInAtMs,
		ExpiresAtMs: st.ExpiresAtMs, DataPlan: string(st.DataPlan), Offline: st.Offline, RiskAckRequired: st.RiskAckRequired,
		ErrorCode: st.ErrorCode, ErrorMessage: st.ErrorMessage, Credentials: s.d.Session.CredentialsInfo(), Secrets: s.secretInfo(),
	}
	if s.d.Secrets != nil && s.d.Secrets.Locked() {
		out.State = StateLocked
	}
	return out
}

// Restore resumes the session at startup, unless the secrets file is still locked (Unlock restores later).
func (s *Service) Restore(ctx context.Context) SessionStatus {
	if s.d.Secrets != nil && s.d.Secrets.Locked() {
		return s.SessionStatus(ctx)
	}
	return s.statusFrom(s.d.Session.Restore(ctx))
}
```

In `internal/app/eventsgen/main_test.go` add `"  secrets: service_SecretStorageInfo;",` after the `credentials` line of `TestGenerate`'s list.

- [ ] **Step 4: Write the implementation**

```go
// internal/service/secrets.go
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"tradeloom/internal/logging"
	"tradeloom/internal/secrets"
)

const secretsStage = "secrets"

// ErrLocked is returned by a SecretSwitch while the encrypted secrets file has not been unlocked this start.
var ErrLocked = errors.New("secrets: encrypted secrets file is locked")

// SecretSwitch is a secrets.Store whose backend changes at runtime (FR-ACCT-013). The Session keeps one Store
// for the whole process while setup, unlock and migration swap what is behind it.
type SecretSwitch struct {
	mu      sync.Mutex
	store   secrets.Store
	locked  bool
	refused bool
}

func NewSecretSwitch(initial secrets.Store, locked bool) *SecretSwitch {
	if initial == nil {
		initial = secrets.NewNone()
	}
	return &SecretSwitch{store: initial, locked: locked}
}

func (w *SecretSwitch) current() (secrets.Store, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.store, w.locked
}

func (w *SecretSwitch) Get(k secrets.Key) (string, error) {
	st, locked := w.current()
	if locked {
		return "", ErrLocked
	}
	return st.Get(k)
}

func (w *SecretSwitch) Set(k secrets.Key, v string) error {
	st, locked := w.current()
	if locked {
		return ErrLocked
	}
	return st.Set(k, v)
}

func (w *SecretSwitch) Delete(k secrets.Key) error {
	st, locked := w.current()
	if locked {
		return ErrLocked
	}
	return st.Delete(k)
}

// Backend is the current backend name, or "locked" while the file waits for its passphrase.
func (w *SecretSwitch) Backend() string {
	st, locked := w.current()
	if locked {
		return "locked"
	}
	return st.Backend()
}

func (w *SecretSwitch) Locked() bool {
	_, locked := w.current()
	return locked
}

func (w *SecretSwitch) Refused() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.refused
}

// Current returns the backend itself (for secrets.Migrate).
func (w *SecretSwitch) Current() secrets.Store {
	st, _ := w.current()
	return st
}

// Swap installs st as the backend and clears the locked and refused flags. A previous *EncFile is closed.
func (w *SecretSwitch) Swap(st secrets.Store) {
	w.mu.Lock()
	prev := w.store
	w.store, w.locked, w.refused = st, false, false
	w.mu.Unlock()
	if ef, ok := prev.(*secrets.EncFile); ok && prev != st {
		ef.Close()
	}
}

// Refuse records the user's choice to store nothing (mode "none").
func (w *SecretSwitch) Refuse() {
	w.Swap(secrets.NewNone())
	w.mu.Lock()
	w.refused = true
	w.mu.Unlock()
}

// SecretStorageInfo is what the setup view and Settings → Kite connection show (FR-ACCT-013).
type SecretStorageInfo struct {
	Mode             string   `json:"mode"`    // keyring | file | choose | none
	Backend          string   `json:"backend"` // keyring | file | none | locked
	KeyringAvailable bool     `json:"keyringAvailable"`
	FileExists       bool     `json:"fileExists"`
	Choices          []string `json:"choices"` // encrypted_file, no_storage in mode choose; empty otherwise
}

func (s *Service) secretsFileExists() bool {
	if s.d.SecretsFile == "" {
		return false
	}
	_, err := os.Stat(s.d.SecretsFile)
	return err == nil
}

func (s *Service) secretInfo() SecretStorageInfo {
	info := SecretStorageInfo{Mode: "none", Backend: secrets.BackendNone, KeyringAvailable: s.d.Detection.KeyringAvailable,
		FileExists: s.secretsFileExists(), Choices: []string{}}
	sw := s.d.Secrets
	if sw == nil {
		return info
	}
	info.Backend = sw.Backend()
	switch {
	case sw.Locked():
		info.Mode = string(secrets.ModeFile)
	case info.Backend == secrets.BackendKeyring:
		info.Mode, info.KeyringAvailable = string(secrets.ModeKeyring), true
	case info.Backend == secrets.BackendFile:
		info.Mode = string(secrets.ModeFile)
	case sw.Refused():
		info.Mode = "none"
	default:
		info.Mode = string(secrets.ModeChoose)
		info.Choices = []string{string(secrets.ChoiceEncryptedFile), string(secrets.ChoiceNoStorage)}
	}
	return info
}

func (s *Service) SecretStorage(_ context.Context) SecretStorageInfo { return s.secretInfo() }

func (s *Service) secretsConfigured() (string, bool) {
	sw := s.d.Secrets
	if sw == nil {
		return "", false
	}
	b := sw.Backend()
	return b, sw.Locked() || b == secrets.BackendKeyring || b == secrets.BackendFile
}

// UseEncryptedFile creates secrets.enc with the passphrase and makes it the backend (setup choice "encrypted file").
func (s *Service) UseEncryptedFile(ctx context.Context, passphrase string) (SessionStatus, error) {
	if s.d.Secrets == nil || s.d.SecretsFile == "" {
		return s.SessionStatus(ctx), refused(secretsStage, "unavailable", "Secret storage is not configured for this build.")
	}
	if b, done := s.secretsConfigured(); done {
		return s.SessionStatus(ctx), refused(secretsStage, "already_configured", "Secret storage is already set up ("+b+").")
	}
	logging.RegisterSecret(passphrase)
	ef, err := secrets.CreateEncFile(s.d.SecretsFile, []byte(passphrase))
	switch {
	case errors.Is(err, secrets.ErrPassphraseTooShort):
		return s.SessionStatus(ctx), refused(secretsStage, "passphrase_too_short", fmt.Sprintf("Use a passphrase of at least %d characters.", secrets.MinPassphraseRunes))
	case errors.Is(err, secrets.ErrFileExists):
		return s.SessionStatus(ctx), refused(secretsStage, "file_exists", "A secrets file already exists; unlock it with its passphrase instead.")
	case err != nil:
		return s.SessionStatus(ctx), asRefused(secretsStage, err)
	}
	s.d.Secrets.Swap(ef)
	s.d.Log.Info("secret storage set up", "backend", secrets.BackendFile)
	return s.Restore(ctx), nil
}

// RefuseSecretStorage records the setup choice "no storage": credentials are kept in memory for this run only.
func (s *Service) RefuseSecretStorage(ctx context.Context) (SessionStatus, error) {
	if s.d.Secrets == nil {
		return s.SessionStatus(ctx), refused(secretsStage, "unavailable", "Secret storage is not configured for this build.")
	}
	if b, done := s.secretsConfigured(); done {
		return s.SessionStatus(ctx), refused(secretsStage, "already_configured", "Secret storage is already set up ("+b+").")
	}
	s.d.Secrets.Refuse()
	s.d.Log.Info("secret storage refused by the user")
	return s.SessionStatus(ctx), nil
}

// Unlock opens secrets.enc with the passphrase and then runs the deferred startup Restore.
func (s *Service) Unlock(ctx context.Context, passphrase string) (SessionStatus, error) {
	if s.d.Secrets == nil || !s.d.Secrets.Locked() {
		return s.SessionStatus(ctx), refused(secretsStage, "not_locked", "The secrets file is not locked.")
	}
	logging.RegisterSecret(passphrase)
	ef, err := secrets.OpenEncFile(s.d.SecretsFile, []byte(passphrase))
	switch {
	case errors.Is(err, secrets.ErrWrongPassphrase), errors.Is(err, secrets.ErrCorruptFile), errors.Is(err, secrets.ErrPassphraseTooShort):
		return s.SessionStatus(ctx), refused(secretsStage, "bad_passphrase", "The passphrase is wrong, or the secrets file is damaged.")
	case err != nil:
		return s.SessionStatus(ctx), asRefused(secretsStage, err)
	}
	s.d.Secrets.Swap(ef)
	s.d.Log.Info("secrets file unlocked")
	return s.Restore(ctx), nil
}

// MigrateToKeyring moves every secret from the encrypted file into the system keyring and removes the file.
func (s *Service) MigrateToKeyring(ctx context.Context) (SecretStorageInfo, error) {
	sw := s.d.Secrets
	if sw == nil || sw.Backend() != secrets.BackendFile {
		return s.secretInfo(), refused(secretsStage, "not_file_backend", "Only the encrypted-file backend can be moved to the keyring.")
	}
	if s.d.KeyringProbe != nil {
		if err := s.d.KeyringProbe(); err != nil {
			return s.secretInfo(), refused(secretsStage, "keyring_unavailable", "No system keyring is available: "+err.Error())
		}
	}
	newKeyring := s.d.NewKeyring
	if newKeyring == nil {
		newKeyring = secrets.NewKeyring
	}
	kr := newKeyring()
	if err := secrets.Migrate(sw.Current(), kr); err != nil {
		return s.secretInfo(), refused(secretsStage, "migrate_failed", "The secrets could not be moved to the keyring: "+err.Error())
	}
	sw.Swap(kr)
	s.d.Log.Info("secrets moved to the system keyring")
	return s.secretInfo(), nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ ./internal/app/eventsgen/ -run 'TestSecretStorage|TestMigrateToKeyring|TestSessionStatus|TestForwardNow_SessionState|TestGenerate' -v`
Expected: PASS (`TestSecretStorage_EncryptedFileSetupAndUnlock` in about 4–6 s because of four argon2id derivations, `TestSecretStorage_RefuseStorage`, `TestMigrateToKeyring` in about 1–2 s, `TestSessionStatus_MergesCredentials`, `TestForwardNow_SessionStateEveryTransition`, `TestGenerate`).

- [ ] **Step 6: Commit**

```bash
git add internal/service/secrets.go internal/service/secrets_test.go internal/service/deps.go internal/service/session.go internal/app/eventsgen/main_test.go
git commit -m "feat(service): secret storage switch with encrypted-file setup, unlock and keyring migration

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
