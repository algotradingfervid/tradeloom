// internal/service/data_test.go
package service

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	"tradeloom/internal/bus"
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
	// deviation D1: the not_logged_in refusal never reaches the refresher, so two calls (failed, ok), not three
	if err := e.svc.RefreshInstruments(ctx); err != nil || rf.calls != 2 {
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
	// deviation D2: store.Open requires BackupDir
	re, err := store.Open(ctx, layout.AppDB, store.Options{AppVersion: "1.2.3", BackupDir: layout.BackupsDir, Migrations: store.EmbeddedMigrations(), Now: e.clock.Now})
	if err != nil {
		t.Fatalf("restored app.db does not open: %v", err)
	}
	re.Close()
	// The platform is closed and the process must restart: no further job may start (deviation D4).
	if _, err := e.svc.BackupNow(ctx, "", false); refusedCode(t, err) != "job_running" {
		t.Fatalf("job after a finished restore: %v", err)
	}
}

// Deviation D3: store.InspectBackup itself refuses an archive from a newer schema with *store.SchemaTooNewError.
// InspectBackup reports it as OK=false with the archive's schema; RestoreBackup refuses with schema_too_new.
func TestBackup_RealNewerSchemaArchive(t *testing.T) {
	e := newSvcEnv(t)
	layout, _, _, _ := withData(t, e)
	ctx := context.Background()
	newer := filepath.Join(layout.ExportsDir, "tradeloom-backup-20991231-000000.tar.zst")
	writeManifestArchive(t, newer, store.Manifest{Format: store.ManifestFormat, CreatedAt: "2099-12-31T00:00:00Z", AppVersion: "9.9.9", Schema: store.SupportedSchema() + 1})
	info, err := e.svc.InspectBackup(ctx, newer)
	if err != nil || info.OK || info.Schema != store.SupportedSchema()+1 || info.SupportedSchema != store.SupportedSchema() || info.AppVersion != "9.9.9" || info.Reason == "" {
		t.Fatalf("%+v %v", info, err)
	}
	if _, err := e.svc.RestoreBackup(ctx, newer, store.RestoreConfirmPhrase); refusedCode(t, err) != "schema_too_new" {
		t.Fatal(err)
	}
}

// Deviation D4: a restore that fails after the platform was closed still ends with stage restart (with an error),
// because app.db is closed and only a relaunch brings the untouched data directory back.
func TestRestoreBackup_FailureAfterCloseStillRestarts(t *testing.T) {
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
	e.svc.d.CloseForRestore = func() error {
		if err := db.Close(); err != nil {
			return err
		}
		return os.Remove(archive) // the archive disappears between the check and the restore
	}
	id, err = e.svc.RestoreBackup(ctx, archive, store.RestoreConfirmPhrase)
	if err != nil {
		t.Fatal(err)
	}
	stages, last := waitJob(t, ch, id)
	if stages[len(stages)-1] != "restart" || last.Error == "" || last.ResultPath != "" {
		t.Fatalf("%v %+v", stages, last)
	}
	if _, err := os.Stat(layout.AppDB); err != nil {
		t.Fatalf("original app.db must stay in place: %v", err)
	}
	if _, err := e.svc.BackupNow(ctx, "", false); refusedCode(t, err) != "job_running" {
		t.Fatalf("job after a failed restore: %v", err)
	}
}

// A refused close keeps the app running: the job ends at stage closing and the slot is free again.
func TestRestoreBackup_CloseRefusedKeepsRunning(t *testing.T) {
	e := newSvcEnv(t)
	_, _, _, _ = withData(t, e)
	ch := startEvents(t, e)
	ctx := context.Background()
	id, err := e.svc.BackupNow(ctx, "", false)
	if err != nil {
		t.Fatal(err)
	}
	_, last := waitJob(t, ch, id)
	e.svc.d.CloseForRestore = func() error { return errors.New("platform busy") }
	id, err = e.svc.RestoreBackup(ctx, last.ResultPath, store.RestoreConfirmPhrase)
	if err != nil {
		t.Fatal(err)
	}
	stages, fin := waitJob(t, ch, id)
	if stages[len(stages)-1] != "closing" || fin.Error == "" {
		t.Fatalf("%v %+v", stages, fin)
	}
	rel := holdJob(t, e.svc)
	rel()
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
	loopDone := make(chan struct{})
	go func() { defer close(loopDone); e.svc.RunBackups(rctx) }()
	if !e.clock.WaitTimers(1, 3*time.Second) { // deviation D6: wait for the armed timer, not a fixed sleep
		t.Fatal("RunBackups armed no timer")
	}
	if n := archives(t, layout.BackupsDir); n != 0 {
		t.Fatalf("%d archives before 16:00", n)
	}
	e.clock.Advance(time.Minute)
	eventually(t, "scheduled backup", func() bool { return archives(t, layout.BackupsDir) == 1 })
	eventually(t, "marker", func() bool {
		v, _ := store.NewWindowStateRepo(db).Get(ctx, UiStateLastScheduledBackup)
		return v == "2026-10-12"
	})
	cancel() // stop the loop so the quit path below is the only runner
	<-loopDone
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

// triggerRefresher records Trigger calls like *instruments.Refresher.Trigger (non-blocking, coalescing).
type triggerRefresher struct {
	fakeRefresher
	mu       sync.Mutex
	triggers int
}

func (f *triggerRefresher) Trigger() { f.mu.Lock(); f.triggers++; f.mu.Unlock() }
func (f *triggerRefresher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.triggers
}

// Carry-forward Ruling 49: the supervisor kicks the instrument refresher once per new login session.
func TestRun_TriggersInstrumentRefreshOnLogin(t *testing.T) {
	e := newSvcEnv(t)
	rf := &triggerRefresher{}
	e.svc.d.Refresher = rf
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := startRun(t, e, ctx)
	eventually(t, "trigger for s1", func() bool { return rf.count() == 1 })
	bus.Publish(e.bus, kite.TopicSessionState, e.session.State()) // same session: no second trigger
	settle()
	if n := rf.count(); n != 1 {
		t.Fatalf("same session triggered again: %d", n)
	}
	e.session.setState(kite.SessionState{State: kite.StateLoggedOut})
	bus.Publish(e.bus, kite.TopicSessionState, e.session.State())
	settle()
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s2"})
	bus.Publish(e.bus, kite.TopicSessionState, e.session.State())
	eventually(t, "trigger for s2", func() bool { return rf.count() == 2 })
	cancel()
	waitDone(t, "Run", done)
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

// writeManifestArchive writes a tar.zst that holds only manifest.json.
func writeManifestArchive(t *testing.T, path string, m store.Manifest) {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	fh, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	zw, err := zstd.NewWriter(fh)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(b)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}
