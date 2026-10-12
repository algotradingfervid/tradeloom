> **P0-3 Session & Service Layer Implementation Plan — Task 30.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 30: Data façade — stats, clear candles, instrument refresh, backup/restore jobs and the daily backup (FR-DATA-002/005)

**Files:**
- Create: `internal/service/data.go`
- Modify: `internal/service/deps.go` (Task 14: `CandleAdmin`, `InstrumentRefresher`, seven `Deps` fields, the job state on `Service`)
- Test: `internal/service/data_test.go`

**Interfaces:**
- Consumes: P0-1 `store.Layout` (`Root`, `AppDB`, `CandlesDB`, `BackupsDir`), `store.NewLayout`, `(Layout).Ensure`, `store.Open`, `store.EmbeddedMigrations`, `store.SupportedSchema`, `(*DB).SchemaVersion`, `(*DB).Backup`, `(*DB).ScheduledBackup`, `store.BackupOptions`, `store.Manifest`, `store.InspectBackup`, `store.Restore`, `store.RestoreRequest`, `store.RestoreResult`, `store.RestoreConfirmPhrase`, `store.ErrBadArchive`, `store.ErrNotConfirmed`, `store.ErrDeploymentsActive`; P0-1 settings `BackupScheduleEnabled`, `BackupScheduleFolder`, `BackupScheduleKeep`, `BackupIncludeCandles`, `settings.Get`; P0-2 `candles.Stats`, `candles.IntervalStats`, `*candles.RunsReferencedError`, `candles.Job`, `(*calendar.Service).Version`; Task 21 `TopicJobProgress`, `JobProgress`; Task 28 `store.WindowStateRepo` (the window_state key backup.last_scheduled); `logging.Go`.
- Produces:
  ```go
  type CandleAdmin interface { Stats(ctx context.Context) (candles.Stats, error); Clear(ctx context.Context, confirmRuns []string) error; OpenJobs(ctx context.Context) ([]candles.Job, error) } // *sqlite.Store
  type InstrumentRefresher interface { Refresh(ctx context.Context) error }                                                                                                             // *instruments.Refresher
  // Deps gains: DB *store.DB; Layout store.Layout; AppVersion string; CandleAdmin CandleAdmin; Refresher InstrumentRefresher; ImportedAt func() time.Time; CloseForRestore func() error
  type DataStats struct{ DataDir string; AppDBBytes, CandleBytes, CandleBudgetBytes, InstrumentsImportedAtMs int64; BackfillOpenJobs int } // TS service.DataStats
  type BackupInfo struct{ Path string; OK bool; Reason string; Schema, SupportedSchema int; AppVersion, CreatedAt string; Counts map[string]int; IncludesCandles bool; SizeBytes int64 } // TS service.BackupInfo
  const UiStateLastScheduledBackup = "backup.last_scheduled"; const ScheduledBackupHour = 16 // IST
  const JobBackup, JobRestore, JobSupportBundle = "backup", "restore", "support_bundle"
  func (s *Service) beginJob(kind string) (id string, release func(), err error) // one job at a time: job_running (also used by Task 32)
  func (s *Service) progress(p JobProgress)                                       // publishes TopicJobProgress
  func (s *Service) DataStats(ctx context.Context) (DataStats, error)
  func (s *Service) ClearCandles(ctx context.Context) error                                            // runs_referenced
  func (s *Service) RefreshInstruments(ctx context.Context) error                                      // not_logged_in, refresh_failed
  func (s *Service) BackupNow(ctx context.Context, folder string, includeCandles bool) (string, error) // invalid_folder, job_running; stages preparing|writing|verifying|done
  func (s *Service) InspectBackup(ctx context.Context, path string) (BackupInfo, error)                // not_found; a bad archive is OK=false with Reason, not an error
  func (s *Service) RestoreBackup(ctx context.Context, path, confirmation string) (string, error)      // not_confirmed, bad_archive, schema_too_new, deployments_active, job_running; stages inspecting|closing|restoring|restart
  func (s *Service) BackupOnQuit(ctx context.Context) // runs the scheduled backup synchronously if it is due
  func (s *Service) RunBackups(ctx context.Context)   // waits for 16:00 IST each day and runs the scheduled backup if due
  ```

