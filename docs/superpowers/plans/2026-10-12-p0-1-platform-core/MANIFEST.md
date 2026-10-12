# P0-1 Platform Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land Tradeloom's platform core — repo skeleton with a Wails hello window, architecture tests with the G-0 order-write gate, `logging`, `bus`, `store` (app.db v1, migrations, backup/restore), `settings`, `secrets`, the data-dir lock, the Linux Docker build, CI and `.deb`/AppImage packaging — plus spikes S0-3 and S0-4, so that P0-2…P0-5 build on tested foundations.

**Architecture:** One Go module `tradeloom` (Go 1.25, `replace tradeloom/ta => ./ta`) whose `internal/` packages are plain Go testable without a GUI; only `cmd/tradeloom` and `internal/app` import Wails. Startup is `app.Start`: data-dir layout → `flock` lock with last-exit marker → slog JSON logging with redaction and crash reports → `app.db` (modernc SQLite, WAL) integrity check, refuse-newer, pre-migration backup, single-transaction migration → bus → typed settings → secrets backend detection; then `wails.Run`. Architecture rules (§2.4 rules 1–8, G-0) are enforced from Task 2 onward by `internal/archtest` (import graph + AST) and later by go-arch-lint, `go tool nm` and CI.

**Tech Stack:** Go 1.25 · Wails v2.16.0 · React 19.3 + TypeScript 5.9 + Vite 8.3 + Vitest 5.0 · modernc.org/sqlite v1.59.0 · github.com/gofrs/flock v0.13.1 · gopkg.in/natefinch/lumberjack.v2 v2.2.1 · github.com/klauspost/compress v1.20.3 (zstd) · github.com/zalando/go-keyring v0.2.8 · golang.org/x/crypto v0.55.0 (argon2, chacha20poly1305) · golang.org/x/tools v0.49.0 (go/packages) · github.com/zerodha/gokiteconnect/v4 v4.4.3 (types only, G-0 stub) · go-arch-lint v1.19.0 · go-licenses v1.6.0 · nfpm v2.47.0 · appimagetool `continuous` · Docker `ubuntu:22.04` · GitHub Actions.

**Spec:** `docs/superpowers/plans/2026-10-12-p0-index.md` (scope + shared contract) · `docs/architecture/ARCHITECTURE.md` §2 (all), §3.2–3.4, §3.7–3.8, §4.1–4.2, §4.4–4.6, §9.1, §10, §11, §12 rows 0/1a/1b/1c/2a, §12.1 S0-3/S0-4 · `docs/requirements/REQUIREMENTS.md` FR-SHELL-018, FR-DATA-005, FR-ACCT-013, NFR-01…04, NFR-09, NFR-10, C-11 · `docs/plan/ROADMAP.md` §P0 exit criteria, §6 G-0.

## Global Constraints

