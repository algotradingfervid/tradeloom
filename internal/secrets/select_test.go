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