Backups and restores are jobs: the binding returns a job id at once and the UI follows `job.progress` (`{jobId, kind, stage, done, total, finished, error, resultPath}`). Only one job runs at a time (`job_running`); jobs run on `context.WithoutCancel` of the binding context so a closed dialog does not abort a half-written archive. `BackupNow` writes `<folder>/tradeloom-backup-YYYYMMDD-HHMMSS.tar.zst` through P0-1's `DB.Backup` and verifies it with `store.InspectBackup` before reporting `done`. `RestoreBackup` requires the typed phrase `RESTORE`, inspects the archive, refuses one made by a newer schema, closes the platform and `app.db` (`Deps.CloseForRestore`, Task 23), calls `store.Restore` and ends with stage `restart`: the app quits with exit code 3 and `main` re-execs (Task 22/23). The scheduled backup (FR-DATA-005 AC4) runs once per day after 16:00 IST when `backup.schedule.enabled` is on, at the first 16:00 the app is running or at the next quit, keeping `backup.schedule.keep` archives in `backup.schedule.folder` (empty = the data directory's `backups/`); the day of the last run is `window_state` key `backup.last_scheduled`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/service/data_test.go
package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/candles"
	"tradeloom/internal/kite"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

type fakeCandleAdmin struct {
	stats    candles.Stats
	clearErr error
	cleared  int
	jobs     []candles.Job
}

func (f *fakeCandleAdmin) Stats(context.Context) (candles.Stats, error) { return f.stats, nil }
func (f *fakeCandleAdmin) Clear(context.Context, []string) error {
	if f.clearErr != nil {
		return f.clearErr
	}
	f.cleared++
	return nil
}
func (f *fakeCandleAdmin) OpenJobs(context.Context) ([]candles.Job, error) { return f.jobs, nil }

type fakeRefresher struct {
	err   error
	calls int
}

func (f *fakeRefresher) Refresh(context.Context) error { f.calls++; return f.err }

