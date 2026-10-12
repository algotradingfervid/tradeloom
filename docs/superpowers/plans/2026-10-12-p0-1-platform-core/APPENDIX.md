## Acceptance-criteria coverage

| Requirement / AC | Proven by (task → test or step) | Remaining owner |
|---|---|---|
| FR-SHELL-018 — JSON logs `time`, `level`, `component`, `event` in `<data dir>/logs/` | T4 `TestNew_WritesJSONWithEventAndComponent`; T15 smoke log lines | — |
| FR-SHELL-018 — INFO default, DEBUG switchable at runtime without restart | T4 `TestRedactHandler_Enabled`; T15 `TestStart_LogLevelFollowsSettings` | P0-4 Settings → Diagnostics toggle |
| FR-SHELL-018 AC1 — no access token / API secret / request token in logs | T4 `TestRedactHandler_RemovesSecrets` (keys, registered values, query strings, header dumps, Telegram URLs, groups, errors, structs); T12 values registered on every keyring read/write | P0-2 Kite HTTP call-site tests; P0-3 login + 100 mock calls grep |
| FR-SHELL-018 AC2 — crash report + "crashed last time" on next start | T5 `TestGo_PanicWritesCrashReport`; T15 `TestStart_ReportsCrashLastTime` (`Boot.CrashedLastTime`, `Boot.CrashReports`) | P0-4 banner/view report |
| FR-SHELL-018 AC3 — ≤ 10 files × 20 MB | T4 `TestRotation_BoundsFilesUnderConcurrentWriters` (1:20 scale), `TestDefaults_MatchSpec` | — |
| FR-SHELL-018 AC4 — support bundle contents, no keyring value | T11 `TestCreateSupportBundle_ContentsAndNoSecrets`, `TestCreateSupportBundle_RejectsEscapingNames`, `TestSupportFiles` | P0-3 service wiring, P0-4 button |
| FR-DATA-005 — data dir, override, layout | T6 `TestResolveDataDir`, `TestLayoutEnsure`; T15 smoke listing | — |
| FR-DATA-005 — migrations in one tx after a backup copy | T7 `TestOpen_FailedMigrationRollsBackAndKeepsBackup`, `TestNewMigrator_Validation` | — |
| FR-DATA-005 AC1 — restore on machine B: all rows present, keyring empty | T9 `TestRestore_RoundTripToFreshDataDir` (no `secrets.enc`; secrets never archived) | P0-4 UI + two-VM demo (ROADMAP exit) |
| FR-DATA-005 AC2 — refuse newer schema with exact message | T7 `TestOpen_RefusesNewerSchema`; T15 `TestExitMessage` | — |
| FR-DATA-005 AC3 — archive contains no secret strings | T8 `TestBackup_ContentsAndNoSecrets` | — |
| FR-DATA-005 AC4 — failed migration leaves app.db unchanged + backup exists | T7 `TestOpen_FailedMigrationRollsBackAndKeepsBackup` | — |
| FR-DATA-005 AC5 — N = 7 retention after the new backup | T8 `TestScheduledBackup_Retention` | P0-3 daily-after-16:00 / on-quit trigger |
| FR-DATA-005 — restore preview counts, typed confirmation, deployments flat, `.replaced-<ts>` | T8 `Counts`; T9 `TestRestore_Refusals`, `TestRestore_RoundTripToFreshDataDir` | P0-4 dialog |
| FR-ACCT-013 AC1 — no daemon → exactly two choices, no plaintext | T14 `TestDetect`; T15 `TestStart_FirstRunCreatesEverything` (`ModeChoose`, 2 choices); T12 `TestNone` | P0-3/P0-4 setup screen |
| FR-ACCT-013 AC2 — no API secret in data dir; wrong passphrase fails | T13 `TestEncFile_NoPlaintextAndWrongPassphrase` (production KDF), `TestEncFile_TamperAndTruncation` | — |
| FR-ACCT-013 AC3 — switch to keyring moves secrets, deletes file | T14 `TestMigrate_FileToKeyring`, `TestMigrate_FailureRollsBackTargetAndKeepsSource` | P0-4 Settings button |
| FR-ACCT-013 — passphrase ≥ 8 chars, memory only | T13 `TestEncFile_Validation`, `TestEncFile_CloseZeroesAndDestroyRemoves` | P0-3 prompt once per start |
| NFR-01 / NFR-02 / C-11 — Linux build, .deb + AppImage, distros | T16 Steps 5–7 (`install-smoke` matrix); T17 | P0-5 exit demo VMs |
| NFR-03 — SQLite + versioned migrations | T7 | — |
| NFR-04 — secrets only in keyring/encrypted file, never logs/settings | T4, T10 `TestSet_RefusesSecrets`, T12–T14, T8/T11 grep tests | P0-3 netguard (outbound hosts) |
| NFR-09 — backup/restore | T8, T9 | — |
| NFR-10 — structured, redacted, rotated logs | T4, T5 | — |
| G-0 — no order-write path in P0–P3 builds | T2 `TestASTChecks_Fixture`, `TestArchitecture`, `TestCheckImports_Table` (`g0-import`), `TestStubRefusesEveryWrite`; T16 `g0-check` + CI | P0-3 recording mock suite (check 3) |
| §2.4 rules 1–8 | T2 `TestCheckImports_Table`, `TestASTChecks_Fixture`, `TestArchitecture` (runs on every later task); T16 go-arch-lint | later sub-plans extend `.go-arch-lint.yml` |
| ARCHITECTURE §3.7 single-instance (flock) | T6 `TestLock_SingleHolderAndExitMarker`; T15 `TestRun_SecondInstanceExits`, `TestStart_FirstRunCreatesEverything` | P0-3 Wails `SingleInstanceLock` focus (S0-8) |
| Bus drop/coalesce (§12 row 1c "done when") | T3 `TestSlowSubscriber_DropsOldestAndNeverBlocksPublisher`, `TestCoalesce_MergesInsteadOfDropping`, `TestCritical_WaitsThenLogsAndDrops` | — |
| S0-3 / S0-4 | T17, T18 | — |

## Self-review (done while writing; fixes applied inline)

1. **Spec coverage.** Every §12 row in scope (0, 1a, 1b, 1c, 2a) and every AC of FR-SHELL-018, FR-DATA-005 (engine) and FR-ACCT-013 maps to a task in the table above; UI halves and time-triggered parts are named with their owning sub-plan. Gaps found and fixed while writing: (a) a headless locked keyring could hang start-up → `ProbeKeyringWithin` + `TestProbeWithin_Timeout` added to Task 12 and used by `app.Start`; (b) Wails strips release binaries, which would make the `go tool nm` G-0 check pass vacuously → separate unstripped `g0-check` binary and a "no symbol table → exit 2" guard; (c) `frontend/dist` would be deleted by `vite build` and break `go:embed` → `public/.gitkeep` keeps `dist/.gitkeep`; (d) `.gitignore`'s old `frontend/dist/` rule would have hidden `.gitkeep` → replaced.
2. **Placeholder scan.** No placeholder markers; every code step contains the full file. Spike documents contain empty result cells by design — they are filled from the commands in the same task.
3. **Type consistency.** Names used across tasks were checked against the code: `store.Options{AppVersion, BackupDir, Now, Migrations, Log}`, `store.Layout.{AppDB, BackupsDir, LogsDir, ExportsDir, SecretsFile, LockFile, CandlesDB}`, `settings.Load(ctx, Backend, *bus.Bus)`, `bus.SubOpts[T]{Name, Buffer, Critical, Coalesce}`, `logging.Options{Dir, Level, MaxSizeMB, MaxBackups, Redactor, Stderr}`, `secrets.Detect(path, probe)`, `app.BootOptions{DataDir, AppVersion, PID, Now, Stderr, ProbeKeyring}`. Each task's code was compiled and its tests run in isolation in task order (tasks 1→18 cumulatively) with Go 1.25.5, `GOTOOLCHAIN=local`, `go test -tags noorders ./...`, plus `-race`, both `GOWORK=off` and workspace mode.
4. **Review Focus.** Each of the five items has its test in the owning task (T7, T12, T8, T9, T4) and all pass.

## Verification record (what was actually run while writing this plan)

- `go test -race -count=1 -tags webkit2_41,noorders ./...` on the assembled tree: all packages `ok` (macOS arm64, go1.25.5, GOWORK=off and workspace mode); `ta` tests still `ok`.
- `wails build -m -nosyncgomod -tags "webkit2_41,noorders"` from `cmd/tradeloom` (Wails CLI v2.16.0, macOS): built `build/bin/tradeloom.app`; `--smoke` run exited 0, wrote `app.db` (schema 1), `lock` = `clean`, JSON logs.
- `go-arch-lint check` (v1.19.0): `OK - No warnings found`; a planted keyring import in `internal/store` was reported.
- `scripts/g0-nm-check.sh`: ok on the unstripped noorders binary; exit 2 on a stripped binary; exit 1 on a probe binary calling `(*Client).PlaceOrder`.
- `go-licenses check` (v1.6.0, `CGO_ENABLED=0 GOOS=linux`): exit 0 with the allow-list; `make notices` deterministic.
- `nfpm package` (v2.47.0) with `build/nfpm.yaml`: produced the `.deb` with the listed control fields and contents.
- `npm install`, `vitest run`, `vite build` with the pinned frontend versions: 1 test passed, build ok.
- Not run locally (no Docker/Linux GUI on the authoring machine): `build/Dockerfile.linux`, `scripts/make-appimage.sh` (appimagetool), `scripts/install-smoke.sh`, the GitHub workflows (YAML parsed only). Their first real run is Task 16 Step 5–7 and Task 17.
