> **P0-1 Platform Core Implementation Plan — Task 18.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 18: Spike S0-4 — keyring behaviour, passphrase UX, migration

**Files:**
- Create: `internal/secrets/keyringprobe/main.go`, `docs/architecture/spikes/S0-4-keyring.md`
- Modify: `.go-arch-lint.yml` already maps `internal/secrets/keyringprobe` (Task 16); if a decision changes `secrets.ProbeTimeout` or `secrets.DefaultKDF`, modify `internal/secrets/keyring.go` / `encfile.go` and re-run their tests.

**Interfaces:**
- Consumes: Task 12–14 `Detect`, `ProbeKeyringWithin`, `ProbeTimeout`, `NewKeyring`, `CreateEncFile`, `OpenEncFile`, `Migrate`; Task 2 `clock.SystemNow`.
- Produces: OQ-48 confirmation (zalando/go-keyring v0.2.8 + `secrets.enc` fallback), measured `ProbeTimeout` adequacy, Argon2id timing on the reference machine (passphrase UX), behaviour matrix for P0-3's setup flow.

- [ ] **Step 1: Write the probe tool**

`internal/secrets/keyringprobe/main.go` (refuses to run unless `TRADELOOM_SPIKE_DISPOSABLE=1` because it writes and deletes the real `tradeloom/telegram.bot_token` keyring entry):

```go
// Command keyringprobe is the S0-4 spike tool (docs/architecture/spikes/S0-4-keyring.md).
// It prints one JSON object describing how internal/secrets behaves in the current session.
// It writes and deletes real keyring entries, so it refuses to run unless
// TRADELOOM_SPIKE_DISPOSABLE=1 (use only in throwaway containers/VMs).
package main

import (
	"encoding/json"
	"fmt"
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
	r := result{Env: map[string]string{
		"DBUS_SESSION_BUS_ADDRESS": os.Getenv("DBUS_SESSION_BUS_ADDRESS"),
		"XDG_CURRENT_DESKTOP":      os.Getenv("XDG_CURRENT_DESKTOP"),
		"DISPLAY":                  os.Getenv("DISPLAY"),
		"WAYLAND_DISPLAY":          os.Getenv("WAYLAND_DISPLAY"),
	}}
	dir, err := os.MkdirTemp("", "keyringprobe-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	t := clock.SystemNow()
	d := secrets.Detect(filepath.Join(dir, "secrets.enc"), func() error { return secrets.ProbeKeyringWithin(secrets.ProbeTimeout) })
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
	f, err := secrets.CreateEncFile(filepath.Join(dir, "secrets.enc"), pass)
	if err != nil {
		panic(err)
	}
	r.EncCreateMillis = ms(t)
	if err := f.Set(secrets.KeyTelegramToken, "s0-4-file-value"); err != nil {
		panic(err)
	}
	f.Close()
	t = clock.SystemNow()
	g, err := secrets.OpenEncFile(filepath.Join(dir, "secrets.enc"), pass)
	if err != nil {
		panic(err)
	}
	r.EncOpenMillis = ms(t)
	r.MigrateToKeyring = "skipped: keyring unavailable"
	if d.KeyringAvailable {
		if err := secrets.Migrate(g, secrets.NewKeyring()); err != nil {
			r.MigrateToKeyring = "error: " + err.Error()
		} else {
			_, statErr := os.Stat(filepath.Join(dir, "secrets.enc"))
			r.MigrateToKeyring = fmt.Sprintf("ok (secrets.enc removed: %v)", os.IsNotExist(statErr))
			secrets.NewKeyring().Delete(secrets.KeyTelegramToken)
		}
	} else {
		g.Close()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(r)
}
```

Run: `go vet ./internal/secrets/keyringprobe && go run ./internal/secrets/keyringprobe; echo "exit=$?"`
Expected: `keyringprobe: set TRADELOOM_SPIKE_DISPOSABLE=1 (it overwrites tradeloom keyring entries)` and `exit=2`. Then `go test -count=1 -tags noorders ./internal/archtest/ && go-arch-lint check` → `ok` / `OK - No warnings found`.

- [ ] **Step 2: Run the behaviour matrix**

Build once inside the Linux image (static, so it runs in any distro container): `docker run --rm -e CGO_ENABLED=0 -v "$PWD":/src -w /src tradeloom-build:p0 go build -o build/bin/keyringprobe ./internal/secrets/keyringprobe`. Then:

```bash
# (a) GNOME Keyring, unlocked (Secret Service present)
docker run --rm -v "$PWD":/src -w /src ubuntu:24.04 bash -c '
  apt-get update -qq && apt-get install -y -qq gnome-keyring dbus >/dev/null &&
  dbus-run-session -- sh -c "echo -n spikepass | gnome-keyring-daemon --unlock --components=secrets >/dev/null;
    TRADELOOM_SPIKE_DISPOSABLE=1 build/bin/keyringprobe"'
# (b) session bus, no keyring daemon
docker run --rm -v "$PWD":/src -w /src ubuntu:24.04 bash -c '
  apt-get update -qq && apt-get install -y -qq dbus >/dev/null &&
  dbus-run-session -- env TRADELOOM_SPIKE_DISPOSABLE=1 build/bin/keyringprobe'
# (c) no session bus at all (headless service / minimal container)
docker run --rm -v "$PWD":/src -w /src ubuntu:24.04 env TRADELOOM_SPIKE_DISPOSABLE=1 build/bin/keyringprobe
# (d) daemon running but collection locked (no unlock, no display → prompt can never complete)
docker run --rm -v "$PWD":/src -w /src ubuntu:24.04 bash -c '
  apt-get update -qq && apt-get install -y -qq gnome-keyring dbus >/dev/null &&
  dbus-run-session -- sh -c "gnome-keyring-daemon --start --components=secrets >/dev/null;
    TRADELOOM_SPIKE_DISPOSABLE=1 build/bin/keyringprobe"'
```
Manual VM rows: (e) KDE Plasma desktop VM (Kubuntu 24.04 or Fedora KDE) with KWallet's Secret Service enabled, run from a desktop terminal; (f) the same GNOME VM reached over `ssh` (no desktop session in the SSH shell); (g) the reference laptop (record CPU) for `encfile_create_ms`/`encfile_open_ms`.
Expected shape: (a) `"mode": "keyring"`, `keyring_roundtrip_ok: true`, `migrate_file_to_keyring: "ok (secrets.enc removed: true)"`; (b) and (c) `"mode": "choose"` with a `probe_error` naming `org.freedesktop.secrets` / the missing bus; (d) `"mode": "choose"` with `probe_error` containing `no answer from the Secret Service within 3s` (or an immediate locked error) — never a hang. Record every JSON output verbatim.

- [ ] **Step 3: Write `docs/architecture/spikes/S0-4-keyring.md`**

```markdown
# S0-4 — Keyring (spike result)

Date: <YYYY-MM-DD> · Commit: <git rev-parse --short HEAD> · Decides: OQ-48, FR-ACCT-013, ARCHITECTURE §9.1

## Behaviour matrix (`internal/secrets/keyringprobe`)
| Row | Environment | mode | probe_ms | probe_error | roundtrip | migrate file→keyring |
|---|---|---|---|---|---|---|
| a | GNOME Keyring unlocked (container) | | | | | |
| b | session bus, no daemon | | | | | |
| c | no session bus | | | | | |
| d | GNOME Keyring locked, no display | | | | | |
| e | KDE / KWallet Secret Service (VM) | | | | | |
| f | GNOME VM over ssh | | | | | |

## Argon2id cost (DefaultKDF t=3, m=64 MiB, p=4)
| Machine | encfile_create_ms | encfile_open_ms |
|---|---|---|

## Decisions
1. OQ-48: zalando/go-keyring v0.2.8 stays the primary backend if rows a and e round-trip; otherwise record the failing desktop and the fallback (encrypted file) as its documented path.
2. ProbeTimeout: keep 3 s if every answering probe (a, e) is < 500 ms; otherwise set it to 3 × the slowest observed answer.
3. DefaultKDF: keep if encfile_open_ms on the reference laptop is ≤ 1500 ms; otherwise lower Time to 2 (memory stays 64 MiB) and update `DefaultKDF` + this table.
4. Setup UX for P0-3/P0-4: rows b, c, d show the two-choice screen (encrypted file / don't store); the passphrase prompt appears once per start in `ModeFile`; a locked keyring (row d) is treated as unavailable, and the Settings → Kite connection page offers "Switch to keyring" once a later start finds it unlocked.

## Follow-ups
- (one line per surprise, with the owning sub-plan)
```

- [ ] **Step 4: Commit**

```bash
git add internal/secrets/keyringprobe docs/architecture/spikes/S0-4-keyring.md
git commit -m "docs(spike): S0-4 keyring behaviour matrix, probe tool and backend decisions" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