// withData gives the env a real app.db inside a real data-directory layout, so backups and restores run for real.
func withData(t *testing.T, e *svcEnv) (store.Layout, *store.DB, *fakeCandleAdmin, *fakeRefresher) {
	t.Helper()
	layout := store.NewLayout(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(context.Background(), layout.AppDB, store.Options{
		AppVersion: "1.2.3", BackupDir: layout.BackupsDir, Now: e.clock.Now, Migrations: store.EmbeddedMigrations(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ca := &fakeCandleAdmin{stats: candles.Stats{Intervals: []candles.IntervalStats{{Interval: candles.Interval("minute"), Rows: 10, Bytes: 4096}}, TotalBytes: 4096, BudgetBytes: 2 << 30}}
	rf := &fakeRefresher{}
	e.svc.d.DB, e.svc.d.Layout, e.svc.d.AppVersion = db, layout, "1.2.3"
	e.svc.d.Calendar = mustCalendar(t)
	e.svc.d.CandleAdmin, e.svc.d.Refresher = ca, rf
	e.svc.d.ImportedAt = func() time.Time { return ist(2026, 10, 12, 8, 30) }
	e.svc.d.CloseForRestore = db.Close
	e.svc.d.UiState = store.NewWindowStateRepo(db)
	return layout, db, ca, rf
}

// waitJob collects job.progress events for one job until it finishes and returns the stages seen and the final event.
func waitJob(t *testing.T, ch chan emitted, id string) ([]string, JobProgress) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	var stages []string
	for {
		select {
		case ev := <-ch:
			jp, ok := ev.payload.(JobProgress)
			if !ok || jp.JobID != id {
				continue
			}
			stages = append(stages, jp.Stage)
			if jp.Finished {
				return stages, jp
			}
		case <-deadline:
			t.Fatalf("job %s did not finish; stages %v", id, stages)
		}
	}
}

func TestDataStats_ClearCandles_RefreshInstruments(t *testing.T) {
	e := newSvcEnv(t)
	layout, _, ca, rf := withData(t, e)
	ctx := context.Background()
	ca.jobs = []candles.Job{{ID: "j1"}, {ID: "j2"}}
	st, err := e.svc.DataStats(ctx)
	if err != nil || st.DataDir != layout.Root || st.AppDBBytes <= 0 || st.CandleBytes != 4096 || st.CandleBudgetBytes != 2<<30 ||
		st.InstrumentsImportedAtMs != ist(2026, 10, 12, 8, 30).UnixMilli() || st.BackfillOpenJobs != 2 {
		t.Fatalf("%+v %v", st, err)
	}
	ca.clearErr = &candles.RunsReferencedError{Runs: []string{"run-1", "run-2"}}
	if err := e.svc.ClearCandles(ctx); refusedCode(t, err) != "runs_referenced" || err.Error() != "2 backtest runs still reference cached candles." {
		t.Fatalf("%v", err)
	}
	ca.clearErr = nil
	if err := e.svc.ClearCandles(ctx); err != nil || ca.cleared != 1 {
		t.Fatalf("%v %d", err, ca.cleared)
	}
	e.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	if err := e.svc.RefreshInstruments(ctx); refusedCode(t, err) != "not_logged_in" {
		t.Fatal(err)
	}
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect})
	rf.err = errors.New("GET /instruments: 503")
	if err := e.svc.RefreshInstruments(ctx); refusedCode(t, err) != "refresh_failed" {
		t.Fatal(err)
	}
	rf.err = nil
	if err := e.svc.RefreshInstruments(ctx); err != nil || rf.calls != 3 {
		t.Fatalf("%v %d", err, rf.calls)
	}
}

// FR-DATA-005 AC1/AC2: backup now, inspect, and the refusals of a second job and a bad folder.
func TestBackupNow_RunsStagesAndInspects(t *testing.T) {
	e := newSvcEnv(t)
	layout, _, _, _ := withData(t, e)
	ch := startEvents(t, e)
	ctx := context.Background()
	if _, err := e.svc.BackupNow(ctx, filepath.Join(layout.Root, "missing"), false); refusedCode(t, err) != "invalid_folder" {
		t.Fatal(err)
	}
	id, err := e.svc.BackupNow(ctx, "", false)
	if err != nil || id == "" {
		t.Fatalf("%q %v", id, err)
	}
	stages, last := waitJob(t, ch, id)
	if len(stages) != 4 || stages[0] != "preparing" || stages[1] != "writing" || stages[2] != "verifying" || stages[3] != "done" || last.Error != "" || last.Kind != JobBackup || last.Done != 4 || last.Total != 4 {
		t.Fatalf("%v %+v", stages, last)
	}
	if filepath.Dir(last.ResultPath) != layout.BackupsDir || filepath.Ext(last.ResultPath) != ".zst" {
		t.Fatalf("result %q", last.ResultPath)
	}
	info, err := e.svc.InspectBackup(ctx, last.ResultPath)
	if err != nil || !info.OK || info.Schema != store.SupportedSchema() || info.SupportedSchema != store.SupportedSchema() || info.AppVersion != "1.2.3" || info.IncludesCandles || info.SizeBytes <= 0 || info.CreatedAt == "" {
		t.Fatalf("%+v %v", info, err)
	}
	if _, err := e.svc.InspectBackup(ctx, filepath.Join(layout.Root, "nope.tar.zst")); refusedCode(t, err) != "not_found" {
		t.Fatal(err)
	}
	bad := filepath.Join(layout.ExportsDir, "bad.tar.zst")
	if err := os.WriteFile(bad, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err = e.svc.InspectBackup(ctx, bad)
	if err != nil || info.OK || info.Reason == "" {
		t.Fatalf("bad archive must be OK=false: %+v %v", info, err)
	}
	// a second job while one runs
	rel := holdJob(t, e.svc)
	if _, err := e.svc.BackupNow(ctx, "", false); refusedCode(t, err) != "job_running" {
		t.Fatal(err)
	}
	rel()
}

// FR-DATA-005 AC3: restore needs the phrase, a good archive and a supported schema; it ends with stage restart.
func TestRestoreBackup_StagesAndRefusals(t *testing.T) {
	e := newSvcEnv(t)
	layout, db, _, _ := withData(t, e)
	ch := startEvents(t, e)
	ctx := context.Background()
	id, err := e.svc.BackupNow(ctx, "", false)
	if err != nil {
		t.Fatal(err)
	}
	_, last := waitJob(t, ch, id)
	archive := last.ResultPath
	if _, err := e.svc.RestoreBackup(ctx, archive, "restore"); refusedCode(t, err) != "not_confirmed" {
		t.Fatal(err)
	}
	if _, err := e.svc.RestoreBackup(ctx, filepath.Join(layout.Root, "nope.tar.zst"), store.RestoreConfirmPhrase); refusedCode(t, err) != "bad_archive" {
		t.Fatal(err)
	}
	e.svc.d.supportedSchema = func() int { return 0 } // pretend this build is older than the archive
	if _, err := e.svc.RestoreBackup(ctx, archive, store.RestoreConfirmPhrase); refusedCode(t, err) != "schema_too_new" {
		t.Fatal(err)
	}
	e.svc.d.supportedSchema = nil
	closed := 0
	e.svc.d.CloseForRestore = func() error { closed++; return db.Close() }
	id, err = e.svc.RestoreBackup(ctx, archive, store.RestoreConfirmPhrase)
	if err != nil || id == "" {
		t.Fatalf("%q %v", id, err)
	}
	stages, last := waitJob(t, ch, id)
	if len(stages) != 4 || stages[0] != "inspecting" || stages[1] != "closing" || stages[2] != "restoring" || stages[3] != "restart" || last.Error != "" || last.Kind != JobRestore {
		t.Fatalf("%v %+v", stages, last)
	}
	if closed != 1 || last.ResultPath == "" {
		t.Fatalf("closed %d result %q", closed, last.ResultPath)
	}
	if _, err := os.Stat(layout.AppDB); err != nil {
		t.Fatalf("restored app.db missing: %v", err)
	}
	if _, err := os.Stat(last.ResultPath); err != nil {
		t.Fatalf("replaced directory %q missing: %v", last.ResultPath, err)
	}
	re, err := store.Open(ctx, layout.AppDB, store.Options{AppVersion: "1.2.3", Migrations: store.EmbeddedMigrations(), Now: e.clock.Now})
	if err != nil {
		t.Fatalf("restored app.db does not open: %v", err)
	}
	re.Close()
}

// FR-DATA-005 AC4: the scheduled backup runs once after 16:00 IST, and at quit when it was missed.
func TestScheduledBackup_DailyAfter1600AndOnQuit(t *testing.T) {
	e := newSvcEnv(t)
	layout, db, _, _ := withData(t, e)
	ctx := context.Background()
	if err := settings.Set(ctx, e.settings, settings.BackupScheduleEnabled, true); err != nil {
		t.Fatal(err)
	}
	if err := settings.Set(ctx, e.settings, settings.BackupScheduleKeep, 2); err != nil {
		t.Fatal(err)
	}
	e.clock.Set(ist(2026, 10, 12, 15, 59))
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go e.svc.RunBackups(rctx)
	time.Sleep(20 * time.Millisecond)
	if n := archives(t, layout.BackupsDir); n != 0 {
		t.Fatalf("%d archives before 16:00", n)
	}
	e.clock.Advance(time.Minute)
	eventually(t, "scheduled backup", func() bool { return archives(t, layout.BackupsDir) == 1 })
	if v, _ := store.NewWindowStateRepo(db).Get(ctx, UiStateLastScheduledBackup); v != "2026-10-12" {
		t.Fatalf("marker %q", v)
	}
	cancel() // stop the loop so the quit path below is the only runner
	time.Sleep(20 * time.Millisecond)
	e.svc.BackupOnQuit(ctx) // same day: nothing more
	if n := archives(t, layout.BackupsDir); n != 1 {
		t.Fatalf("%d archives after same-day quit", n)
	}
	e.clock.Set(ist(2026, 10, 13, 18, 0)) // next day, the app was not running at 16:00
	e.svc.BackupOnQuit(ctx)
	if n := archives(t, layout.BackupsDir); n != 2 {
		t.Fatalf("%d archives after next-day quit", n)
	}
	e.clock.Set(ist(2026, 10, 14, 18, 0))
	e.svc.BackupOnQuit(ctx)
	if n := archives(t, layout.BackupsDir); n != 2 { // keep = 2 pruned the oldest
		t.Fatalf("%d archives after pruning", n)
	}
	if err := settings.Set(ctx, e.settings, settings.BackupScheduleEnabled, false); err != nil {
		t.Fatal(err)
	}
	e.clock.Set(ist(2026, 10, 15, 18, 0))
	e.svc.BackupOnQuit(ctx)
	if n := archives(t, layout.BackupsDir); n != 2 {
		t.Fatalf("%d archives while disabled", n)
	}
}

// holdJob occupies the single job slot from a test.
func holdJob(t *testing.T, s *Service) func() {
	t.Helper()
	_, rel, err := s.beginJob("test")
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func archives(t *testing.T, dir string) int {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "tradeloom-backup-*.tar.zst"))
	if err != nil {
		t.Fatal(err)
	}
	return len(m)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run 'TestDataStats|TestBackupNow|TestRestoreBackup|TestScheduledBackup' -v`
Expected: FAIL to compile with `undefined: (*Service).DataStats`, `e.svc.d.DB undefined`.

- [ ] **Step 3: Extend `deps.go`**

Add the interfaces, the `Deps` fields (after `Quotes`) and the job state; import `"tradeloom/internal/store"` is already present.

```go
// CandleAdmin is the maintenance half of *sqlite.Store (FR-DATA-002).
type CandleAdmin interface {
	Stats(ctx context.Context) (candles.Stats, error)
	Clear(ctx context.Context, confirmRuns []string) error
	OpenJobs(ctx context.Context) ([]candles.Job, error)
}

// InstrumentRefresher is *instruments.Refresher.
type InstrumentRefresher interface {
	Refresh(ctx context.Context) error
}
```

```go
	DB              *store.DB           // app.db (backups, calendar overrides, support files) (Task 30)
	Layout          store.Layout        // data directory layout (Task 30)
	AppVersion      string              // app.Version (Task 30)
	CandleAdmin     CandleAdmin         // candle stats / clear (Task 30)
	Refresher       InstrumentRefresher // manual instrument refresh (Task 30)
	ImportedAt      func() time.Time    // Master.ImportedAt (Task 30)
	CloseForRestore func() error        // stops the platform and closes app.db before store.Restore (Task 30)
	supportedSchema func() int          // store.SupportedSchema; tests override it
```

and on `Service` (after `deferred`):

```go
	jobMu      sync.Mutex
	jobRunning string // kind of the running job, "" when idle
	jobSeq     int
```

- [ ] **Step 4: Write the implementation**

```go
// internal/service/data.go
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/candles"
	"tradeloom/internal/clock"
	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

const (
	dataStage                  = "data"
	UiStateLastScheduledBackup = "backup.last_scheduled"
	ScheduledBackupHour        = 16 // IST, after the NSE close
	JobBackup                  = "backup"
	JobRestore                 = "restore"
	JobSupportBundle           = "support_bundle"
)

// DataStats is Settings → Data's summary (FR-DATA-002).
type DataStats struct {
	DataDir                 string `json:"dataDir"`
	AppDBBytes              int64  `json:"appDbBytes"`
	CandleBytes             int64  `json:"candleBytes"`
	CandleBudgetBytes       int64  `json:"candleBudgetBytes"`
	InstrumentsImportedAtMs int64  `json:"instrumentsImportedAtMs"`
	BackfillOpenJobs        int    `json:"backfillOpenJobs"`
}

// BackupInfo is what the restore dialog shows before the user types the phrase (FR-DATA-005 AC3).
type BackupInfo struct {
	Path            string         `json:"path"`
	OK              bool           `json:"ok"`
	Reason          string         `json:"reason"`
	Schema          int            `json:"schema"`
	SupportedSchema int            `json:"supportedSchema"`
	AppVersion      string         `json:"appVersion"`
	CreatedAt       string         `json:"createdAt"`
	Counts          map[string]int `json:"counts"`
	IncludesCandles bool           `json:"includesCandles"`
	SizeBytes       int64          `json:"sizeBytes"`
}

func (s *Service) supportedSchema() int {
	if s.d.supportedSchema != nil {
		return s.d.supportedSchema()
	}
	return store.SupportedSchema()
}

// beginJob claims the single job slot; release frees it.
func (s *Service) beginJob(kind string) (string, func(), error) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if s.jobRunning != "" {
		return "", nil, refused(dataStage, "job_running", "Another job ("+s.jobRunning+") is still running.")
	}
	s.jobSeq++
	id := fmt.Sprintf("%s-%d", kind, s.jobSeq)
	s.jobRunning = kind
	return id, func() {
		s.jobMu.Lock()
		s.jobRunning = ""
		s.jobMu.Unlock()
	}, nil
}

func (s *Service) progress(p JobProgress) { bus.Publish(s.d.Bus, TopicJobProgress, p) }

func (s *Service) DataStats(ctx context.Context) (DataStats, error) {
	out := DataStats{DataDir: s.d.Layout.Root}
	if fi, err := os.Stat(s.d.Layout.AppDB); err == nil {
		out.AppDBBytes = fi.Size()
	}
	if s.d.CandleAdmin != nil {
		st, err := s.d.CandleAdmin.Stats(ctx)
		if err != nil {
			return out, asRefused(dataStage, err)
		}
		out.CandleBytes, out.CandleBudgetBytes = st.TotalBytes, st.BudgetBytes
		jobs, err := s.d.CandleAdmin.OpenJobs(ctx)
		if err != nil {
			return out, asRefused(dataStage, err)
		}
		out.BackfillOpenJobs = len(jobs)
	}
	if s.d.ImportedAt != nil {
		if at := s.d.ImportedAt(); !at.IsZero() {
			out.InstrumentsImportedAtMs = at.UnixMilli()
		}
	}
	return out, nil
}

// ClearCandles drops the candle cache (FR-DATA-002); refused while backtest runs reference it.
func (s *Service) ClearCandles(ctx context.Context) error {
	if s.d.CandleAdmin == nil {
		return refused(dataStage, "unavailable", "The candle store is not available.")
	}
	err := s.d.CandleAdmin.Clear(ctx, nil)
	var rr *candles.RunsReferencedError
	if errors.As(err, &rr) {
		return refused(dataStage, "runs_referenced", fmt.Sprintf("%d backtest runs still reference cached candles.", len(rr.Runs)))
	}
	return asRefused(dataStage, err)
}

// RefreshInstruments re-downloads the instrument master now (FR-WATCH-008 manual refresh).
func (s *Service) RefreshInstruments(ctx context.Context) error {
	if s.d.Session.State().State != kite.StateReady {
		return refused(dataStage, "not_logged_in", "Log in to refresh the instrument list.")
	}
	if s.d.Refresher == nil {
		return refused(dataStage, "unavailable", "The instrument refresher is not available.")
	}
	if err := s.d.Refresher.Refresh(ctx); err != nil {
		return refused(dataStage, "refresh_failed", "The instrument list could not be refreshed: "+err.Error())
	}
	return nil
}

func (s *Service) backupOptions(dir string, includeCandles bool) store.BackupOptions {
	o := store.BackupOptions{Dir: dir, IncludeCandles: includeCandles, CandlesDB: s.d.Layout.CandlesDB, AppVersion: s.d.AppVersion, Now: s.d.Clock.Now}
	if s.d.Calendar != nil {
		o.CalendarVersion = s.d.Calendar.Version()
	}
	return o
}

// BackupNow starts a backup job into folder ("" = the data directory's backups/).
func (s *Service) BackupNow(ctx context.Context, folder string, includeCandles bool) (string, error) {
	if s.d.DB == nil {
		return "", refused(dataStage, "unavailable", "The database is not open.")
	}
	if folder == "" {
		folder = s.d.Layout.BackupsDir
	}
	if fi, err := os.Stat(folder); err != nil || !fi.IsDir() || !filepath.IsAbs(folder) {
		return "", refused(dataStage, "invalid_folder", "Choose an existing folder for the backup.")
	}
	id, release, err := s.beginJob(JobBackup)
	if err != nil {
		return "", err
	}
	jctx := context.WithoutCancel(ctx)
	logging.Go("service.backup", func() {
		defer release()
		step := func(stage string, n int) { s.progress(JobProgress{JobID: id, Kind: JobBackup, Stage: stage, Done: n, Total: 4}) }
		fail := func(stage string, n int, err error) {
			s.d.Log.Error("backup failed", "stage", stage, "err", err)
			s.progress(JobProgress{JobID: id, Kind: JobBackup, Stage: stage, Done: n, Total: 4, Finished: true, Error: err.Error()})
		}
		step("preparing", 1)
		step("writing", 2)
		path, err := s.d.DB.Backup(jctx, s.backupOptions(folder, includeCandles))
		if err != nil {
			fail("writing", 2, err)
			return
		}
		step("verifying", 3)
		if _, err := store.InspectBackup(path); err != nil {
			fail("verifying", 3, err)
			return
		}
		s.d.Log.Info("backup written", "path", path)
		s.progress(JobProgress{JobID: id, Kind: JobBackup, Stage: "done", Done: 4, Total: 4, Finished: true, ResultPath: path})
	})
	return id, nil
}

// InspectBackup describes an archive for the restore dialog; an unreadable archive is OK=false, not an error.
func (s *Service) InspectBackup(_ context.Context, path string) (BackupInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return BackupInfo{}, refused(dataStage, "not_found", "The backup file was not found.")
	}
	info := BackupInfo{Path: path, SupportedSchema: s.supportedSchema(), SizeBytes: fi.Size(), Counts: map[string]int{}}
	m, err := store.InspectBackup(path)
	if err != nil {
		info.Reason = "This file is not a Tradeloom backup: " + err.Error()
		return info, nil
	}
	info.Schema, info.AppVersion, info.CreatedAt, info.IncludesCandles = m.Schema, m.AppVersion, m.CreatedAt, m.IncludesCandles
	if m.Counts != nil {
		info.Counts = m.Counts
	}
	if m.Schema > info.SupportedSchema {
		info.Reason = fmt.Sprintf("This backup was made by a newer version (schema %d; this build supports %d).", m.Schema, info.SupportedSchema)
		return info, nil
	}
	info.OK = true
	return info, nil
}

