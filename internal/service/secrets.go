// internal/service/secrets.go
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
	"tradeloom/internal/secrets"
)

const secretsStage = "secrets"

// lockedError is ErrLocked. It also matches secrets.ErrUnavailable, so the Session reports a locked store
// as a secret-store failure (CodeSecretStore), never as "no credentials" (Ruling 52).
type lockedError struct{}

func (lockedError) Error() string        { return "secrets: encrypted secrets file is locked" }
func (lockedError) Is(target error) bool { return target == secrets.ErrUnavailable }

// ErrLocked is returned by a SecretSwitch while the encrypted secrets file has not been unlocked this start.
var ErrLocked error = lockedError{}

// SecretSwitch is a secrets.Store whose backend changes at runtime (FR-ACCT-013). The Session keeps one Store
// for the whole process while setup, unlock and migration swap what is behind it.
//
// Locks: setup serialises the façade's setup/unlock/migration calls. It is taken first and held for the
// whole façade call, including the Session.Restore that UseEncryptedFile and Unlock run afterwards; the
// Session never calls back into the façade, so the order is setup → (Session: exMu → feedMu → mu) → op.
// op is held shared by Get/Set/Delete for the duration of the backend call (the Session calls them while
// holding its own locks) and exclusively by Swap, Refuse and the migration, so no write can land in a
// backend while it is being copied or closed. The migration holds op exclusively but never calls into the
// Session while it does. mu guards the fields and is a leaf.
type SecretSwitch struct {
	setup sync.Mutex
	op    sync.RWMutex

	// leftoverFile (guarded by setup) is set only when a MigrateToKeyring of this run moved every key but
	// returned file_not_removed. It is the only licence to delete secrets.enc while the keyring is the
	// backend (Ruling 72 (b)): "keyring backend + file exists" alone never deletes the file, which may be
	// the only copy of the secrets after any startup that picks the keyring next to an existing file.
	leftoverFile bool

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
	w.op.RLock()
	defer w.op.RUnlock()
	st, locked := w.current()
	if locked {
		return "", ErrLocked
	}
	return st.Get(k)
}

func (w *SecretSwitch) Set(k secrets.Key, v string) error {
	w.op.RLock()
	defer w.op.RUnlock()
	st, locked := w.current()
	if locked {
		return ErrLocked
	}
	return st.Set(k, v)
}

