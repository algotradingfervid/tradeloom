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

// Destroy closes the store and removes secrets.enc (used after migrating to the keyring). A failed
// removal is retried once.
func (f *EncFile) Destroy() error {
	f.Close()
	return RemoveFile(f.path)
}

// RemoveFile removes path, retrying once on failure; an absent file is not an error.
func RemoveFile(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	if err = os.Remove(path); err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