// RestoreBackup starts a restore job; the app restarts when it finishes (FR-DATA-005 AC3).
func (s *Service) RestoreBackup(ctx context.Context, path, confirmation string) (string, error) {
	if confirmation != store.RestoreConfirmPhrase {
		return "", refused(dataStage, "not_confirmed", "Type "+store.RestoreConfirmPhrase+" to confirm.")
	}
	if _, err := os.Stat(path); err != nil {
		return "", refused(dataStage, "bad_archive", "The backup file was not found.")
	}
	m, err := store.InspectBackup(path)
	if err != nil {
		return "", refused(dataStage, "bad_archive", "This file is not a Tradeloom backup: "+err.Error())
	}
	if m.Schema > s.supportedSchema() {
		return "", refused(dataStage, "schema_too_new", fmt.Sprintf("This backup was made by a newer version (schema %d; this build supports %d).", m.Schema, s.supportedSchema()))
	}
	if s.d.CloseForRestore == nil {
		return "", refused(dataStage, "unavailable", "Restore is not available in this build.")
	}
	id, release, err := s.beginJob(JobRestore)
	if err != nil {
		return "", err
	}
	jctx := context.WithoutCancel(ctx)
	logging.Go("service.restore", func() {
		defer release()
		step := func(stage string, n int) { s.progress(JobProgress{JobID: id, Kind: JobRestore, Stage: stage, Done: n, Total: 4}) }
		fail := func(stage string, n int, err error) {
			s.d.Log.Error("restore failed", "stage", stage, "err", err)
			s.progress(JobProgress{JobID: id, Kind: JobRestore, Stage: stage, Done: n, Total: 4, Finished: true, Error: err.Error()})
		}
		step("inspecting", 1)
		step("closing", 2)
		if err := s.d.CloseForRestore(); err != nil {
			fail("closing", 2, err)
			return
		}
		step("restoring", 3)
		res, err := store.Restore(jctx, store.RestoreRequest{Archive: path, DataDir: s.d.Layout.Root, Confirmation: confirmation, DeploymentsFlat: true, Now: s.d.Clock.Now})
		switch {
		case errors.Is(err, store.ErrDeploymentsActive):
			fail("restoring", 3, errors.New("deployments_active: stop every live strategy before restoring"))
			return
		case errors.Is(err, store.ErrNotConfirmed), errors.Is(err, store.ErrBadArchive):
			fail("restoring", 3, err)
			return
		case err != nil:
			fail("restoring", 3, err)
			return
		}
		s.d.Log.Info("restore complete; restarting", "replaced", res.ReplacedDir)
		s.progress(JobProgress{JobID: id, Kind: JobRestore, Stage: "restart", Done: 4, Total: 4, Finished: true, ResultPath: res.ReplacedDir})
	})
	return id, nil
}