func (w *SecretSwitch) Delete(k secrets.Key) error {
	w.op.RLock()
	defer w.op.RUnlock()
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

// Swap installs st as the backend and clears the locked and refused flags. A previous *EncFile is closed.
func (w *SecretSwitch) Swap(st secrets.Store) {
	w.op.Lock()
	defer w.op.Unlock()
	w.swapLocked(st, false)
}

// swapLocked runs with op held exclusively.
func (w *SecretSwitch) swapLocked(st secrets.Store, refused bool) {
	w.mu.Lock()
	prev := w.store
	w.store, w.locked, w.refused = st, false, refused
	w.mu.Unlock()
	if ef, ok := prev.(*secrets.EncFile); ok && prev != st {
		ef.Close()
	}
}

// Refuse records the user's choice to store nothing (mode "none").
func (w *SecretSwitch) Refuse() {
	w.op.Lock()
	defer w.op.Unlock()
	w.swapLocked(secrets.NewNone(), true)
}

// exclusive runs fn with no backend call in flight and, when fn returns a store, installs it.
func (w *SecretSwitch) exclusive(fn func(cur secrets.Store) (secrets.Store, error)) error {
	w.op.Lock()
	defer w.op.Unlock()
	cur, _ := w.current()
	next, err := fn(cur)
	if err != nil {
		return err
	}
	w.swapLocked(next, false)
	return nil
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

// zeroBytes wipes a passphrase copy once the KDF has used it.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

const msgNoSecretStorage = "Secret storage is not configured for this build."

// UseEncryptedFile creates secrets.enc with the passphrase and makes it the backend (setup choice "encrypted file").
func (s *Service) UseEncryptedFile(ctx context.Context, passphrase string) (SessionStatus, error) {
	if s.d.Secrets == nil || s.d.SecretsFile == "" {
		return s.SessionStatus(ctx), refused(secretsStage, "unavailable", msgNoSecretStorage)
	}
	s.d.Secrets.setup.Lock()
	defer s.d.Secrets.setup.Unlock()
	if b, done := s.secretsConfigured(); done {
		return s.SessionStatus(ctx), refused(secretsStage, "already_configured", "Secret storage is already set up ("+b+").")
	}
	logging.RegisterSecret(passphrase)
	pp := []byte(passphrase)
	ef, err := secrets.CreateEncFile(s.d.SecretsFile, pp)
	zeroBytes(pp)
	switch {
	case errors.Is(err, secrets.ErrPassphraseTooShort):
		return s.SessionStatus(ctx), refused(secretsStage, "passphrase_too_short", fmt.Sprintf("Use a passphrase of at least %d characters.", secrets.MinPassphraseRunes))
	case errors.Is(err, secrets.ErrFileExists):
		return s.SessionStatus(ctx), refused(secretsStage, "file_exists", "A secrets file already exists; unlock it with its passphrase instead.")
	case err != nil:
		return s.SessionStatus(ctx), s.refuse(secretsStage, err)
	}
	s.d.Secrets.Swap(ef)
	s.d.Log.Info("secret storage set up", "backend", secrets.BackendFile)
	return s.Restore(ctx), nil
}

// RefuseSecretStorage records the setup choice "no storage": credentials are kept in memory for this run only.
func (s *Service) RefuseSecretStorage(ctx context.Context) (SessionStatus, error) {
	if s.d.Secrets == nil {
		return s.SessionStatus(ctx), refused(secretsStage, "unavailable", msgNoSecretStorage)
	}
	s.d.Secrets.setup.Lock()
	defer s.d.Secrets.setup.Unlock()
	if b, done := s.secretsConfigured(); done {
		return s.SessionStatus(ctx), refused(secretsStage, "already_configured", "Secret storage is already set up ("+b+").")
	}
	s.d.Secrets.Refuse()
	s.d.Log.Info("secret storage refused by the user")
	return s.SessionStatus(ctx), nil
}

// msgBadPassphrase is the fixed copy for a failed unlock: secrets.ErrWrongPassphrase cannot tell a wrong
// passphrase from a modified file (P0-1 note), so the text names both.
const msgBadPassphrase = "Unlock failed: wrong passphrase or damaged file. Check the passphrase and try again."

// Unlock opens secrets.enc with the passphrase and then runs the deferred startup Restore.
func (s *Service) Unlock(ctx context.Context, passphrase string) (SessionStatus, error) {
	if s.d.Secrets == nil {
		return s.SessionStatus(ctx), refused(secretsStage, "not_locked", "The secrets file is not locked.")
	}
	s.d.Secrets.setup.Lock()
	defer s.d.Secrets.setup.Unlock()
	if !s.d.Secrets.Locked() {
		return s.SessionStatus(ctx), refused(secretsStage, "not_locked", "The secrets file is not locked.")
	}
	logging.RegisterSecret(passphrase)
	pp := []byte(passphrase)
	ef, err := secrets.OpenEncFile(s.d.SecretsFile, pp)
	zeroBytes(pp)
	switch {
	case errors.Is(err, secrets.ErrWrongPassphrase), errors.Is(err, secrets.ErrCorruptFile), errors.Is(err, secrets.ErrPassphraseTooShort):
		s.d.Log.Warn("unlocking the secrets file failed", "reason", "wrong passphrase or damaged file")
		return s.SessionStatus(ctx), refused(secretsStage, "bad_passphrase", msgBadPassphrase)
	case err != nil:
		return s.SessionStatus(ctx), s.refuse(secretsStage, err)
	}
	s.d.Secrets.Swap(ef)
	s.d.Log.Info("secrets file unlocked")
	return s.Restore(ctx), nil
}

const (
	msgNotFileBackend  = "Only an encrypted secrets file can be forgotten."
	msgForgetNotRemove = "The secrets file could not be removed. Check that Tradeloom may write to its data folder, then try again."
)

// ForgetSecretsFile removes secrets.enc and returns secret storage to the setup choice (Ruling 68 carry-forward:
// "forget secrets file and start over"). It is the way out of the locked state when the passphrase is lost or the
// file is damaged (both unlock as bad_passphrase). The in-memory copy of an unlocked file is closed (key and values
// zeroed) before the file goes; a locked file has no copy in memory. Refusals: login_in_progress and
// already_logged_in (a live session's token would outlive its store), not_file_backend for the keyring, "none"
// or a build without secret storage (a file next to the keyring is never touched, Ruling 72 (b)), and
// file_not_removed when the removal fails — then the backend is not swapped (a locked file stays locked; an
// unlocked one is already closed and answers no reads) and the call can be repeated.
func (s *Service) ForgetSecretsFile(ctx context.Context) (SessionStatus, error) {
	sw := s.d.Secrets
	if sw == nil || s.d.SecretsFile == "" {
		return s.SessionStatus(ctx), refused(secretsStage, "not_file_backend", msgNotFileBackend)
	}
	sw.setup.Lock()
	defer sw.setup.Unlock()
	switch s.d.Session.State().State {
	case kite.StateLoggingIn:
		return s.SessionStatus(ctx), refused(secretsStage, kite.CodeLoginBusy, "Finish or cancel the current login before forgetting the secrets file.")
	case kite.StateReady:
		return s.SessionStatus(ctx), refused(secretsStage, kite.CodeAlreadyLoggedIn, "Log out before forgetting the secrets file.")
	}
	if !sw.Locked() && sw.Backend() != secrets.BackendFile {
		return s.SessionStatus(ctx), refused(secretsStage, "not_file_backend", msgNotFileBackend)
	}
	err := sw.exclusive(func(cur secrets.Store) (secrets.Store, error) {
		if ef, ok := cur.(*secrets.EncFile); ok {
			ef.Close()
		}
		if err := secrets.RemoveFile(s.d.SecretsFile); err != nil {
			return nil, err
		}
		return secrets.NewNone(), nil
	})
	if err != nil {
		s.d.Log.Warn("removing the secrets file at the user's request failed", "err", err)
		return s.SessionStatus(ctx), refused(secretsStage, "file_not_removed", msgForgetNotRemove)
	}
	sw.leftoverFile = false
	s.d.Log.Warn("secrets file forgotten at the user's request; secret storage is back to the setup choice")
	// The deferred startup Restore now runs against the empty store: it publishes session.state, which moves the
	// UI off the locked screen (the returned status alone is not adopted across a state change).
	return s.Restore(ctx), nil
}

// keyringCallTimeout bounds each keyring call made during a migration: a locked collection can block on
// an unlock prompt that never appears (P0-1 carry-forward). A variable only so tests can shorten it.
var keyringCallTimeout = secrets.ProbeTimeout

// boundedKeyring runs each call of the wrapped keyring on its own goroutine, one call at a time, and gives
// up after keyringCallTimeout with an error wrapping secrets.ErrUnavailable. Calls are serialised: a later
// call waits (within its own bound) for a hung earlier one, so a timed-out Set can never land after the
// rollback's Delete of the same key.
type boundedKeyring struct {
	next secrets.Store
	slot chan struct{} // capacity 1: held while a call to next is in flight
}

func newBoundedKeyring(next secrets.Store) *boundedKeyring {
	return &boundedKeyring{next: next, slot: make(chan struct{}, 1)}
}

func (b *boundedKeyring) do(name string, fn func() (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), keyringCallTimeout)
	defer cancel()
	timeout := func() error {
		return fmt.Errorf("%w: the keyring did not answer within %v", secrets.ErrUnavailable, keyringCallTimeout)
	}
	select {
	case b.slot <- struct{}{}:
	case <-ctx.Done():
		return "", timeout()
	}
	type result struct {
		v   string
		err error
	}
	ch := make(chan result, 1)
	logging.Go(name, func() {
		v, err := fn()
		<-b.slot
		ch <- result{v, err}
	})
	select {
	case r := <-ch:
		return r.v, r.err
	case <-ctx.Done():
		return "", timeout()
	}
}

func (b *boundedKeyring) Get(k secrets.Key) (string, error) {
	return b.do("service.keyring.get", func() (string, error) { return b.next.Get(k) })
}

func (b *boundedKeyring) Set(k secrets.Key, v string) error {
	_, err := b.do("service.keyring.set", func() (string, error) { return "", b.next.Set(k, v) })
	return err
}

func (b *boundedKeyring) Delete(k secrets.Key) error {
	_, err := b.do("service.keyring.delete", func() (string, error) { return "", b.next.Delete(k) })
	return err
}

func (b *boundedKeyring) Backend() string { return b.next.Backend() }

const msgFileNotRemoved = "The secrets were moved to the system keyring, but the old secrets file could not be removed. Try again to remove it."

// MigrateToKeyring moves every secret from the encrypted file into the system keyring and removes the file.
// secrets.Migrate copies and verifies each key and rolls back on any failure, leaving the file untouched.
// When the keys moved but the file could not be removed (secrets.ErrSourceNotRemoved), the keyring becomes
// the backend and the call still refuses with file_not_removed; calling it again with the keyring backend
// and the leftover file present removes only that file (Ruling 68) — and only when this run's migration
// reported file_not_removed (Ruling 72 (b)); otherwise a keyring backend is "not_file_backend".
func (s *Service) MigrateToKeyring(ctx context.Context) (SecretStorageInfo, error) {
	sw := s.d.Secrets
	if sw == nil {
		return s.secretInfo(), refused(secretsStage, "not_file_backend", "Only the encrypted-file backend can be moved to the keyring.")
	}
	sw.setup.Lock()
	defer sw.setup.Unlock()
	if s.d.Session.State().State == kite.StateLoggingIn {
		return s.secretInfo(), refused(secretsStage, kite.CodeLoginBusy, "Finish or cancel the current login before moving the secrets.")
	}
	if sw.leftoverFile && sw.Backend() == secrets.BackendKeyring && s.secretsFileExists() {
		if err := secrets.RemoveFile(s.d.SecretsFile); err != nil {
			s.d.Log.Warn("removing the leftover secrets file failed", "err", err)
			return s.secretInfo(), refused(secretsStage, "file_not_removed", msgFileNotRemoved)
		}
		sw.leftoverFile = false
		s.d.Log.Info("leftover secrets file removed")
		return s.secretInfo(), nil
	}
	if sw.Backend() != secrets.BackendFile {
		return s.secretInfo(), refused(secretsStage, "not_file_backend", "Only the encrypted-file backend can be moved to the keyring.")
	}
	if s.d.KeyringProbe != nil {
		if err := s.d.KeyringProbe(); err != nil {
			s.d.Log.Warn("keyring probe failed before migration", "err", err)
			return s.secretInfo(), refused(secretsStage, "keyring_unavailable", "No system keyring is available. Unlock or start it, then try again.")
		}
	}
	newKeyring := s.d.NewKeyring
	if newKeyring == nil {
		newKeyring = secrets.NewKeyring
	}
	kr := newKeyring()
	notRemoved := false
	err := sw.exclusive(func(cur secrets.Store) (secrets.Store, error) {
		err := secrets.Migrate(cur, newBoundedKeyring(kr))
		switch {
		case err == nil:
			return kr, nil
		case errors.Is(err, secrets.ErrSourceNotRemoved):
			// Every key is in the keyring and verified; the file store is closed. Use the keyring.
			s.d.Log.Warn("secrets moved to the keyring, but the old secrets file could not be removed", "err", err)
			notRemoved = true
			return kr, nil
		default:
			return nil, err
		}
	})
	if err != nil {
		s.d.Log.Warn("moving the secrets to the keyring failed", "err", err)
		return s.secretInfo(), refused(secretsStage, "migrate_failed", "The secrets could not be moved to the keyring; they stay in the encrypted file.")
	}
	if notRemoved {
		sw.leftoverFile = true
		return s.secretInfo(), refused(secretsStage, "file_not_removed", msgFileNotRemoved)
	}
	s.d.Log.Info("secrets moved to the system keyring")
	return s.secretInfo(), nil
}

// compile-time check: the switch is the Session's secrets.Store.
var _ secrets.Store = (*SecretSwitch)(nil)
