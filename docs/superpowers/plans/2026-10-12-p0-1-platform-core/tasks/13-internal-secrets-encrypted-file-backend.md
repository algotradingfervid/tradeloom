> **P0-1 Platform Core Implementation Plan — Task 13.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 13: `internal/secrets` — encrypted file backend (`secrets.enc`)

**Files:**
- Create: `internal/secrets/encfile.go`, `internal/secrets/encfile_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: Task 12 `Key`, `Store`, errors, `BackendFile`; Task 4 `logging.RegisterSecret`.
- Produces (`package secrets`):
  - `const MinPassphraseRunes = 8`; `type KDFParams struct { Time uint32; MemoryKiB uint32; Threads uint8 }`; `var DefaultKDF = KDFParams{Time: 3, MemoryKiB: 64 * 1024, Threads: 4}`
  - `type EncFile struct{…}` (implements `Store`); `func CreateEncFile(path string, passphrase []byte) (*EncFile, error)`; `func OpenEncFile(path string, passphrase []byte) (*EncFile, error)`; `func (f *EncFile) Close()` (zeroes key and values); `func (f *EncFile) Destroy() error` (Close + remove file)
  - File format: `"TLSEC1"` | time u32 BE | memoryKiB u32 BE | threads u8 | salt[16] | nonce[24] | XChaCha20-Poly1305 ciphertext of a JSON map; the 31-byte header is the AEAD additional data. Key = `argon2.IDKey(passphrase, salt, t, m, p, 32)` (`golang.org/x/crypto/argon2`), AEAD = `chacha20poly1305.NewX` — both verified in x/crypto v0.55.0.
  - The passphrase is asked once per start by P0-3/P0-4 and passed as `[]byte`; callers zero their copy after `OpenEncFile`/`CreateEncFile` returns.

- [ ] **Step 1: Write the failing test**

`internal/secrets/encfile_test.go`:

```go
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
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags noorders ./internal/secrets/`
Expected: FAIL — build errors `undefined: kdfForNewFiles`, `undefined: KDFParams`, `undefined: CreateEncFile`.

- [ ] **Step 3: Write the implementation**

```bash
go get golang.org/x/crypto@v0.55.0
```

`internal/secrets/encfile.go`:

```go
package secrets

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"

	"tradeloom/internal/logging"
)

// File format (ARCHITECTURE §9.1), all integers big-endian:
//
//	"TLSEC1" | time uint32 | memoryKiB uint32 | threads uint8 | salt [16] | nonce [24] | ciphertext
//
// The 31-byte header (magic … salt) is the AEAD additional data, so tampering with the KDF
// parameters or the salt fails authentication.
const (
	magic      = "TLSEC1"
	saltLen    = 16
	headerLen  = len(magic) + 4 + 4 + 1 + saltLen
	keyLen     = chacha20poly1305.KeySize
	nonceLen   = chacha20poly1305.NonceSizeX
	fileMode   = 0o600
	maxMemKiB  = 1 << 20 // 1 GiB: refuse crafted headers that would exhaust memory
	maxTime    = 16
	maxThreads = 64
)

// MinPassphraseRunes is the FR-ACCT-013 minimum passphrase length.
const MinPassphraseRunes = 8

// KDFParams are the Argon2id cost parameters stored in the file header.
type KDFParams struct {
	Time      uint32
	MemoryKiB uint32
	Threads   uint8
}

// DefaultKDF is t=3, m=64 MiB, p=4 (ARCHITECTURE §9.1).
var DefaultKDF = KDFParams{Time: 3, MemoryKiB: 64 * 1024, Threads: 4}

// kdfForNewFiles is replaced by tests to keep them fast; production never changes it.
var kdfForNewFiles = DefaultKDF

// EncFile is the encrypted-file backend. The derived key and values live only in memory and are
// zeroed by Close.
type EncFile struct {
	mu     sync.Mutex
	path   string
	params KDFParams
	salt   []byte
	key    []byte
	vals   map[string][]byte
	closed bool
}

func checkPassphrase(p []byte) error {
	if utf8.RuneCount(p) < MinPassphraseRunes {
		return ErrPassphraseTooShort
	}
	return nil
}