// scheduledBackupDue reports whether today's scheduled backup has not run yet and it is past 16:00 IST.
func (s *Service) scheduledBackupDue(ctx context.Context) (string, bool) {
	if s.d.DB == nil || s.d.UiState == nil || !settings.Get(s.d.Settings, settings.BackupScheduleEnabled) {
		return "", false
	}
	now := s.d.Clock.Now().In(clock.IST)
	if now.Hour() < ScheduledBackupHour {
		return "", false
	}
	today := now.Format("2006-01-02")
	last, err := s.d.UiState.Get(ctx, UiStateLastScheduledBackup)
	if err != nil || last == today {
		return "", false
	}
	return today, true
}

func (s *Service) runScheduledBackup(ctx context.Context, today string) {
	_, release, err := s.beginJob(JobBackup)
	if err != nil {
		s.d.Log.Info("scheduled backup skipped", "reason", err.Error())
		return
	}
	defer release()
	dir := settings.Get(s.d.Settings, settings.BackupScheduleFolder)
	if dir == "" {
		dir = s.d.Layout.BackupsDir
	}
	keep := settings.Get(s.d.Settings, settings.BackupScheduleKeep)
	path, pruned, err := s.d.DB.ScheduledBackup(ctx, s.backupOptions(dir, settings.Get(s.d.Settings, settings.BackupIncludeCandles)), keep)
	if err != nil {
		s.d.Log.Error("scheduled backup failed", "err", err)
		return
	}
	if err := s.d.UiState.Set(ctx, UiStateLastScheduledBackup, today); err != nil {
		s.d.Log.Warn("scheduled backup marker not saved", "err", err)
	}
	s.d.Log.Info("scheduled backup written", "path", path, "pruned", len(pruned))
}

// BackupOnQuit runs a missed scheduled backup synchronously (App.BeforeClose).
func (s *Service) BackupOnQuit(ctx context.Context) {
	if today, due := s.scheduledBackupDue(ctx); due {
		s.runScheduledBackup(ctx, today)
	}
}

// RunBackups runs the scheduled backup at the first 16:00 IST the app sees each day (App.start).
func (s *Service) RunBackups(ctx context.Context) {
	for {
		if today, due := s.scheduledBackupDue(ctx); due {
			s.runScheduledBackup(ctx, today)
		}
		now := s.d.Clock.Now().In(clock.IST)
		next := time.Date(now.Year(), now.Month(), now.Day(), ScheduledBackupHour, 0, 0, 0, clock.IST)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		tm := s.d.Clock.At(next)
		select {
		case <-ctx.Done():
			tm.Stop()
			return
		case <-tm.C():
		}
	}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -run 'TestDataStats|TestBackupNow|TestRestoreBackup|TestScheduledBackup' -v`
Expected: PASS (4 tests; the backup and restore tests each write a real `tar.zst` archive, about 1 s together).

- [ ] **Step 6: Commit**

```bash
git add internal/service/data.go internal/service/data_test.go internal/service/deps.go
git commit -m "feat(service): data stats, candle clear, instrument refresh, backup/restore jobs and the daily backup

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
