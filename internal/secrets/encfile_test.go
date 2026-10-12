package secrets

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func fastKDF(t *testing.T) {
	t.Helper()
	old := kdfForNewFiles
	kdfForNewFiles = KDFParams{Time: 1, MemoryKiB: 8 * 1024, Threads: 1}
	t.Cleanup(func() { kdfForNewFiles = old })
}

const apiSecret = "kiteApiSecretForGrep-7f3a9c"

// FR-ACCT-013 AC2 with the production KDF parameters.
func TestEncFile_NoPlaintextAndWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	f, err := CreateEncFile(path, []byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Set(KeyAPISecret, apiSecret); err != nil {
		t.Fatal(err)
	}
	if err := f.Set(KeyAccessToken, "accessTokenValue-123"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		b, _ := os.ReadFile(p)
		if bytes.Contains(b, []byte(apiSecret)) {
			t.Fatalf("%s contains the API secret in plaintext", p)
		}
		return nil
	})
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}
	if _, err := OpenEncFile(path, []byte("wrong horse")); !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("want ErrWrongPassphrase, got %v", err)
	}
	g, err := OpenEncFile(path, []byte("correct horse"))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	if v, _ := g.Get(KeyAPISecret); v != apiSecret {
		t.Fatalf("got %q", v)
	}
}

func TestEncFile_Validation(t *testing.T) {
	fastKDF(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	cases := []struct {
		name string
		pass string
		want error
	}{
		{"7 chars", "1234567", ErrPassphraseTooShort},
		{"8 multibyte chars ok", "पासवर्ड१२३", nil},
	}
	for _, tc := range cases {
		f, err := CreateEncFile(path, []byte(tc.pass))
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s: got %v", tc.name, err)
		}
		if f != nil {
			f.Close()
		}
	}
	if _, err := CreateEncFile(path, []byte("another-pass")); !errors.Is(err, ErrFileExists) {
		t.Fatalf("want ErrFileExists, got %v", err)
	}
}

func TestEncFile_TamperAndTruncation(t *testing.T) {
	fastKDF(t)
	path := filepath.Join(t.TempDir(), "secrets.enc")
	f, _ := CreateEncFile(path, []byte("passphrase-1"))
	f.Set(KeyTelegramToken, "123456789:tok")
	f.Close()
	good, _ := os.ReadFile(path)

	cases := []struct {
		name   string
		mutate func([]byte) []byte
		want   error
	}{
		{"truncated", func(b []byte) []byte { return b[:20] }, ErrCorruptFile},
		{"bad magic", func(b []byte) []byte { c := append([]byte(nil), b...); c[0] = 'X'; return c }, ErrCorruptFile},
		{"huge memory param", func(b []byte) []byte { c := append([]byte(nil), b...); c[10] = 0xff; return c }, ErrCorruptFile},
		{"salt flipped", func(b []byte) []byte { c := append([]byte(nil), b...); c[20] ^= 1; return c }, ErrWrongPassphrase},
		{"ciphertext flipped", func(b []byte) []byte { c := append([]byte(nil), b...); c[len(c)-1] ^= 1; return c }, ErrWrongPassphrase},
	}
	for _, tc := range cases {
		os.WriteFile(path, tc.mutate(good), 0o600)
		if _, err := OpenEncFile(path, []byte("passphrase-1")); !errors.Is(err, tc.want) {
			t.Errorf("%s: want %v, got %v", tc.name, tc.want, err)
		}
	}
}

func TestEncFile_CloseZeroesAndDestroyRemoves(t *testing.T) {
	fastKDF(t)
	path := filepath.Join(t.TempDir(), "secrets.enc")
	f, _ := CreateEncFile(path, []byte("passphrase-1"))
	f.Set(KeyAPISecret, apiSecret)
	key, val := f.key, f.vals[string(KeyAPISecret)]
	f.Close()
	if !bytes.Equal(key, make([]byte, len(key))) || !bytes.Equal(val, make([]byte, len(val))) {
		t.Fatal("key/value not zeroed")
	}
	if _, err := f.Get(KeyAPISecret); err == nil {
		t.Fatal("closed store must refuse")
	}
	if err := f.Destroy(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file not removed")
	}
}
