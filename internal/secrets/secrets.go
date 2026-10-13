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
	// ErrSourceNotRemoved: Migrate copied and verified every key into the target, but the source could
	// not be cleaned up (secrets.enc not removed, or a key not deleted). The target holds every secret.
	ErrSourceNotRemoved = errors.New("secrets: migrated, but the source was not removed")
)

// none is the "refuse to store credentials" backend (FR-ACCT-013 choice b).
type none struct{}

// NewNone returns the backend used when the user refuses credential storage.
func NewNone() Store { return none{} }

func (none) Get(Key) (string, error) { return "", ErrNotFound }
func (none) Set(Key, string) error   { return ErrStorageRefused }
func (none) Delete(Key) error        { return nil }
func (none) Backend() string         { return BackendNone }
