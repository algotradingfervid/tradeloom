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
	// StageRestart is the last stage of every restore job that got past its checks: the app must quit with
	// exit code 3 and relaunch, whether or not Error is set (Ruling 75).
	StageRestart = "restart"
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

func (s *Service) progress(p JobProgress) {
	p.Error = logging.Default.RedactString(p.Error)
	bus.Publish(s.d.Bus, TopicJobProgress, p)
}

// fileSize is the size of path, 0 when it does not exist.
func fileSize(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}

func (s *Service) DataStats(ctx context.Context) (DataStats, error) {
	out := DataStats{DataDir: s.d.Layout.Root}
	if s.d.Layout.AppDB != "" {
		// The WAL holds committed pages not yet checkpointed into app.db; both are app.db's footprint.
		out.AppDBBytes = fileSize(s.d.Layout.AppDB) + fileSize(s.d.Layout.AppDB+"-wal")
	}
	if s.d.CandleAdmin != nil {
		st, err := s.d.CandleAdmin.Stats(ctx)
		if err != nil {
			return out, s.refuse(dataStage, err)
		}
		out.CandleBytes, out.CandleBudgetBytes = st.TotalBytes, st.BudgetBytes
		jobs, err := s.d.CandleAdmin.OpenJobs(ctx)
		if err != nil {
			return out, s.refuse(dataStage, err)
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

// ClearCandles drops the candle cache (FR-DATA-002); refused while a backfill job is open (it would write
// into the emptied store and lose its own job rows) and while backtest runs reference the candles.
// It never confirms referenced runs on the user's behalf (confirmRuns is nil).
func (s *Service) ClearCandles(ctx context.Context) error {
	if s.d.CandleAdmin == nil {
		return s.refuse(dataStage, refused(dataStage, "unavailable", "The candle store is not available."))
	}
	jobs, err := s.d.CandleAdmin.OpenJobs(ctx)
	if err != nil {
		return s.refuse(dataStage, err)
	}
	if len(jobs) > 0 {
		return s.refuse(dataStage, refused(dataStage, "backfill_running", fmt.Sprintf("%d candle downloads are still running. Clear the cache when they have finished.", len(jobs))))
	}
	err = s.d.CandleAdmin.Clear(ctx, nil)
	var rr *candles.RunsReferencedError
	if errors.As(err, &rr) {
		return s.refuse(dataStage, refused(dataStage, "runs_referenced", fmt.Sprintf("%d backtest runs still reference cached candles.", len(rr.Runs))))
	}
	if err != nil {
		return s.refuse(dataStage, err)
	}
	s.d.Log.Info("candle cache cleared")
	return nil
}

// RefreshInstruments re-downloads the instrument master now (FR-WATCH-008 manual refresh).
func (s *Service) RefreshInstruments(ctx context.Context) error {
	if s.d.Session.State().State != kite.StateReady {
		return s.refuse(dataStage, refused(dataStage, "not_logged_in", "Log in to refresh the instrument list."))
	}
	if s.d.Refresher == nil {
		return s.refuse(dataStage, refused(dataStage, "unavailable", "The instrument refresher is not available."))
	}
	if err := s.d.Refresher.Refresh(ctx); err != nil {
		s.d.Log.Warn("manual instrument refresh failed", "err", err)
		return s.refuse(dataStage, refused(dataStage, "refresh_failed", "The instrument list could not be refreshed: "+err.Error()))
	}
	return nil
}

// triggerInstrumentRefresh asks the refresher to download today's list if it is due (P0-2 Ruling 49:
// the 08:30 attempt usually fails before login). Called by Run once per new login session.
func (s *Service) triggerInstrumentRefresh() {
	if s.d.Refresher != nil {
		s.d.Refresher.Trigger()
	}
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
		return "", s.refuse(dataStage, refused(dataStage, "unavailable", "The database is not open."))
	}
	if folder == "" {
		folder = s.d.Layout.BackupsDir
	}
	if fi, err := os.Stat(folder); err != nil || !fi.IsDir() || !filepath.IsAbs(folder) {
		return "", s.refuse(dataStage, refused(dataStage, "invalid_folder", "Choose an existing folder for the backup."))
	}
	id, release, err := s.beginJob(JobBackup)
	if err != nil {
		return "", s.refuse(dataStage, err)
	}
	jctx := context.WithoutCancel(ctx)
	logging.Go("service.backup", func() {
		defer release()
		step := func(stage string, n int) {
			s.progress(JobProgress{JobID: id, Kind: JobBackup, Stage: stage, Done: n, Total: 4})
		}
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

func schemaTooNewReason(found, supported int) string {
	return fmt.Sprintf("This backup was made by a newer version (schema %d; this build supports %d).", found, supported)
}

// InspectBackup describes an archive for the restore dialog; an unreadable archive is OK=false, not an error.
func (s *Service) InspectBackup(_ context.Context, path string) (BackupInfo, error) {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return BackupInfo{}, s.refuse(dataStage, refused(dataStage, "not_found", "The backup file was not found."))
	}
	info := BackupInfo{Path: path, SupportedSchema: s.supportedSchema(), SizeBytes: fi.Size(), Counts: map[string]int{}}
	m, err := store.InspectBackup(path)
	var tooNew *store.SchemaTooNewError
	if errors.As(err, &tooNew) {
		// store.InspectBackup refuses a newer schema itself; show what the archive says.
		info.Schema, info.AppVersion = tooNew.Found, tooNew.WrittenBy
		info.Reason = schemaTooNewReason(tooNew.Found, info.SupportedSchema)
		return info, nil
	}
	if err != nil {
		info.Reason = logging.Default.RedactString("This file is not a Tradeloom backup: " + err.Error())
		return info, nil
	}
	info.Schema, info.AppVersion, info.CreatedAt, info.IncludesCandles = m.Schema, m.AppVersion, m.CreatedAt, m.IncludesCandles
	if m.Counts != nil {
		info.Counts = m.Counts
	}
	if m.Schema > info.SupportedSchema {
		info.Reason = schemaTooNewReason(m.Schema, info.SupportedSchema)
		return info, nil
	}
	info.OK = true
	return info, nil
}

// RestoreBackup starts a restore job (FR-DATA-005 AC3). It needs the typed phrase RESTORE; nothing is
// closed or replaced before every check passed. Once the job has started closing the platform it always
// ends with StageRestart (Ruling 74/75), with Error set when the close or the restore failed: app.db (or
// part of the platform) is closed and, after a swap, the data-dir lock lives in the replaced directory,
// so the app must quit with exit code 3 and relaunch at once. A relaunch is always safe: a failed restore
// replaced nothing. From the closing stage on the job slot stays taken, so no other job can start.
func (s *Service) RestoreBackup(ctx context.Context, path, confirmation string) (string, error) {
	if confirmation != store.RestoreConfirmPhrase {
		return "", s.refuse(dataStage, refused(dataStage, "not_confirmed", "Type "+store.RestoreConfirmPhrase+" to confirm."))
	}
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return "", s.refuse(dataStage, refused(dataStage, "bad_archive", "The backup file was not found."))
	}
	m, err := store.InspectBackup(path)
	var tooNew *store.SchemaTooNewError
	if errors.As(err, &tooNew) {
		return "", s.refuse(dataStage, refused(dataStage, "schema_too_new", schemaTooNewReason(tooNew.Found, s.supportedSchema())))
	}
	if err != nil {
		return "", s.refuse(dataStage, refused(dataStage, "bad_archive", "This file is not a Tradeloom backup: "+err.Error()))
	}
	if m.Schema > s.supportedSchema() {
		return "", s.refuse(dataStage, refused(dataStage, "schema_too_new", schemaTooNewReason(m.Schema, s.supportedSchema())))
	}
	if s.d.CloseForRestore == nil || s.d.Layout.Root == "" {
		return "", s.refuse(dataStage, refused(dataStage, "unavailable", "Restore is not available in this build."))
	}
	// The restore never releases the job slot: it always ends in a restart (Ruling 74).
	id, _, err := s.beginJob(JobRestore)
	if err != nil {
		return "", s.refuse(dataStage, err)
	}
	jctx := context.WithoutCancel(ctx)
	logging.Go("service.restore", func() {
		step := func(stage string, n int) {
			s.progress(JobProgress{JobID: id, Kind: JobRestore, Stage: stage, Done: n, Total: 4})
		}
		step("inspecting", 1)
		step("closing", 2)
		if err := s.d.CloseForRestore(); err != nil {
			// Part of the platform may already be closed; nothing was replaced, so a relaunch is safe.
			s.d.Log.Error("restore failed; restarting", "stage", "closing", "err", err)
			s.progress(JobProgress{JobID: id, Kind: JobRestore, Stage: StageRestart, Done: 4, Total: 4, Finished: true,
				Error: "Tradeloom could not close for the restore; your previous data will be put back when Tradeloom restarts: " + err.Error()})
			return
		}
		step("restoring", 3)
		res, err := store.Restore(jctx, store.RestoreRequest{Archive: path, DataDir: s.d.Layout.Root, Confirmation: confirmation, DeploymentsFlat: true, Now: s.d.Clock.Now})
		if err != nil && res.ReplacedDir == "" {
			msg := err.Error()
			if errors.Is(err, store.ErrDeploymentsActive) {
				msg = "deployments_active: stop every live strategy before restoring"
			}
			s.d.Log.Error("restore failed; restarting with the previous data directory", "stage", "restoring", "err", err)
			s.progress(JobProgress{JobID: id, Kind: JobRestore, Stage: StageRestart, Done: 4, Total: 4, Finished: true,
				Error: "The restore failed; your previous data will be put back when Tradeloom restarts: " + msg})
			return
		}
		if err != nil { // swapped, but the parent directory could not be synced
			s.d.Log.Warn("restore swapped the data directory with a warning", "err", err)
		}
		if s.d.Layout.SecretsFile != "" {
			if _, serr := os.Stat(filepath.Join(res.ReplacedDir, filepath.Base(s.d.Layout.SecretsFile))); serr == nil {
				s.d.Log.Warn("the encrypted secrets file stays in the replaced directory; Kite credentials must be entered again", "replaced", res.ReplacedDir)
			}
		}
		s.d.Log.Info("restore complete; restarting", "replaced", res.ReplacedDir)
		s.progress(JobProgress{JobID: id, Kind: JobRestore, Stage: StageRestart, Done: 4, Total: 4, Finished: true, ResultPath: res.ReplacedDir})
	})
	return id, nil
}

// scheduledBackupDue reports whether today's scheduled backup has not run yet and it is past 16:00 IST.
func (s *Service) scheduledBackupDue(ctx context.Context) (string, bool) {
	if s.d.DB == nil || s.d.UiState == nil || s.d.Settings == nil || !settings.Get(s.d.Settings, settings.BackupScheduleEnabled) {
		return "", false
	}
	now := s.d.Clock.Now().In(clock.IST)
	if now.Hour() < ScheduledBackupHour {
		return "", false
	}
	today := now.Format("2006-01-02")
	last, err := s.d.UiState.Get(ctx, UiStateLastScheduledBackup)
	if err != nil {
		s.d.Log.Warn("scheduled backup marker not readable", "err", err)
		return "", false
	}
	if last == today {
		return "", false
	}
	return today, true
}

// runScheduledBackup writes the day's backup. A failure is published on job.progress (kind backup, redacted
// Error, Ruling 74) and leaves the day's marker unset, so the next 16:00 or quit retries.
func (s *Service) runScheduledBackup(ctx context.Context, today string) {
	id, release, err := s.beginJob(JobBackup)
	if err != nil {
		s.d.Log.Info("scheduled backup skipped", "reason", err.Error())
		return
	}
	defer release()
	fail := func(stage string, n int, err error) {
		s.progress(JobProgress{JobID: id, Kind: JobBackup, Stage: stage, Done: n, Total: 4, Finished: true, Error: "The scheduled backup failed: " + err.Error()})
	}
	dir := settings.Get(s.d.Settings, settings.BackupScheduleFolder)
	if dir == "" {
		dir = s.d.Layout.BackupsDir
	}
	// Same rule as BackupNow: never create the folder (an unplugged drive must not fill the system disk).
	if fi, serr := os.Stat(dir); serr != nil || !fi.IsDir() || !filepath.IsAbs(dir) {
		s.d.Log.Warn("scheduled backup skipped: folder missing or not a folder", "folder", dir)
		fail("preparing", 1, fmt.Errorf("the folder %s does not exist or is not a folder", dir))
		return
	}
	keep := settings.Get(s.d.Settings, settings.BackupScheduleKeep)
	path, pruned, err := s.d.DB.ScheduledBackup(ctx, s.backupOptions(dir, settings.Get(s.d.Settings, settings.BackupIncludeCandles)), keep)
	if err != nil && path == "" {
		s.d.Log.Error("scheduled backup failed", "err", err)
		fail("writing", 2, err)
		return
	}
	if err != nil { // written, pruning failed: the day's backup exists
		s.d.Log.Warn("scheduled backup written; pruning old archives failed", "err", err)
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
