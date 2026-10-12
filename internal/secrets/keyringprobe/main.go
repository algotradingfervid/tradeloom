// Command keyringprobe is the S0-4 spike tool (docs/architecture/spikes/S0-4-keyring.md).
// It prints one JSON object describing how internal/secrets behaves in the current session.
// It writes and deletes real keyring entries, so it refuses to run unless
// TRADELOOM_SPIKE_DISPOSABLE=1 (use only in throwaway containers/VMs).
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"tradeloom/internal/clock"
	"tradeloom/internal/secrets"
)

type result struct {
	Env              map[string]string `json:"env"`
	Mode             string            `json:"mode"`
	ProbeError       string            `json:"probe_error,omitempty"`
	ProbeMillis      int64             `json:"probe_ms"`
	RoundTripOK      bool              `json:"keyring_roundtrip_ok"`
	RoundTripError   string            `json:"keyring_roundtrip_error,omitempty"`
	RoundTripMillis  int64             `json:"keyring_roundtrip_ms"`
	EncCreateMillis  int64             `json:"encfile_create_ms"`
	EncOpenMillis    int64             `json:"encfile_open_ms"`
	MigrateToKeyring string            `json:"migrate_file_to_keyring"`
}

func ms(start time.Time) int64 { return clock.SystemNow().Sub(start).Milliseconds() }

func main() {
	if os.Getenv("TRADELOOM_SPIKE_DISPOSABLE") != "1" {
		fmt.Fprintln(os.Stderr, "keyringprobe: set TRADELOOM_SPIKE_DISPOSABLE=1 (it overwrites tradeloom keyring entries)")
		os.Exit(2)
	}
	r, err := run(func() error { return secrets.ProbeKeyringWithin(secrets.ProbeTimeout) })
	if err != nil {
		fmt.Fprintln(os.Stderr, "keyringprobe:", err)
		os.Exit(1)
	}
	if err := write(os.Stdout, r); err != nil {
		fmt.Fprintln(os.Stderr, "keyringprobe:", err)
		os.Exit(1)
	}
}

func write(w io.Writer, r result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// run executes the whole matrix probe in a temp dir using the given keyring probe function.
func run(probe func() error) (result, error) {
	r := result{Env: map[string]string{
		"DBUS_SESSION_BUS_ADDRESS": os.Getenv("DBUS_SESSION_BUS_ADDRESS"),
		"XDG_CURRENT_DESKTOP":      os.Getenv("XDG_CURRENT_DESKTOP"),
		"DISPLAY":                  os.Getenv("DISPLAY"),
		"WAYLAND_DISPLAY":          os.Getenv("WAYLAND_DISPLAY"),
	}}
	dir, err := os.MkdirTemp("", "keyringprobe-")
	if err != nil {
		return r, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "secrets.enc")

	t := clock.SystemNow()
	d := secrets.Detect(path, probe)
	r.ProbeMillis, r.Mode = ms(t), string(d.Mode)
	if d.ProbeError != nil {
		r.ProbeError = d.ProbeError.Error()
	}
	if d.KeyringAvailable {
		t = clock.SystemNow()
		kr := secrets.NewKeyring()
		err := kr.Set(secrets.KeyTelegramToken, "s0-4-probe-value")
		if err == nil {
			var v string
			v, err = kr.Get(secrets.KeyTelegramToken)
			r.RoundTripOK = err == nil && v == "s0-4-probe-value"
			if derr := kr.Delete(secrets.KeyTelegramToken); err == nil {
				err = derr
			}
		}
		r.RoundTripMillis = ms(t)
		if err != nil {
			r.RoundTripError = err.Error()
		}
	}
	pass := []byte("s0-4 spike passphrase")
	t = clock.SystemNow()
	f, err := secrets.CreateEncFile(path, pass)
	if err != nil {
		return r, err
	}
	r.EncCreateMillis = ms(t)
	if err := f.Set(secrets.KeyTelegramToken, "s0-4-file-value"); err != nil {
		f.Close()
		return r, err
	}
	f.Close()
	t = clock.SystemNow()
	g, err := secrets.OpenEncFile(path, pass)
	if err != nil {
		return r, err
	}
	r.EncOpenMillis = ms(t)
	r.MigrateToKeyring = "skipped: keyring unavailable"
	if d.KeyringAvailable {
		if err := secrets.Migrate(g, secrets.NewKeyring()); err != nil {
			r.MigrateToKeyring = "error: " + err.Error()
			g.Close()
		} else {
			_, statErr := os.Stat(path)
			r.MigrateToKeyring = fmt.Sprintf("ok (secrets.enc removed: %v)", os.IsNotExist(statErr))
		}
		_ = secrets.NewKeyring().Delete(secrets.KeyTelegramToken)
	} else {
		g.Close()
	}
	return r, nil
}
