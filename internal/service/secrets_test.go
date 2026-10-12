// internal/service/secrets_test.go
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
func (s *memStore) Set(k secrets.Key, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}
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

// ---- Additional tests (P0-1 notes, Ruling 52, keyring bounds) ----

// lockedFile creates a secrets file holding the API secret and returns an env that starts locked over it.
func lockedFile(t *testing.T) (*svcEnv, *SecretSwitch, string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "secrets.enc")
	ef, err := secrets.CreateEncFile(file, []byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	if err := ef.Set(secrets.KeyAPISecret, "s3cr3t-value-abc"); err != nil {
		t.Fatal(err)
	}
	ef.Close()
	e := newSvcEnv(t)
	e.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	sw := NewSecretSwitch(secrets.NewNone(), true)
	e.svc.d.Secrets, e.svc.d.SecretsFile, e.svc.d.Detection = sw, file, secrets.Detection{Mode: secrets.ModeFile, FileExists: true}
	return e, sw, file
}

// P0-1 note: ErrWrongPassphrase also means a damaged file, so the copy says both. The passphrase never
// appears in the refusal or in a log line.
func TestUnlock_BadPassphrase_CopyAndNoLeak(t *testing.T) {
	e, _, file := lockedFile(t)
	var logs syncBuf
	e.svc.d.Log = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()
	const wrong = "not-the-right-passphrase-42"
	_, err := e.svc.Unlock(ctx, wrong)
	if refusedCode(t, err) != "bad_passphrase" {
		t.Fatal(err)
	}
	r := err.(*ErrRefused)
	if !strings.Contains(r.Reason, "wrong passphrase or damaged file") || strings.Contains(r.Reason, wrong) {
		t.Fatalf("reason %q", r.Reason)
	}
	// A damaged file gives the same refusal.
	if err := os.WriteFile(file, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Unlock(ctx, passphrase); refusedCode(t, err) != "bad_passphrase" {
		t.Fatal(err)
	}
	if st := e.svc.SessionStatus(ctx); st.State != StateLocked {
		t.Fatalf("still locked expected: %+v", st)
	}
	if out := logs.String(); strings.Contains(out, wrong) || strings.Contains(out, passphrase) {
		t.Fatalf("passphrase in logs: %s", out)
	}
}

// Ruling 52: a locked store reads as "store unavailable" (secret_store), never as "no credentials".
func TestSecretSwitch_LockedIsUnavailable(t *testing.T) {
	sw := NewSecretSwitch(secrets.NewNone(), true)
	for _, err := range []error{
		func() error { _, err := sw.Get(secrets.KeyAPISecret); return err }(),
		sw.Set(secrets.KeyAPISecret, "x"),
		sw.Delete(secrets.KeyAccessToken),
	} {
		if !errors.Is(err, ErrLocked) || !errors.Is(err, secrets.ErrUnavailable) || errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("locked error %v", err)
		}
	}
	if sw.Backend() != "locked" || !sw.Locked() {
		t.Fatal(sw.Backend())
	}
}

// failingKeyring refuses every Set (a keyring that breaks mid-migration).
type failingKeyring struct{ memStore }

func (f *failingKeyring) Set(secrets.Key, string) error {
	return fmt.Errorf("%w: collection locked", secrets.ErrUnavailable)
}

// P0-1 Migrate has rollback: a failed migration keeps the file and the switch on it.
func TestMigrateToKeyring_FailureKeepsFile(t *testing.T) {
	e, sw, file := lockedFile(t)
	ctx := context.Background()
	if _, err := e.svc.Unlock(ctx, passphrase); err != nil {
		t.Fatal(err)
	}
	kr := &failingKeyring{memStore{m: map[secrets.Key]string{}, backend: secrets.BackendKeyring}}
	e.svc.d.KeyringProbe = func() error { return nil }
	e.svc.d.NewKeyring = func() secrets.Store { return kr }
	info, err := e.svc.MigrateToKeyring(ctx)
	if refusedCode(t, err) != "migrate_failed" || info.Backend != secrets.BackendFile || !info.FileExists {
		t.Fatalf("%+v %v", info, err)
	}
	if strings.Contains(err.Error(), "s3cr3t-value-abc") {
		t.Fatal("secret in refusal")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("file removed after a failed migration")
	}
	if v, err := sw.Get(secrets.KeyAPISecret); err != nil || v != "s3cr3t-value-abc" {
		t.Fatalf("switch after failed migration %q %v", v, err)
	}
}

// hungKeyring blocks every call until gate closes (a locked collection waiting on an unlock prompt).
type hungKeyring struct {
	memStore
	gate chan struct{}
}

func (h *hungKeyring) Get(k secrets.Key) (string, error) { <-h.gate; return h.memStore.Get(k) }
func (h *hungKeyring) Set(k secrets.Key, v string) error { <-h.gate; return h.memStore.Set(k, v) }
func (h *hungKeyring) Delete(k secrets.Key) error        { <-h.gate; return h.memStore.Delete(k) }

// P0-1 note: keyring calls are time-bounded. A hung keyring fails the migration within the bound and
// leaves the file in place.
func TestMigrateToKeyring_HungKeyringIsBounded(t *testing.T) {
	old := keyringCallTimeout
	keyringCallTimeout = 50 * time.Millisecond
	t.Cleanup(func() { keyringCallTimeout = old })
	e, sw, file := lockedFile(t)
	ctx := context.Background()
	if _, err := e.svc.Unlock(ctx, passphrase); err != nil {
		t.Fatal(err)
	}
	kr := &hungKeyring{memStore: memStore{m: map[secrets.Key]string{}, backend: secrets.BackendKeyring}, gate: make(chan struct{})}
	defer close(kr.gate)
	e.svc.d.KeyringProbe = func() error { return nil }
	e.svc.d.NewKeyring = func() secrets.Store { return kr }
	done := make(chan error, 1)
	go func() { _, err := e.svc.MigrateToKeyring(ctx); done <- err }()
	select {
	case err := <-done:
		if refusedCode(t, err) != "migrate_failed" {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("migration not bounded")
	}
	if _, err := os.Stat(file); err != nil || sw.Backend() != secrets.BackendFile {
		t.Fatalf("file %v backend %s", err, sw.Backend())
	}
}

type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }
