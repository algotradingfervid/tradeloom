# S0-4 — Keyring (spike)

Date: 2026-10-12 · Commit: see git log of this file · Decides: OQ-48, FR-ACCT-013, ARCHITECTURE §9.1

**Status: tool and procedure complete; Results: PENDING — run on Linux (Ubuntu 24.04 GNOME, KDE, headless SSH).**
The build host for this task is macOS with no Docker and no Linux desktop. The probe was deliberately NOT run
against the macOS login Keychain (it would write to the user's real keychain). No measured values exist yet; none are invented below.

## What is known from the code (no measurement needed)

- Backend: `zalando/go-keyring` v0.2.8 (`go.mod`), service name `tradeloom` (`secrets.KeyringService`), user attribute = the key (`kite.api_secret`, `kite.access_token`, `telegram.bot_token`).
- `ProbeKeyring` does a `Get("tradeloom","probe")`. Success or `ErrNotFound` means the Secret Service answered; any other error is wrapped as `ErrUnavailable` (no daemon, no session bus, locked/denied collection).
- `ProbeTimeout = 3s`. `ProbeKeyringWithin` runs the probe in a goroutine and, if no answer arrives in time, returns `ErrUnavailable: no answer from the Secret Service within 3s`. This exists because a locked collection can block on an unlock prompt that never appears headless. The goroutine is not cancelled (it may outlive the timeout until D-Bus returns).
- Error mapping for the live store (`wrapKeyring`): `keyring.ErrNotFound` to `ErrNotFound`; everything else to `ErrUnavailable` wrapping the cause. No fallback of any kind happens inside a keyring call; the caller surfaces the error. `Delete` of a missing key is a no-op (nil).
- `Detect(secretsFile, probe)`: an existing `secrets.enc` wins, then the `ModeFile` passphrase flow (even if the keyring is available; switching is explicit). Otherwise keyring available gives `ModeKeyring`; otherwise `ModeChoose` with exactly `{encrypted_file, no_storage}` (never a plaintext option).
- Encrypted file: Argon2id + XChaCha20-Poly1305, `DefaultKDF` t=3, m=64 MiB, p=4; minimum passphrase length 8 characters (`ErrPassphraseTooShort`); KDF params are stored in the header and bounded on read (max time 16).
- `Migrate(from, to)` (FR-ACCT-013 AC3): for each key in `AllKeys` present in `from`: snapshot the target's prior value (or absence) BEFORE writing, write to target, read back and compare. On any failure (read, snapshot, write, verify) the rollback restores each touched target key to its prior value (`Set`) or deletes it if it did not exist, and joins the rollback errors with the cause; the source is left untouched. This is the Task 13/14 hardening: a failed migration no longer destroys a pre-existing target value. Only after all keys succeed: if `from` is an `*EncFile` its file is `Destroy`ed; otherwise copied keys are deleted from `from`. A source-removal failure after a successful copy returns a distinct "migrated, but ..." error. Same-store migration is rejected.

## Procedure

Tool: `internal/secrets/keyringprobe`. It refuses to run (exit 2) unless `TRADELOOM_SPIKE_DISPOSABLE=1`, because it writes and deletes the real `tradeloom/telegram.bot_token` entry. Use only in throwaway containers/VMs. It prints one JSON object (env, mode, probe_error, probe_ms, keyring_roundtrip_ok/error/ms, encfile_create_ms, encfile_open_ms, migrate_file_to_keyring). Unit tests (go-keyring `MockInit`) cover the available and unavailable paths: `go test -tags noorders ./internal/secrets/keyringprobe`.

Prerequisite: the `tradeloom-build:p0` image from Task 16 (`make` target / Dockerfile for the Linux build). Build once (static): `mkdir -p build/bin && docker run --rm -e CGO_ENABLED=0 -v "$PWD":/src -w /src tradeloom-build:p0 go build -o build/bin/keyringprobe ./internal/secrets/keyringprobe`. Safety: `secrets.KeyringService` is a fixed constant, so the probe cannot use an isolated service name. Instead, before any write it does a `Get` of `tradeloom/telegram.bot_token`; if that entry already exists it aborts with exit code 3 and touches nothing. Cross-compile with `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` was verified to build on macOS.

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
# (d) daemon running but collection locked (no unlock, no display, prompt can never complete)
docker run --rm -v "$PWD":/src -w /src ubuntu:24.04 bash -c '
  apt-get update -qq && apt-get install -y -qq gnome-keyring dbus >/dev/null &&
  dbus-run-session -- sh -c "gnome-keyring-daemon --start --components=secrets >/dev/null;
    TRADELOOM_SPIKE_DISPOSABLE=1 build/bin/keyringprobe"'
```
Manual VM rows: (e) KDE Plasma desktop VM (Kubuntu 24.04 or Fedora KDE) with KWallet's Secret Service enabled, run from a desktop terminal; (f) the same GNOME VM reached over `ssh` (no desktop session in the SSH shell); (g) the reference laptop (record CPU) for `encfile_create_ms` / `encfile_open_ms`. Record every JSON output verbatim.

Concrete commands for the manual rows (replace `<arch>` with the VM's `uname -m` mapped to amd64/arm64, and `vm` with its host):
```bash
# e (KDE VM) and f/g: build for the target, copy, run
GOOS=linux GOARCH=<arch> CGO_ENABLED=0 go build -o build/bin/keyringprobe-linux ./internal/secrets/keyringprobe
# e: run from a terminal INSIDE the KDE desktop session
scp build/bin/keyringprobe-linux user@kde-vm:/tmp/keyringprobe && ssh -t user@kde-vm 'echo run it in the desktop terminal: TRADELOOM_SPIKE_DISPOSABLE=1 /tmp/keyringprobe'
# f: GNOME VM over plain ssh (no desktop session); first verify the env is really bare
scp build/bin/keyringprobe-linux user@gnome-vm:/tmp/keyringprobe && ssh user@gnome-vm 'env -u DBUS_SESSION_BUS_ADDRESS -u XDG_RUNTIME_DIR TRADELOOM_SPIKE_DISPOSABLE=1 /tmp/keyringprobe'
# f env check (must print nothing): ssh user@gnome-vm 'env | grep -E "^(DBUS_SESSION_BUS_ADDRESS|XDG_RUNTIME_DIR)="'
# g: reference laptop (Linux) - run locally, note CPU from `lscpu | grep "Model name"`
TRADELOOM_SPIKE_DISPOSABLE=1 build/bin/keyringprobe-linux
```
Row f is only valid if `DBUS_SESSION_BUS_ADDRESS` and `XDG_RUNTIME_DIR` are unset in the ssh shell (the `env -u` above enforces it; the JSON `env` block records the rest).

Expected shape: (a) `mode: keyring`, `keyring_roundtrip_ok: true`, `migrate_file_to_keyring: "ok (secrets.enc removed: true)"`; (b) and (c) `mode: choose` with a `probe_error` naming `org.freedesktop.secrets` / the missing bus; (d) `mode: choose` with `probe_error` containing `no answer from the Secret Service within 3s` (or an immediate locked error), never a hang.

## Results: PENDING — run on Linux (Ubuntu 24.04 GNOME, KDE, headless SSH)

### Behaviour matrix (`internal/secrets/keyringprobe`)
| Row | Environment | mode | probe_ms | probe_error | roundtrip | migrate file→keyring |
|---|---|---|---|---|---|---|
| a | GNOME Keyring unlocked (container) | PENDING | PENDING | PENDING | PENDING | PENDING |
| b | session bus, no daemon | PENDING | PENDING | PENDING | PENDING | PENDING |
| c | no session bus | PENDING | PENDING | PENDING | PENDING | PENDING |
| d | GNOME Keyring locked, no display | PENDING | PENDING | PENDING | PENDING | PENDING |
| e | KDE / KWallet Secret Service (VM) | PENDING | PENDING | PENDING | PENDING | PENDING |
| f | GNOME VM over ssh | PENDING | PENDING | PENDING | PENDING | PENDING |

### Argon2id cost (DefaultKDF t=3, m=64 MiB, p=4)
| Machine | encfile_create_ms | encfile_open_ms |
|---|---|---|
| Reference laptop (row g; CPU to be recorded) | PENDING | PENDING |

macOS login Keychain timing: intentionally not measured (would modify the user's keychain); macOS is not a P0 keyring target for this spike.

## Decision rules (apply once results exist)
1. OQ-48: zalando/go-keyring v0.2.8 stays the primary backend if rows a and e round-trip; otherwise record the failing desktop and the fallback (encrypted file) as its documented path.
2. ProbeTimeout: keep 3 s if every answering probe (a, e) is < 500 ms; otherwise set it to 3 × the slowest observed answer (edit `internal/secrets/keyring.go`, re-run its tests).
3. DefaultKDF: keep if encfile_open_ms on the reference laptop is ≤ 1500 ms; otherwise lower Time to 2 (memory stays 64 MiB), update `DefaultKDF` (`internal/secrets/encfile.go`) and this table.
4. Setup UX for P0-3/P0-4: rows b, c, d show the two-choice screen (encrypted file / don't store); the passphrase prompt appears once per start in `ModeFile`; a locked keyring (row d) is treated as unavailable, and the Settings → Kite connection page offers "Switch to keyring" once a later start finds it unlocked.

## Decisions
PENDING until the matrix above is filled. Current defaults (3 s probe timeout, t=3/m=64MiB/p=4, go-keyring v0.2.8 + `secrets.enc` fallback) remain in force and are unverified on real Linux desktops.

## Follow-ups
- None recorded yet (to be filled from the Linux run, one line per surprise, with the owning sub-plan).
