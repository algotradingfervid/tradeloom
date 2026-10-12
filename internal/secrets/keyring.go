package secrets

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"

	"tradeloom/internal/logging"
)

// KeyringService is the Secret Service "service" attribute; the user attribute is the Key.
const KeyringService = "tradeloom"

const probeUser = "probe"

// ProbeKeyring returns nil when the Secret Service answers (a missing probe entry is an answer),
// or an error wrapping ErrUnavailable (no daemon, no session bus, locked/denied collection).
func ProbeKeyring() error {
	_, err := keyring.Get(KeyringService, probeUser)
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrUnavailable, err)
}

// ProbeTimeout bounds ProbeKeyringWithin: a locked collection can block on an unlock prompt that
// never appears in a headless session (S0-4).
const ProbeTimeout = 3 * time.Second

// ProbeKeyringWithin runs ProbeKeyring and reports ErrUnavailable if it has not answered within d.
func ProbeKeyringWithin(d time.Duration) error { return probeWithin(ProbeKeyring, d) }

func probeWithin(probe func() error, d time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	done := make(chan error, 1)
	logging.Go("secrets.probe", func() { done <- probe() })
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%w: no answer from the Secret Service within %v", ErrUnavailable, d)
	}
}

type keyringStore struct{}

// NewKeyring returns the OS keyring backend.
func NewKeyring() Store { return keyringStore{} }

func (keyringStore) Backend() string { return BackendKeyring }

func wrapKeyring(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, keyring.ErrNotFound):
		return ErrNotFound
	default:
		// No fallback of any kind: the caller surfaces the error (Review Focus 2).
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
}

func (keyringStore) Get(k Key) (string, error) {
	v, err := keyring.Get(KeyringService, string(k))
	if err != nil {
		return "", wrapKeyring(err)
	}
	logging.RegisterSecret(v)
	return v, nil
}

func (keyringStore) Set(k Key, v string) error {
	logging.RegisterSecret(v)
	return wrapKeyring(keyring.Set(KeyringService, string(k), v))
}

func (keyringStore) Delete(k Key) error {
	err := wrapKeyring(keyring.Delete(KeyringService, string(k)))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
