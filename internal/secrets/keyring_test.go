package secrets

import (
	"errors"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestProbeKeyring(t *testing.T) {
	keyring.MockInit()
	if err := ProbeKeyring(); err != nil {
		t.Fatalf("daemon present: %v", err)
	}
	keyring.MockInitWithError(errors.New("dbus: The name org.freedesktop.secrets was not provided by any .service files"))
	if err := ProbeKeyring(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no daemon: want ErrUnavailable, got %v", err)
	}
}

func TestKeyringStore_RoundTrip(t *testing.T) {
	keyring.MockInit()
	s := NewKeyring()
	if s.Backend() != BackendKeyring {
		t.Fatal(s.Backend())
	}
	if _, err := s.Get(KeyAPISecret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.Set(KeyAPISecret, "s3cret-value"); err != nil {
		t.Fatal(err)
	}
	if v, err := s.Get(KeyAPISecret); err != nil || v != "s3cret-value" {
		t.Fatalf("%q %v", v, err)
	}
	if err := s.Delete(KeyAPISecret); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(KeyAPISecret); err != nil {
		t.Fatalf("deleting a missing key must be a no-op: %v", err)
	}
}

// Review Focus 2: the daemon disappears mid-session → every call fails with ErrUnavailable;
// there is no silent fallback.
func TestKeyringStore_DaemonGoneMidSession(t *testing.T) {
	keyring.MockInit()
	s := NewKeyring()
	if err := s.Set(KeyAccessToken, "tok-123456"); err != nil {
		t.Fatal(err)
	}
	keyring.MockInitWithError(errors.New("dbus: connection closed by user"))
	if err := s.Set(KeyAccessToken, "tok-654321"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Set: want ErrUnavailable, got %v", err)
	}
	if _, err := s.Get(KeyAccessToken); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get: want ErrUnavailable, got %v", err)
	}
	if err := s.Delete(KeyAccessToken); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Delete: want ErrUnavailable, got %v", err)
	}
	if s.Backend() != BackendKeyring {
		t.Fatal("backend changed silently")
	}
}

func TestNone(t *testing.T) {
	s := NewNone()
	if err := s.Set(KeyAPISecret, "x"); !errors.Is(err, ErrStorageRefused) {
		t.Fatal(err)
	}
	if _, err := s.Get(KeyAPISecret); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if s.Backend() != BackendNone || s.Delete(KeyAPISecret) != nil {
		t.Fatal("none backend")
	}
}

// S0-4 / Review Focus 2: a probe that never answers (locked collection, prompt with no display)
// must not hang startup.
func TestProbeWithin_Timeout(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	start := time.Now()
	err := probeWithin(func() error { <-block; return nil }, 50*time.Millisecond)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("probe waited %v", d)
	}
	if err := probeWithin(func() error { return nil }, time.Second); err != nil {
		t.Fatalf("fast probe: %v", err)
	}
}
