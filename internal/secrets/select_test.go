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
	f, err := CreateEncFile(path, []byte("passphrase-1"))
	must(t, err)
	must(t, f.Set(KeyAPISecret, "api-secret-1"))
	must(t, f.Set(KeyAccessToken, "access-token-1"))
	kr := NewKeyring()
	if err = Migrate(f, kr); err != nil {
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
	must(t, kr.Set(KeyTelegramToken, "123456:telegram"))
	f, err := CreateEncFile(filepath.Join(t.TempDir(), "secrets.enc"), []byte("passphrase-1"))
	must(t, err)
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
	f, err := CreateEncFile(path, []byte("passphrase-1"))
	must(t, err)
	defer f.Close()
	must(t, f.Set(KeyAPISecret, "api-secret-1"))
	must(t, f.Set(KeyAccessToken, "access-token-1"))
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

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMigrate_RollbackRestoresPreexistingTargetValue(t *testing.T) {
	fastKDF(t)
	keyring.MockInit()
	path := filepath.Join(t.TempDir(), "secrets.enc")
	f, err := CreateEncFile(path, []byte("passphrase-1"))
	must(t, err)
	defer f.Close()
	must(t, f.Set(KeyAPISecret, "new-api-secret"))
	must(t, f.Set(KeyAccessToken, "new-access-token"))
	kr := NewKeyring()
	must(t, kr.Set(KeyAPISecret, "precious-old"))
	if err := Migrate(f, failingStore{Store: kr, failOn: KeyAccessToken}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if v, err := kr.Get(KeyAPISecret); err != nil || v != "precious-old" {
		t.Fatalf("pre-existing target value lost: %q %v", v, err)
	}
	for _, k := range []Key{KeyAccessToken, KeyTelegramToken} {
		if _, err := kr.Get(k); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s must be absent after rollback: %v", k, err)
		}
	}
	if v, _ := f.Get(KeyAPISecret); v != "new-api-secret" {
		t.Fatal("source changed")
	}
	if v, _ := f.Get(KeyAccessToken); v != "new-access-token" {
		t.Fatal("source changed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("source file removed")
	}
}

// corruptingStore stores a different value than asked for, so verify must fail.
type corruptingStore struct {
	Store
	key Key
}

func (s corruptingStore) Set(k Key, v string) error {
	if k == s.key {
		v += "-corrupt"
	}
	return s.Store.Set(k, v)
}

func TestMigrate_VerifyFailureRollsBack(t *testing.T) {
	fastKDF(t)
	keyring.MockInit()
	f, err := CreateEncFile(filepath.Join(t.TempDir(), "secrets.enc"), []byte("passphrase-1"))
	must(t, err)
	defer f.Close()
	must(t, f.Set(KeyAPISecret, "a"))
	must(t, f.Set(KeyAccessToken, "b"))
	kr := NewKeyring()
	if err := Migrate(f, corruptingStore{Store: kr, key: KeyAccessToken}); err == nil {
		t.Fatal("expected verify failure")
	}
	for _, k := range []Key{KeyAPISecret, KeyAccessToken} {
		if _, err := kr.Get(k); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s not rolled back: %v", k, err)
		}
	}
	if v, _ := f.Get(KeyAPISecret); v != "a" {
		t.Fatal("source changed")
	}
}

func TestMigrate_SameStoreRejected(t *testing.T) {
	keyring.MockInit()
	kr := NewKeyring()
	must(t, kr.Set(KeyAPISecret, "x"))
	if err := Migrate(kr, kr); err == nil {
		t.Fatal("expected error")
	}
	if v, err := kr.Get(KeyAPISecret); err != nil || v != "x" {
		t.Fatalf("secret lost: %q %v", v, err)
	}
}

// readOnlyDir makes dir non-writable (os.Remove of its entries fails) until the test ends.
func readOnlyDir(t *testing.T, dir string) {
	t.Helper()
	must(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	probe := filepath.Join(dir, "probe")
	if err := os.WriteFile(probe, nil, 0o600); err == nil {
		os.Remove(probe)
		t.Skip("directory permissions are not enforced (running as root?)")
	}
}

// Ruling 68 (1): when every key was copied and verified but the source file cannot be removed, Migrate
// reports ErrSourceNotRemoved (checked with errors.Is), and the target keeps every secret.
func TestMigrate_SourceNotRemoved_Sentinel(t *testing.T) {
	fastKDF(t)
	keyring.MockInit()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	f, err := CreateEncFile(path, []byte("passphrase-1"))
	must(t, err)
	must(t, f.Set(KeyAPISecret, "api-secret-1"))
	readOnlyDir(t, dir)
	kr := NewKeyring()
	err = Migrate(f, kr)
	if !errors.Is(err, ErrSourceNotRemoved) {
		t.Fatalf("Migrate = %v, want ErrSourceNotRemoved", err)
	}
	if v, err := kr.Get(KeyAPISecret); err != nil || v != "api-secret-1" {
		t.Fatalf("target after partial success %q %v", v, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("precondition: file should still exist")
	}
}
