package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"tradeloom/internal/secrets"
)

func TestRunKeyringAvailable(t *testing.T) {
	keyring.MockInit()
	r, err := run(func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != "keyring" || !r.RoundTripOK || r.RoundTripError != "" {
		t.Fatalf("unexpected result: %+v", r)
	}
	if r.MigrateToKeyring != "ok (secrets.enc removed: true)" {
		t.Fatalf("migrate = %q", r.MigrateToKeyring)
	}
	if _, err := keyring.Get(secrets.KeyringService, string(secrets.KeyTelegramToken)); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("probe entry left behind: %v", err)
	}
	var buf bytes.Buffer
	if err := write(&buf, r); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "s0-4-") {
		t.Fatalf("output leaks probe values: %s", buf.String())
	}
	var back map[string]any
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	for _, k := range []string{"mode", "probe_ms", "keyring_roundtrip_ok", "encfile_create_ms", "encfile_open_ms", "migrate_file_to_keyring"} {
		if _, ok := back[k]; !ok {
			t.Errorf("missing JSON key %q", k)
		}
	}
}

func TestRunKeyringUnavailable(t *testing.T) {
	keyring.MockInitWithError(errors.New("boom"))
	probeErr := errors.New("secrets: keyring unavailable: no bus")
	r, err := run(func() error { return probeErr })
	if err != nil {
		t.Fatal(err)
	}
	if r.Mode != "choose" || r.ProbeError != probeErr.Error() || r.RoundTripOK {
		t.Fatalf("unexpected result: %+v", r)
	}
	if r.MigrateToKeyring != "skipped: keyring unavailable" {
		t.Fatalf("migrate = %q", r.MigrateToKeyring)
	}
	if r.EncCreateMillis < 0 || r.EncOpenMillis < 0 {
		t.Fatalf("bad timings: %+v", r)
	}
}

func TestRunRefusesExistingEntry(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(secrets.KeyringService, string(secrets.KeyTelegramToken), "real-user-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(func() error { return nil }); !errors.Is(err, errEntryExists) {
		t.Fatalf("err = %v, want errEntryExists", err)
	}
	if v, _ := keyring.Get(secrets.KeyringService, string(secrets.KeyTelegramToken)); v != "real-user-token" {
		t.Fatalf("existing entry was modified: %q", v)
	}
}