// CreateEncFile creates a new, empty secrets.enc at path (mode 0600). It refuses to overwrite.
func CreateEncFile(path string, passphrase []byte) (*EncFile, error) {
	if err := checkPassphrase(passphrase); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err == nil {
		return nil, ErrFileExists
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	p := kdfForNewFiles
	f := &EncFile{path: path, params: p, salt: salt, vals: map[string][]byte{},
		key: argon2.IDKey(passphrase, salt, p.Time, p.MemoryKiB, p.Threads, keyLen)}
	if err := f.persist(); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// OpenEncFile decrypts path with passphrase. A wrong passphrase or any modification yields
// ErrWrongPassphrase; a bad header or truncated file yields ErrCorruptFile.
func OpenEncFile(path string, passphrase []byte) (*EncFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < headerLen+nonceLen+chacha20poly1305.Overhead || string(b[:len(magic)]) != magic {
		return nil, ErrCorruptFile
	}
	p := KDFParams{
		Time:      binary.BigEndian.Uint32(b[6:10]),
		MemoryKiB: binary.BigEndian.Uint32(b[10:14]),
		Threads:   b[14],
	}
	if p.Time == 0 || p.Time > maxTime || p.MemoryKiB == 0 || p.MemoryKiB > maxMemKiB || p.Threads == 0 || p.Threads > maxThreads {
		return nil, ErrCorruptFile
	}
	header := b[:headerLen]
	salt := append([]byte(nil), b[15:headerLen]...)
	nonce := b[headerLen : headerLen+nonceLen]
	key := argon2.IDKey(passphrase, salt, p.Time, p.MemoryKiB, p.Threads, keyLen)
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		zero(key)
		return nil, err
	}
	plain, err := aead.Open(nil, nonce, b[headerLen+nonceLen:], header)
	if err != nil {
		zero(key)
		return nil, ErrWrongPassphrase
	}
	vals := map[string][]byte{}
	err = json.Unmarshal(plain, &vals)
	zero(plain)
	if err != nil {
		zero(key)
		return nil, ErrCorruptFile
	}
	for _, v := range vals {
		logging.RegisterSecret(string(v))
	}
	return &EncFile{path: path, params: p, salt: salt, key: key, vals: vals}, nil
}

// persist encrypts the map with a fresh nonce and atomically replaces the file (temp + fsync + rename).
func (f *EncFile) persist() error {
	plain, err := json.Marshal(f.vals)
	if err != nil {
		return err
	}
	defer zero(plain)
	aead, err := chacha20poly1305.NewX(f.key)
	if err != nil {
		return err
	}
	var hdr bytes.Buffer
	hdr.WriteString(magic)
	binary.Write(&hdr, binary.BigEndian, f.params.Time)
	binary.Write(&hdr, binary.BigEndian, f.params.MemoryKiB)
	hdr.WriteByte(f.params.Threads)
	hdr.Write(f.salt)
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	out := append(append(hdr.Bytes(), nonce...), aead.Seal(nil, nonce, plain, hdr.Bytes())...)

	dir := filepath.Dir(f.path)
	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err := tmp.Chmod(fileMode); err != nil {
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return err
	}
	ok = true
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

func (f *EncFile) Backend() string { return BackendFile }

func (f *EncFile) Get(k Key) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return "", errors.New("secrets: file store closed")
	}
	v, ok := f.vals[string(k)]
	if !ok {
		return "", ErrNotFound
	}
	return string(v), nil
}

func (f *EncFile) Set(k Key, v string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errors.New("secrets: file store closed")
	}
	logging.RegisterSecret(v)
	old, had := f.vals[string(k)]
	f.vals[string(k)] = []byte(v)
	if err := f.persist(); err != nil {
		if had {
			f.vals[string(k)] = old
		} else {
			delete(f.vals, string(k))
		}
		return fmt.Errorf("secrets: write %s: %w", f.path, err)
	}
	if had {
		zero(old)
	}
	return nil
}

func (f *EncFile) Delete(k Key) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return errors.New("secrets: file store closed")
	}
	old, ok := f.vals[string(k)]
	if !ok {
		return nil
	}
	delete(f.vals, string(k))
	if err := f.persist(); err != nil {
		f.vals[string(k)] = old
		return err
	}
	zero(old)
	return nil
}

// Close zeroes the key and every value held in memory. Further calls fail.
func (f *EncFile) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	zero(f.key)
	for k, v := range f.vals {
		zero(v)
		delete(f.vals, k)
	}
	f.closed = true
}

// Destroy closes the store and removes secrets.enc (used after migrating to the keyring).
func (f *EncFile) Destroy() error {
	f.Close()
	if err := os.Remove(f.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go mod tidy && grep -E '^go ' go.mod && go test -race -count=1 -tags noorders ./internal/secrets/ ./internal/archtest/`
Expected: `go 1.25.0`; `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/secrets/encfile.go internal/secrets/encfile_test.go
git commit -m "feat(secrets): Argon2id + XChaCha20-Poly1305 encrypted secrets file with atomic writes" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