- Go: `go 1.25.0` in `go.mod`/`go.work`; CI and Docker use go1.25.14; every shell, Makefile and CI step runs with `GOTOOLCHAIN=local` (a dependency that needs Go ≥ 1.26 must fail loudly, never switch toolchains).
- Module `tradeloom`, `replace tradeloom/ta => ./ta`; `go.work` = `use (. ./ta)` for developers; CI and Docker set `GOWORK=off`.
- Build/test tags: P0–P3 builds and CI use `-tags webkit2_41,noorders`; every test in this plan also passes with `go test -tags noorders ./...` on macOS and Linux.
- Pinned versions (verified 2026-10-12 against proxy.golang.org / npm / GitHub releases): Wails v2.16.0, modernc.org/sqlite **v1.59.0**, golang.org/x/crypto **v0.55.0**, golang.org/x/tools **v0.49.0**, go-keyring v0.2.8, flock v0.13.1, lumberjack v2.2.1, klauspost/compress v1.20.3, gokiteconnect v4.4.3. (ARCHITECTURE names sqlite v1.60.1 and x/crypto v0.58.0; both declare `go 1.26.0` and cannot be used with Go 1.25 — v1.59.0/v0.55.0 are the newest releases that declare `go 1.25.0`.)
- Frontend pins: react/react-dom 19.3.0, @types/react(-dom) 19.3.0, vite 8.3.4, @vitejs/plugin-react 6.1.2, typescript 5.9.3, vitest 5.0.3, @testing-library/react 16.3.3, @testing-library/dom 10.4.2, jsdom 30.1.2; Node 22.23.3 (jsdom 30 needs ≥ 22.22.2).
- Data dir: `~/.local/share/tradeloom`; `$TRADELOOM_DATA_DIR` (absolute) overrides; `$XDG_DATA_HOME/tradeloom` honoured; layout exactly ARCHITECTURE §4.5; directories 0700, files 0600.
- Dependency rules ARCHITECTURE §2.4 rules 1–8 and G-0 (§2.5) are enforced by `internal/archtest` from Task 2; no code outside `internal/clock` calls `time.Now/Since/Until/Sleep/After/AfterFunc/Tick/NewTimer/NewTicker` — pass `clock.SystemNow` (a `func() time.Time`) instead; tests may use `time` freely.
- Goroutines outside `internal/logging` and `internal/bus` start through `logging.Go(name, fn)`.
- Secrets (Kite API secret, access token, Telegram bot token) live only in `internal/secrets` backends; never in `app.db`, logs, events, backups or support bundles; no plaintext backend exists.
- Logs: JSON lines with `time`, `level`, `event` (the slog message), `component`; INFO default; rotation 20 MB, 9 rotated + 1 active = 10 files (ARCHITECTURE §3.8 says `MaxBackups: 10`, which would leave 11 files and break FR-SHELL-018 AC3).
- Go 1.25 is past upstream support (go1.27.2 and go1.26.9 are current on 2026-10-12). Staying on 1.25 is the spec's decision; moving to 1.26 would also unlock sqlite v1.60.x / x/crypto v0.58.0 and is the user's call, not this plan's.
- Wails project file is `cmd/tradeloom/wails.json` (Wails v2.16.0 compiles the main package in the directory holding `wails.json`: `pkg/commands/build/base.go` sets `cmd.Dir = b.projectData.Path`); output binary `build/bin/tradeloom`.
- Every commit message ends with the line `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Damaged `app.db` at start** (truncated by a crash or disk error, or overwritten with garbage) → `store.Open` returns `ErrCorrupt` naming the newest automatic backup, does not migrate, does not modify the file. Test: Task 7 `TestOpen_CorruptDatabase`.
2. **Keyring daemon disappears mid-session, or a locked collection waits forever for an unlock prompt** → every keyring call returns `secrets.ErrUnavailable`; nothing silently falls back to a file; the start-up probe gives up after `secrets.ProbeTimeout`. Tests: Task 12 `TestKeyringStore_DaemonGoneMidSession`, `TestProbeWithin_Timeout`.
3. **Disk full while writing a backup** → no file with the final archive name, no temp file left, error wraps `ENOSPC`, and a failed scheduled backup prunes nothing. Tests: Task 8 `TestBackup_DiskFullLeavesNothing`, `TestScheduledBackup_Retention`.
4. **Restore of an archive from a newer app (manifest or the `app.db` inside), or a hostile archive** (`../` paths, symlinks, missing `app.db`) → refused with `*SchemaTooNewError` / `ErrBadArchive`; current data dir byte-identical; staging removed. Test: Task 9 `TestRestore_RejectsHostileOrNewerArchives`.
5. **Log rotation under concurrent writers** (16 goroutines across many rotations) → every line in every file is valid JSON, ≤ 10 files, each ≤ the size cap. Test: Task 4 `TestRotation_BoundsFilesUnderConcurrentWriters`.

---

## File Structure

| Path | Task | Responsibility |
|---|---|---|
| `go.mod`, `go.sum` | 1 (deps added per task) | module `tradeloom`, Go 1.25, `ignore ./frontend/node_modules`, `replace tradeloom/ta => ./ta` |
| `go.work` | 1 | developer workspace `use (. ./ta)` |
| `.gitignore` | 1 | build outputs, `frontend/dist/*` except `.gitkeep`, `/dist/` packages |
| `Makefile` | 1, 16 | test/build/package entry points (Task 16 writes the final version) |
| `cmd/tradeloom/main.go` | 1, 15 | `main`: flags, data dir, `app.Start`, `wails.Run`, `--smoke` |
| `cmd/tradeloom/main_test.go` | 15 | second instance exits 1; bad flag exits 2 |
| `cmd/tradeloom/wails.json` | 1 | Wails project file (frontend dir, bindings dir, build dir) |
| `frontend/package.json`, `package-lock.json`, `tsconfig.json`, `vite.config.ts`, `index.html` | 1 | Vite + React + TS + Vitest scaffold |
| `frontend/src/main.tsx`, `App.tsx`, `App.test.tsx` | 1 | hello window UI + smoke test |
| `frontend/public/.gitkeep`, `frontend/dist/.gitkeep` | 1 | keep `dist/` present (Go embed) before and after `vite build` |
| `frontend/assets.go`, `assets_test.go` | 1 | Go package `tradeloom/frontend`: embeds `dist/` |
| `frontend/wailsjs/**` | 1 | generated by Wails, committed (ARCHITECTURE §8.1) |
| `internal/app/app.go`, `app_test.go` | 1 | bound `App`, `Version` (ldflags) |
| `internal/app/boot.go`, `boot_test.go` | 15 | `Start`/`Boot`/`Shutdown`/`ExitMessage`: pre-window startup sequence |
| `internal/clock/system.go` | 2 | `SystemNow` — the only wall-clock read (P0-2 extends the package) |
| `internal/kite/orderwrite/{doc,iface,writer_stub}.go`, `writer_stub_test.go` | 2 | G-0 gate: `Writer` interface + stub (`!orders \|\| noorders`) |
| `internal/archtest/{rules,ast}.go`, `{rules,ast,arch}_test.go` | 2 | rules 1–8 + G-0 as tests |
| `internal/archtest/testdata/fixture/**` | 2 | fixture module proving the AST checks fire |
| `internal/bus/bus.go`, `bus_test.go` | 3 | typed topics, bounded queues, coalescing, critical subscribers, drop counters |
| `internal/logging/{logging,handler,redact}.go`, `{logging,redact}_test.go` | 4 | JSON logger, rotation, `RedactHandler`, `Redactor`, `ParseLevel` |
| `internal/logging/crash.go`, `crash_test.go` | 5 | `Go`, `Recover`, crash reports, unseen-report listing |
| `internal/logging/bundle.go`, `bundle_test.go` | 11 | support bundle zip |
| `internal/store/{datadir,lock}.go`, `datadir_test.go` | 6 | data-dir resolution/layout; flock lock + last-exit marker |
| `internal/store/{store,migrate}.go`, `migrations/0001_init.sql`, `store_test.go` | 7 | open (WAL), integrity, refuse-newer, pre-migration backup, migrator, v1 schema |
| `internal/store/backup.go`, `backup_test.go` | 8 | `Backup`, `Counts`, `PruneBackups`, `ScheduledBackup` |
| `internal/store/restore.go`, `restore_test.go` | 9 | `InspectBackup`, `Restore` |
| `internal/store/settings_repo.go`, `settings_repo_test.go` | 10 | `SettingsRepo` (settings.Backend) |
| `internal/settings/{settings,keys}.go`, `settings_test.go` | 10 | typed keys, defaults, validation, change events |
| `internal/store/support.go`, `support_test.go` | 11 | `SupportFiles` (settings, versions, journal tail) |
| `internal/secrets/{secrets,keyring}.go`, `keyring_test.go` | 12 | `Store`, keys, errors, keyring backend, probe with timeout, `none` backend |
| `internal/secrets/encfile.go`, `encfile_test.go` | 13 | Argon2id + XChaCha20-Poly1305 `secrets.enc` |
| `internal/secrets/select.go`, `select_test.go` | 14 | backend detection (two choices), migration keyring ⇄ file |
| `.go-arch-lint.yml` | 16 | go-arch-lint v1.19.0 config |
| `build/Dockerfile.linux`, `build/nfpm.yaml`, `build/notices.tpl`, `build/linux/tradeloom.desktop`, `build/appimage/AppRun`, `build/appicon.png` | 16 (icon: 1) | Linux build image and packaging |
| `scripts/{g0-nm-check,gen-notices,make-appimage,install-smoke}.sh` | 16 | G-0 symbol check, notices, AppImage, install smoke |
| `THIRD_PARTY_NOTICES.md` | 16 | generated licence notices |
| `.github/workflows/ci.yml`, `release.yml` | 16 | CI jobs and tag release |
| `docs/architecture/spikes/S0-3-linux-build.md` | 17 | S0-3 results and decisions |
| `internal/secrets/keyringprobe/main.go` | 18 | S0-4 probe tool |
| `docs/architecture/spikes/S0-4-keyring.md` | 18 | S0-4 results and decisions |

Out of scope for P0-1 (owned elsewhere): `clock.Clock`/timers and `calendar` (P0-2), the scheduled-backup trigger "daily after 16:00 or on quit" (P0-3, needs `clock`), Wails `SingleInstanceLock` focus callback and event bridge (P0-3), every UI for logs/backup/secrets (P0-4), golangci-lint/forbidigo (not required by this sub-plan; archtest covers rule 8), the `ta` oracle CI job (C TA-Lib build).

## Prerequisites (developer machine)

- Go 1.25.x (`go version` ≥ go1.25.5), `export GOTOOLCHAIN=local`.
- Node 22.23.3 (any 22.x ≥ 22.22.2) and npm.
- Wails CLI: `GOTOOLCHAIN=local go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0`.
- Linux only: `sudo apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev`.
- Task 16 tools: `go install github.com/fe3dback/go-arch-lint@v1.19.0`, `go install github.com/google/go-licenses@v1.6.0`, Docker; nfpm v2.47.0 is installed inside the Docker image (its own module needs Go ≥ 1.26.4, so do not `go install` it with Go 1.25).

---


<!-- generated task table: begin (plan_split.py) -->
## Task files

Each implementer reads this manifest plus its one task file. Sizes are approximate (characters / 4).
"Consumes tasks" lists earlier tasks named in the task's Consumes lines. Dispatch in number order
unless Global Constraints or the P0 index state a different execution order.

| # | Task | File | ~tokens | Consumes tasks |
|---|---|---|---|---|
| 1 | Repository skeleton and Wails hello window | [`tasks/01-repository-skeleton-and-wails-hello-window.md`](tasks/01-repository-skeleton-and-wails-hello-window.md) | 2.8k | — |
| 2 | Architecture tests, `clock.SystemNow` and the G-0 order-write stub | [`tasks/02-architecture-tests-clock-systemnow-and-the.md`](tasks/02-architecture-tests-clock-systemnow-and-the.md) | 7.0k | 1 |
| 3 | `internal/bus` — typed pub/sub with bounded queues | [`tasks/03-internal-bus-typed-pub-sub-with.md`](tasks/03-internal-bus-typed-pub-sub-with.md) | 3.3k | — |
| 4 | `internal/logging` — JSON logger, rotation, redaction, runtime level | [`tasks/04-internal-logging-json-logger-rotation-redaction.md`](tasks/04-internal-logging-json-logger-rotation-redaction.md) | 4.3k | — |
| 5 | `internal/logging` — crash reports and `logging.Go` | [`tasks/05-internal-logging-crash-reports-and-logging.md`](tasks/05-internal-logging-crash-reports-and-logging.md) | 1.9k | 4 |
| 6 | `internal/store` — data directory and single-instance lock | [`tasks/06-internal-store-data-directory-and-single.md`](tasks/06-internal-store-data-directory-and-single.md) | 2.2k | — |
| 7 | `internal/store` — open app.db, migrator and schema v1 | [`tasks/07-internal-store-open-app-db-migrator.md`](tasks/07-internal-store-open-app-db-migrator.md) | 6.1k | 2, 6 |
| 8 | `internal/store` — Backup now, retention and scheduled backup | [`tasks/08-internal-store-backup-now-retention-and.md`](tasks/08-internal-store-backup-now-retention-and.md) | 4.1k | 6, 7 |
| 9 | `internal/store` — Restore | [`tasks/09-internal-store-restore.md`](tasks/09-internal-store-restore.md) | 3.7k | 6, 7, 8 |
| 10 | `internal/settings` and `store.SettingsRepo` | [`tasks/10-internal-settings-and-store-settingsrepo.md`](tasks/10-internal-settings-and-store-settingsrepo.md) | 3.7k | 3, 4, 7 |
| 11 | Support bundle (`logging.CreateSupportBundle` + `store.SupportFiles`) | [`tasks/11-support-bundle.md`](tasks/11-support-bundle.md) | 2.9k | 4, 5, 7, 8, 10 |
| 12 | `internal/secrets` — Store interface, keyring backend, probe, `none` backend | [`tasks/12-internal-secrets-store-interface-keyring-backend.md`](tasks/12-internal-secrets-store-interface-keyring-backend.md) | 2.5k | 4, 5 |
| 13 | `internal/secrets` — encrypted file backend (`secrets.enc`) | [`tasks/13-internal-secrets-encrypted-file-backend.md`](tasks/13-internal-secrets-encrypted-file-backend.md) | 3.2k | 4, 12 |
| 14 | `internal/secrets` — backend detection and migration keyring ⇄ file | [`tasks/14-internal-secrets-backend-detection-and-migration.md`](tasks/14-internal-secrets-backend-detection-and-migration.md) | 2.0k | 12, 13 |
| 15 | Startup wiring — `app.Start`, clean shutdown, `--smoke`, second-instance exit | [`tasks/15-startup-wiring-app-start-clean-shutdown.md`](tasks/15-startup-wiring-app-start-clean-shutdown.md) | 3.9k | 1, 2, 3, 4, 5, 6, 7, 10, 12, 14 |
| 16 | go-arch-lint, G-0 symbol check, licences, Docker Linux build, packaging and CI | [`tasks/16-go-arch-lint-g-0-symbol.md`](tasks/16-go-arch-lint-g-0-symbol.md) | 5.7k | 1, 15 |
| 17 | Spike S0-3 — Linux build pipeline and install matrix | [`tasks/17-spike-s0-3-linux-build-pipeline.md`](tasks/17-spike-s0-3-linux-build-pipeline.md) | 1.0k | 15, 16 |
| 18 | Spike S0-4 — keyring behaviour, passphrase UX, migration | [`tasks/18-spike-s0-4-keyring-behaviour-passphrase.md`](tasks/18-spike-s0-4-keyring-behaviour-passphrase.md) | 2.2k | 2, 12 |

Coverage and self-review: [`APPENDIX.md`](APPENDIX.md).
<!-- generated task table: end -->